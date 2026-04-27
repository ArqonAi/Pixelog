package memory

import (
	"context"
	"crypto/sha256"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/ArqonAi/Pixelog/pkg/publish"
)

// ArchivalPhase represents which phase the pipeline is in.
type ArchivalPhase string

const (
	PhaseCompress    ArchivalPhase = "compress"
	PhaseArchive     ArchivalPhase = "archive"
	PhaseConsolidate ArchivalPhase = "consolidate"
)

// ArchivalResult captures the output of a full archival pipeline run.
type ArchivalResult struct {
	Phase          ArchivalPhase    `json:"phase"`
	Namespace      string           `json:"namespace"`
	MessageCount   int              `json:"message_count"`
	Summary        string           `json:"summary,omitempty"`
	ExtractedCount int              `json:"extracted_count"`
	CapsulePath    string           `json:"capsule_path,omitempty"`
	ContentHash    [32]byte         `json:"content_hash,omitempty"`
	PixeURI        string           `json:"pixe_uri,omitempty"`
	OnChainTx      string           `json:"on_chain_tx,omitempty"`
	Publications   []publish.Result `json:"publications,omitempty"`
	PublishErrors  map[string]string `json:"publish_errors,omitempty"`
	Duration       time.Duration    `json:"duration"`
	Error          string           `json:"error,omitempty"`
}

// CapsuleConverter writes the supplied content to a .pixe capsule and returns
// the canonical capsule path (or content-addressed URI) it produced.
type CapsuleConverter func(ctx context.Context, inputPath string) (string, error)

// OnChainPublisher publishes a content hash on-chain for an agent.
// Implementation is host-supplied; pixelog core stays chain-agnostic.
type OnChainPublisher func(ctx context.Context, agentTokenID uint64, contentHash [32]byte, description string) error

// ArchivalPipeline implements the three-phase compress -> archive -> consolidate flow.
type ArchivalPipeline struct {
	condenser         *Condenser
	categories        *CategoryStore
	converter         CapsuleConverter
	publisher         OnChainPublisher
	blobPublishers    []publish.Publisher
	summarizer        LLMSummarizer
	contentSummarizer ContentSummarizer
	namespace         string
}

// ArchivalConfig holds pipeline configuration.
type ArchivalConfig struct {
	Namespace         string // capsule namespace used for memory URIs
	Condenser         *CondenserConfig
	Summarizer        LLMSummarizer
	ContentSummarizer ContentSummarizer // for L0/L1 tier generation
	Converter         CapsuleConverter
	Publisher         OnChainPublisher    // optional on-chain hash anchor (e.g. ERC-8004)
	BlobPublishers    []publish.Publisher // optional durability layer (IPFS, Arweave, ...)
	Categories        *CategoryStore
}

// NewArchivalPipeline creates a new three-phase archival pipeline.
func NewArchivalPipeline(cfg *ArchivalConfig) *ArchivalPipeline {
	if cfg == nil {
		cfg = &ArchivalConfig{}
	}
	condenserCfg := cfg.Condenser
	if condenserCfg == nil {
		condenserCfg = DefaultCondenserConfig()
	}
	return &ArchivalPipeline{
		condenser:         NewCondenser(condenserCfg, cfg.Summarizer),
		categories:        cfg.Categories,
		converter:         cfg.Converter,
		publisher:         cfg.Publisher,
		blobPublishers:    cfg.BlobPublishers,
		summarizer:        cfg.Summarizer,
		contentSummarizer: cfg.ContentSummarizer,
		namespace:         cfg.Namespace,
	}
}

// CompressResult captures Phase 1 output.
type CompressResult struct {
	Summary       string        `json:"summary"`
	Memories      []TypedMemory `json:"memories"`
	TieredEntries []TieredEntry `json:"tiered_entries,omitempty"`
	SummaryL0     string        `json:"summary_l0,omitempty"`
	SummaryL1     string        `json:"summary_l1,omitempty"`
	TokensSaved   int           `json:"tokens_saved"`
}

// Compress runs Phase 1: summarize conversation, extract typed memories.
func (p *ArchivalPipeline) Compress(ctx context.Context, messages []CondenserEvent) (*CompressResult, error) {
	if len(messages) == 0 {
		return &CompressResult{}, nil
	}

	for _, msg := range messages {
		p.condenser.AddEvent(msg)
	}

	condensed, err := p.condenser.Condense(ctx)
	if err != nil {
		return nil, fmt.Errorf("condensation failed: %w", err)
	}

	tokensSaved := 0
	if condensed != nil {
		tokensSaved = condensed.TokensSaved
	}

	events := p.condenser.GetEvents()
	var summaryParts []string
	for _, e := range events {
		if e.Condensed {
			summaryParts = append(summaryParts, e.Content)
		}
	}
	summary := strings.Join(summaryParts, "\n")
	if summary == "" {
		var sb strings.Builder
		for _, e := range events {
			sb.WriteString(fmt.Sprintf("[%s] %s\n", e.Role, truncateContent(e.Content, 200)))
		}
		summary = sb.String()
	}

	memories := extractTypedMemories(messages)

	if p.categories != nil && len(memories) > 0 {
		for _, mem := range memories {
			if err := p.categories.Store(ctx, mem); err != nil {
				log.Printf("[Archival] WARNING: failed to store typed memory %s: %v", mem.ID, err)
			}
		}
	}

	var tieredEntries []TieredEntry
	for _, mem := range memories {
		tieredEntries = append(tieredEntries, TieredFromTypedMemory(p.namespace, mem))
	}

	summaryL0, summaryL1, _ := GenerateTiers(ctx, summary, p.contentSummarizer)

	p.condenser.Reset()

	return &CompressResult{
		Summary:       summary,
		Memories:      memories,
		TieredEntries: tieredEntries,
		SummaryL0:     summaryL0,
		SummaryL1:     summaryL1,
		TokensSaved:   tokensSaved,
	}, nil
}

