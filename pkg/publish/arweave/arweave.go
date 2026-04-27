package arweave

import (
	"bytes"
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/ArqonAi/Pixelog/pkg/publish"
)

// Config configures an ArweavePublisher.
type Config struct {
	// NodeURL is the base URL of an Arweave HTTP node
	// (e.g. "https://arweave.net" or "http://localhost:1984" for arlocal).
	NodeURL string

	// Wallet is a parsed Arweave JWK. Required for signing.
	Wallet *Wallet

	// GatewayURL is an optional public gateway used to populate Result.URL
	// (defaults to NodeURL when empty).
	GatewayURL string

	// HTTPClient overrides the default http.Client.
	HTTPClient *http.Client

	// ExtraTags lets callers attach Arweave tags to every published tx
	// (e.g. {"App-Name":"Pixelog"}). Keys and values are UTF-8 strings.
	ExtraTags []Tag
}

// Tag is a single Arweave transaction tag.
type Tag struct {
	Name  string `json:"name"`  // base64url-encoded on the wire
	Value string `json:"value"` // base64url-encoded on the wire
}

// ArweavePublisher implements publish.Publisher against the Arweave network.
type ArweavePublisher struct {
	node    string
	gateway string
	wallet  *Wallet
	http    *http.Client
	tags    []Tag
}

// New constructs an ArweavePublisher.
func New(cfg Config) (*ArweavePublisher, error) {
	if cfg.NodeURL == "" {
		return nil, fmt.Errorf("arweave: NodeURL is required")
	}
	if cfg.Wallet == nil {
		return nil, fmt.Errorf("arweave: Wallet is required")
	}
	hc := cfg.HTTPClient
	if hc == nil {
		hc = &http.Client{Timeout: 120 * time.Second}
	}
	gw := cfg.GatewayURL
	if gw == "" {
		gw = cfg.NodeURL
	}
	return &ArweavePublisher{
		node:    strings.TrimRight(cfg.NodeURL, "/"),
		gateway: strings.TrimRight(gw, "/"),
		wallet:  cfg.Wallet,
		http:    hc,
		tags:    cfg.ExtraTags,
	}, nil
}

// Network returns "arweave".
func (p *ArweavePublisher) Network() string { return "arweave" }

// transaction is the wire-format Arweave format-2 transaction.
type transaction struct {
	Format    int    `json:"format"`
	ID        string `json:"id"`         // b64url(sha256(signature))
	LastTx    string `json:"last_tx"`    // anchor
	Owner     string `json:"owner"`      // b64url(modulus)
	Tags      []Tag  `json:"tags"`       // base64url-encoded names/values
	Target    string `json:"target"`     // empty for data tx
	Quantity  string `json:"quantity"`   // "0" for data tx
	Data      string `json:"data"`       // empty in format 2 (uploaded via /chunk)
	DataSize  string `json:"data_size"`  // decimal string
	DataRoot  string `json:"data_root"`  // b64url(merkle root)
	Reward    string `json:"reward"`     // decimal Winston
	Signature string `json:"signature"`  // b64url(RSA-PSS-SHA256)
}

// Publish builds, signs, submits, and uploads chunks for a format-2 tx.
func (p *ArweavePublisher) Publish(ctx context.Context, data []byte, mimeType string) (publish.Result, error) {
	if len(data) == 0 {
		return publish.Result{}, fmt.Errorf("arweave: empty payload")
	}

	// Step 1: chunk + merkle.
	chunks := chunkData(data)
	root, leaves := buildMerkleTree(chunks)
	if root == nil {
		return publish.Result{}, fmt.Errorf("arweave: empty merkle tree")
	}
	dataRoot := root.id

	// Step 2: assemble tags (Content-Type + caller extras).
	tags := append([]Tag{}, p.tags...)
	if mimeType != "" {
		tags = append(tags, Tag{Name: "Content-Type", Value: mimeType})
	}
	tags = append(tags, Tag{Name: "App-Name", Value: "Pixelog"})

	// Step 3: anchor + price.
	anchor, err := p.fetchAnchor(ctx)
	if err != nil {
		return publish.Result{}, err
	}
	reward, err := p.fetchPrice(ctx, len(data))
	if err != nil {
		return publish.Result{}, err
	}

	// Step 4: build deep-hash signature payload.
	encodedTags := encodeTags(tags)
	sigData := DHList{
		DHBlob([]byte("2")),                  // format
		DHBlob(p.wallet.OwnerN),              // owner (raw modulus)
		DHBlob(nil),                          // target (none)
		DHBlob([]byte("0")),                  // quantity
		DHBlob([]byte(reward)),               // reward
		DHBlob(decodeAnchor(anchor)),         // last_tx as raw bytes
		tagsToDeepHash(encodedTags),          // tags
		DHBlob([]byte(strconv.Itoa(len(data)))), // data_size
		DHBlob(dataRoot),                     // data_root
	}
	sigPayload := deepHash(sigData)

	// Step 5: sign with RSA-PSS-SHA256, salt length = digest length (32).
	// Web-Crypto-style RSA-PSS hashes the input with the named hash before
	// the PSS primitive; Go's rsa.SignPSS expects the caller to supply the
	// already-computed digest, so we sha256 the deep-hash output first.
	digest := sha256.Sum256(sigPayload)
	sig, err := rsa.SignPSS(rand.Reader, p.wallet.Key, crypto.SHA256, digest[:], &rsa.PSSOptions{
		SaltLength: rsa.PSSSaltLengthEqualsHash,
		Hash:       crypto.SHA256,
	})
	if err != nil {
		return publish.Result{}, fmt.Errorf("arweave: PSS sign: %w", err)
	}

	idBytes := sha256.Sum256(sig)
	tx := &transaction{
		Format:    2,
		ID:        b64URLEncode(idBytes[:]),
		LastTx:    anchor,
		Owner:     b64URLEncode(p.wallet.OwnerN),
		Tags:      encodedTags,
		Target:    "",
		Quantity:  "0",
		Data:      "",
		DataSize:  strconv.Itoa(len(data)),
		DataRoot:  b64URLEncode(dataRoot),
		Reward:    reward,
		Signature: b64URLEncode(sig),
	}

	// Step 6: submit tx.
	if err := p.submitTx(ctx, tx); err != nil {
		return publish.Result{}, err
	}

	// Step 7: upload each data chunk.
	for i, c := range chunks {
		path := generatePath(root, i, leaves)
		if err := p.submitChunk(ctx, tx.DataRoot, len(data), c, path); err != nil {
			return publish.Result{}, fmt.Errorf("arweave: chunk %d/%d upload: %w", i+1, len(chunks), err)
		}
	}

	res := publish.Result{
		Network:  "arweave",
		Locator:  tx.ID,
		Size:     int64(len(data)),
		MimeType: mimeType,
		URL:      fmt.Sprintf("%s/%s", p.gateway, tx.ID),
	}
	return res, nil
}

