package memory

import (
	"fmt"
	"regexp"
	"strings"
)

// CompressedFrame represents raw content distilled into structured facts, concepts, and narrative.
// It bridges raw .pixe storage with the L0/L1/L2 tiered-context retrieval layer.
type CompressedFrame struct {
	FrameID   string   `json:"frame_id"`
	Facts     []string `json:"facts"`     // discrete, searchable assertions
	Concepts  []string `json:"concepts"`  // tags/topics for graph building
	Narrative string   `json:"narrative"` // human-readable summary (maps to L0/L1)
	Source    string   `json:"source"`    // original source file
}

// LLMCompressor uses an LLM to extract structured information from raw content.
type LLMCompressor struct {
	chatFn func(prompt string) (string, error) // injected LLM call
}

// NewLLMCompressor creates a compressor with the given LLM chat function.
// The chatFn should accept a prompt and return the LLM response.
func NewLLMCompressor(chatFn func(string) (string, error)) *LLMCompressor {
	return &LLMCompressor{chatFn: chatFn}
}

// CompressFrame takes raw content and extracts structured facts, concepts, and a narrative.
func (c *LLMCompressor) CompressFrame(frameID, content, sourceFile string) (*CompressedFrame, error) {
	if c.chatFn == nil {
		return c.fallbackCompress(frameID, content, sourceFile), nil
	}

	prompt := buildCompressionPrompt(content)
	response, err := c.chatFn(prompt)
	if err != nil {
		// Degrade gracefully to heuristic extraction
		return c.fallbackCompress(frameID, content, sourceFile), nil
	}

	frame := parseCompressedResponse(response)
	frame.FrameID = frameID
	frame.Source = sourceFile

	// Ensure we got something useful
	if len(frame.Facts) == 0 && frame.Narrative == "" {
		return c.fallbackCompress(frameID, content, sourceFile), nil
	}

	return frame, nil
}

// CompressBatch compresses multiple content chunks in sequence.
func (c *LLMCompressor) CompressBatch(items []struct {
	FrameID    string
	Content    string
	SourceFile string
}) ([]*CompressedFrame, error) {
	results := make([]*CompressedFrame, 0, len(items))
	for _, item := range items {
		frame, err := c.CompressFrame(item.FrameID, item.Content, item.SourceFile)
		if err != nil {
			continue
		}
		results = append(results, frame)
	}
	return results, nil
}

func buildCompressionPrompt(content string) string {
	// Truncate very long content to stay within token limits
	if len(content) > 6000 {
		content = content[:6000] + "\n... [truncated]"
	}

	return fmt.Sprintf(`Extract structured information from the following content.

Respond in EXACTLY this format:

<facts>
- fact 1
- fact 2
</facts>

<concepts>
concept1, concept2, concept3
</concepts>

<narrative>
A 1-2 sentence summary of the content.
</narrative>

Content:
%s`, content)
}

func parseCompressedResponse(response string) *CompressedFrame {
	frame := &CompressedFrame{}

	// Extract facts
	factsRe := regexp.MustCompile(`(?s)<facts>\s*(.*?)\s*</facts>`)
	if m := factsRe.FindStringSubmatch(response); len(m) > 1 {
		lines := strings.Split(m[1], "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			line = strings.TrimPrefix(line, "- ")
			line = strings.TrimPrefix(line, "* ")
			if line != "" {
				frame.Facts = append(frame.Facts, line)
			}
		}
	}

	// Extract concepts
	conceptsRe := regexp.MustCompile(`(?s)<concepts>\s*(.*?)\s*</concepts>`)
	if m := conceptsRe.FindStringSubmatch(response); len(m) > 1 {
		parts := strings.Split(m[1], ",")
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if part != "" {
				frame.Concepts = append(frame.Concepts, part)
			}
		}
	}

	// Extract narrative
	narrativeRe := regexp.MustCompile(`(?s)<narrative>\s*(.*?)\s*</narrative>`)
	if m := narrativeRe.FindStringSubmatch(response); len(m) > 1 {
		frame.Narrative = strings.TrimSpace(m[1])
	}

	return frame
}

// fallbackCompress uses heuristic extraction when LLM is unavailable.
func (c *LLMCompressor) fallbackCompress(frameID, content, sourceFile string) *CompressedFrame {
	frame := &CompressedFrame{
		FrameID: frameID,
		Source:  sourceFile,
	}

	// Extract first 200 chars as narrative
	if len(content) > 200 {
		frame.Narrative = strings.TrimSpace(content[:200]) + "..."
	} else {
		frame.Narrative = strings.TrimSpace(content)
	}

	// Extract sentences as facts (first 5 meaningful sentences)
	sentences := splitSentences(content)
	maxFacts := 5
	if len(sentences) < maxFacts {
		maxFacts = len(sentences)
	}
	for i := 0; i < maxFacts; i++ {
		s := strings.TrimSpace(sentences[i])
		if len(s) > 10 { // skip very short fragments
			frame.Facts = append(frame.Facts, s)
		}
	}

	// Extract words that look like concepts (capitalized multi-word terms, technical terms)
	frame.Concepts = extractHeuristicConcepts(content)

	return frame
}

func splitSentences(text string) []string {
	// Simple sentence splitter
	re := regexp.MustCompile(`[.!?]+\s+`)
	return re.Split(text, -1)
}

func extractHeuristicConcepts(content string) []string {
	// Find capitalized multi-word terms and technical patterns
	conceptRe := regexp.MustCompile(`[A-Z][a-z]+(?:\s+[A-Z][a-z]+)+`)
	matches := conceptRe.FindAllString(content, 20)

	seen := make(map[string]struct{})
	var concepts []string
	for _, m := range matches {
		lower := strings.ToLower(m)
		if _, ok := seen[lower]; ok {
			continue
		}
		seen[lower] = struct{}{}
		concepts = append(concepts, m)
		if len(concepts) >= 10 {
			break
		}
	}

	return concepts
}