// Archive runs Phase 2: convert to .pixe, compute content hash, generate URI.
func (p *ArchivalPipeline) Archive(ctx context.Context, capsulePath string, compressed *CompressResult) (*ArchivalResult, error) {
	if compressed == nil {
		return nil, fmt.Errorf("no compressed data to archive")
	}

	result := &ArchivalResult{
		Phase:          PhaseArchive,
		Namespace:      p.namespace,
		ExtractedCount: len(compressed.Memories),
	}
	start := time.Now()

	if p.converter != nil && capsulePath != "" {
		converted, err := p.converter(ctx, capsulePath)
		if err != nil {
			result.Error = fmt.Sprintf("capsule conversion failed: %v", err)
			log.Printf("[Archival] WARNING: %s", result.Error)
		} else {
			result.CapsulePath = converted
		}
	}

	hash := sha256.Sum256([]byte(compressed.Summary))
	result.ContentHash = hash
	result.PixeURI = BuildCapsuleURI(fmt.Sprintf("%x", hash))

	// Publish the capsule blob to every configured durability layer in
	// parallel. Publisher failures are recorded but do not abort the
	// pipeline — archival succeeds as long as the local capsule is written.
	if len(p.blobPublishers) > 0 && result.CapsulePath != "" {
		capsuleBytes, err := os.ReadFile(result.CapsulePath)
		if err != nil {
			log.Printf("[Archival] WARNING: read capsule for publishing: %v", err)
		} else {
			pubs, errs := p.runBlobPublishers(ctx, capsuleBytes)
			result.Publications = pubs
			if len(errs) > 0 {
				result.PublishErrors = errs
			}
		}
	}

	result.Duration = time.Since(start)
	return result, nil
}

// runBlobPublishers fans the capsule out to every configured publisher
// concurrently and collects results / per-network errors.
func (p *ArchivalPipeline) runBlobPublishers(ctx context.Context, blob []byte) ([]publish.Result, map[string]string) {
	type outcome struct {
		res publish.Result
		err error
		net string
	}
	ch := make(chan outcome, len(p.blobPublishers))
	var wg sync.WaitGroup
	for _, pub := range p.blobPublishers {
		wg.Add(1)
		go func(pub publish.Publisher) {
			defer wg.Done()
			res, err := pub.Publish(ctx, blob, "video/mp4")
			ch <- outcome{res: res, err: err, net: pub.Network()}
		}(pub)
	}
	wg.Wait()
	close(ch)

	var pubs []publish.Result
	errs := map[string]string{}
	for o := range ch {
		if o.err != nil {
			errs[o.net] = o.err.Error()
			log.Printf("[Archival] WARNING: publish to %s failed: %v", o.net, o.err)
			continue
		}
		pubs = append(pubs, o.res)
	}
	return pubs, errs
}

// Publish runs Phase 3 (optional): publish content hash on-chain.
func (p *ArchivalPipeline) Publish(ctx context.Context, agentTokenID uint64, archiveResult *ArchivalResult, description string) error {
	if p.publisher == nil {
		return nil
	}
	if archiveResult == nil {
		return fmt.Errorf("no archive result to publish")
	}
	return p.publisher(ctx, agentTokenID, archiveResult.ContentHash, description)
}

// RunFull executes all three phases in sequence.
// agentTokenID is optional; pass 0 to skip on-chain publishing.
func (p *ArchivalPipeline) RunFull(ctx context.Context, namespace string, messages []CondenserEvent, capsulePath string, agentTokenID uint64) (*ArchivalResult, error) {
	start := time.Now()

	compressed, err := p.Compress(ctx, messages)
	if err != nil {
		return &ArchivalResult{
			Phase:     PhaseCompress,
			Namespace: namespace,
			Error:     err.Error(),
			Duration:  time.Since(start),
		}, err
	}

	archiveResult, err := p.Archive(ctx, capsulePath, compressed)
	if err != nil {
		return &ArchivalResult{
			Phase:     PhaseArchive,
			Namespace: namespace,
			Error:     err.Error(),
			Duration:  time.Since(start),
		}, err
	}

	archiveResult.Namespace = namespace
	archiveResult.MessageCount = len(messages)
	archiveResult.Summary = truncateContent(compressed.Summary, 500)

	if agentTokenID > 0 {
		desc := fmt.Sprintf("session-%s-%s", namespace, time.Now().Format("20060102-150405"))
		if err := p.Publish(ctx, agentTokenID, archiveResult, desc); err != nil {
			log.Printf("[Archival] WARNING: on-chain publish failed for %s: %v", namespace, err)
		} else {
			archiveResult.Phase = PhaseConsolidate
		}
	}

	archiveResult.Duration = time.Since(start)
	return archiveResult, nil
}