// fetchAnchor pulls a recent tx anchor used as last_tx.
func (p *ArweavePublisher) fetchAnchor(ctx context.Context) (string, error) {
	body, err := p.get(ctx, "/tx_anchor")
	if err != nil {
		return "", fmt.Errorf("arweave: tx_anchor: %w", err)
	}
	return strings.TrimSpace(string(body)), nil
}

// fetchPrice asks the node for the cost of `bytes` worth of storage.
func (p *ArweavePublisher) fetchPrice(ctx context.Context, bytes int) (string, error) {
	body, err := p.get(ctx, fmt.Sprintf("/price/%d", bytes))
	if err != nil {
		return "", fmt.Errorf("arweave: price: %w", err)
	}
	return strings.TrimSpace(string(body)), nil
}

func (p *ArweavePublisher) submitTx(ctx context.Context, tx *transaction) error {
	body, err := json.Marshal(tx)
	if err != nil {
		return fmt.Errorf("arweave: marshal tx: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.node+"/tx", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := p.http.Do(req)
	if err != nil {
		return fmt.Errorf("arweave: POST /tx: %w", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("arweave: POST /tx -> %d: %s",
			resp.StatusCode, truncate(respBody, 500))
	}
	return nil
}

// chunkPayload mirrors the JSON expected by Arweave's POST /chunk.
type chunkPayload struct {
	DataRoot string `json:"data_root"`
	DataSize string `json:"data_size"`
	DataPath string `json:"data_path"`
	Offset   string `json:"offset"`
	Chunk    string `json:"chunk"`
}

func (p *ArweavePublisher) submitChunk(ctx context.Context, dataRoot string, totalSize int, c Chunk, path []byte) error {
	payload := chunkPayload{
		DataRoot: dataRoot,
		DataSize: strconv.Itoa(totalSize),
		DataPath: b64URLEncode(path),
		Offset:   strconv.Itoa(c.MaxByteRange - 1),
		Chunk:    b64URLEncode(c.Data),
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.node+"/chunk", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := p.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("POST /chunk -> %d: %s", resp.StatusCode, truncate(respBody, 500))
	}
	return nil
}

func (p *ArweavePublisher) get(ctx context.Context, path string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.node+path, nil)
	if err != nil {
		return nil, err
	}
	resp, err := p.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode/100 != 2 {
		return nil, fmt.Errorf("GET %s -> %d: %s", path, resp.StatusCode, truncate(body, 500))
	}
	return body, nil
}

// encodeTags base64url-encodes tag names and values for the wire JSON.
func encodeTags(tags []Tag) []Tag {
	out := make([]Tag, len(tags))
	for i, t := range tags {
		out[i] = Tag{
			Name:  b64URLEncode([]byte(t.Name)),
			Value: b64URLEncode([]byte(t.Value)),
		}
	}
	return out
}

// tagsToDeepHash converts the wire-encoded tags back to raw bytes for
// deep-hashing (the deep-hash spec consumes the *raw* tag bytes).
func tagsToDeepHash(encoded []Tag) DHList {
	out := make(DHList, 0, len(encoded))
	for _, t := range encoded {
		nameBytes, _ := b64URLDecode(t.Name)
		valueBytes, _ := b64URLDecode(t.Value)
		out = append(out, DHList{DHBlob(nameBytes), DHBlob(valueBytes)})
	}
	return out
}

// decodeAnchor returns raw bytes of the b64url-encoded anchor string.
// An empty anchor is permitted (genesis case for arlocal).
func decodeAnchor(anchor string) []byte {
	if anchor == "" {
		return nil
	}
	b, err := b64URLDecode(anchor)
	if err != nil {
		// Anchor came from the node and must be valid b64url; if not, fall
		// back to raw bytes so signing can still proceed deterministically.
		return []byte(anchor)
	}
	return b
}

func truncate(b []byte, n int) string {
	if len(b) <= n {
		return string(b)
	}
	return string(b[:n]) + "..."
}