// extractTypedMemories analyzes conversation events and extracts typed memories.
// Uses heuristic keyword matching for classification.
func extractTypedMemories(events []CondenserEvent) []TypedMemory {
	var memories []TypedMemory
	ts := time.Now()

	for i, e := range events {
		if e.Role != "user" && e.Role != "assistant" {
			continue
		}

		lower := strings.ToLower(e.Content)
		// Offset nanoseconds by event index to prevent ID collision when
		// multiple events share the same timestamp.
		eventTS := ts.Add(time.Duration(i) * time.Nanosecond)
		extracted := classifyContent(e.Content, lower, e.Role, eventTS)
		memories = append(memories, extracted...)
	}

	return memories
}

func classifyContent(content, lower, role string, ts time.Time) []TypedMemory {
	var memories []TypedMemory

	if containsAny(lower, []string{"i prefer", "i like", "i want", "i always", "i never", "my favorite", "i'd rather"}) {
		memories = append(memories, TypedMemory{
			ID:         fmt.Sprintf("pref_%d", ts.UnixNano()),
			Category:   CategoryPreference,
			Key:        extractKey(content, "preference"),
			Value:      truncateContent(content, 300),
			Source:     role,
			Timestamp:  ts,
			Confidence: 0.7,
		})
	}

	if containsAny(lower, []string{"always ", "never ", "remember to", "make sure", "don't forget", "from now on"}) && role == "user" {
		memories = append(memories, TypedMemory{
			ID:         fmt.Sprintf("inst_%d", ts.UnixNano()),
			Category:   CategoryInstruction,
			Key:        extractKey(content, "instruction"),
			Value:      truncateContent(content, 300),
			Source:     role,
			Timestamp:  ts,
			Confidence: 0.8,
		})
	}

	if containsAny(lower, []string{"is located", "was founded", "the answer is", "it means", "defined as"}) {
		memories = append(memories, TypedMemory{
			ID:         fmt.Sprintf("fact_%d", ts.UnixNano()),
			Category:   CategoryFact,
			Key:        extractKey(content, "fact"),
			Value:      truncateContent(content, 300),
			Source:     role,
			Timestamp:  ts,
			Confidence: 0.6,
		})
	}

	if containsAny(lower, []string{"yesterday", "tomorrow", "next week", "last month", "meeting at", "scheduled for", "deadline"}) {
		memories = append(memories, TypedMemory{
			ID:         fmt.Sprintf("event_%d", ts.UnixNano()),
			Category:   CategoryEvent,
			Key:        extractKey(content, "event"),
			Value:      truncateContent(content, 300),
			Source:     role,
			Timestamp:  ts,
			Confidence: 0.6,
		})
	}

	if containsAny(lower, []string{"works at", "married to", "friend of", "manager is", "team lead", "reports to", "colleague"}) {
		memories = append(memories, TypedMemory{
			ID:         fmt.Sprintf("rel_%d", ts.UnixNano()),
			Category:   CategoryRelationship,
			Key:        extractKey(content, "relationship"),
			Value:      truncateContent(content, 300),
			Source:     role,
			Timestamp:  ts,
			Confidence: 0.65,
		})
	}

	if containsAny(lower, []string{"how to ", "the steps are", "procedure for", "workflow:", "to do this you"}) {
		memories = append(memories, TypedMemory{
			ID:         fmt.Sprintf("skill_%d", ts.UnixNano()),
			Category:   CategorySkill,
			Key:        extractKey(content, "skill"),
			Value:      truncateContent(content, 300),
			Source:     role,
			Timestamp:  ts,
			Confidence: 0.6,
		})
	}

	return memories
}

func containsAny(s string, needles []string) bool {
	for _, n := range needles {
		if strings.Contains(s, n) {
			return true
		}
	}
	return false
}

func extractKey(content, fallback string) string {
	for _, sep := range []string{". ", "! ", "? ", "\n"} {
		if idx := strings.Index(content, sep); idx > 0 && idx < 100 {
			return truncateContent(content[:idx], 80)
		}
	}
	if len(content) > 80 {
		return content[:80]
	}
	if content == "" {
		return fallback
	}
	return content
}

// truncateContent returns at most maxLen bytes of s, with an ellipsis suffix
// when truncation occurs.
func truncateContent(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}
