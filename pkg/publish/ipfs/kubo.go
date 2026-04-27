// Package ipfs provides a real BlobPublisher targeting any Kubo-compatible
// IPFS HTTP API endpoint (local kubo node, Pinata, web3.storage,
// nft.storage, IPFS Cluster, ...).
//
// The publisher uses /api/v0/add with CID v1 and raw-leaves enabled so
// every upload of identical bytes produces the same CID, regardless of
// gateway.
package ipfs

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"strings"
	"time"

	"github.com/ArqonAi/Pixelog/pkg/publish"
)

// Config configures a KuboPublisher.
type Config struct {
	// APIURL is the base URL of the Kubo HTTP API
	// (e.g. "http://127.0.0.1:5001" or "https://api.pinata.cloud").
	APIURL string

	// AuthHeader is an optional value for the Authorization header
	// (e.g. "Bearer <jwt>" for Pinata).
	AuthHeader string

	// GatewayURL is an optional public gateway used to populate Result.URL
	// (e.g. "https://ipfs.io"). Empty means no URL is returned.
	GatewayURL string

	// HTTPClient overrides the default http.Client. Useful for tests and
	// custom transports.
	HTTPClient *http.Client

	// Pin requests the node to pin the content (Kubo default is true).
	// Set false for Pinata or read-only gateways.
	Pin *bool
}

// KuboPublisher implements publish.Publisher against the Kubo HTTP API.
type KuboPublisher struct {
	apiURL     string
	authHeader string
	gateway    string
	http       *http.Client
	pin        bool
}

// New constructs a KuboPublisher.
func New(cfg Config) (*KuboPublisher, error) {
	if cfg.APIURL == "" {
		return nil, fmt.Errorf("ipfs: APIURL is required")
	}
	if _, err := url.Parse(cfg.APIURL); err != nil {
		return nil, fmt.Errorf("ipfs: invalid APIURL: %w", err)
	}
	hc := cfg.HTTPClient
	if hc == nil {
		hc = &http.Client{Timeout: 60 * time.Second}
	}
	pin := true
	if cfg.Pin != nil {
		pin = *cfg.Pin
	}
	return &KuboPublisher{
		apiURL:     strings.TrimRight(cfg.APIURL, "/"),
		authHeader: cfg.AuthHeader,
		gateway:    strings.TrimRight(cfg.GatewayURL, "/"),
		http:       hc,
		pin:        pin,
	}, nil
}

// Network returns "ipfs".
func (p *KuboPublisher) Network() string { return "ipfs" }

// kuboAddResponse mirrors Kubo's /api/v0/add JSON response.
type kuboAddResponse struct {
	Name string `json:"Name"`
	Hash string `json:"Hash"`
	Size string `json:"Size"`
}

// Publish uploads data via /api/v0/add and returns the resulting CID.
func (p *KuboPublisher) Publish(ctx context.Context, data []byte, mimeType string) (publish.Result, error) {
	if len(data) == 0 {
		return publish.Result{}, fmt.Errorf("ipfs: empty payload")
	}

	body := &bytes.Buffer{}
	mp := multipart.NewWriter(body)

	header := make(textproto.MIMEHeader)
	header.Set("Content-Disposition", `form-data; name="file"; filename="capsule.pixe"`)
	if mimeType != "" {
		header.Set("Content-Type", mimeType)
	} else {
		header.Set("Content-Type", "application/octet-stream")
	}
	part, err := mp.CreatePart(header)
	if err != nil {
		return publish.Result{}, fmt.Errorf("ipfs: multipart create: %w", err)
	}
	if _, err := part.Write(data); err != nil {
		return publish.Result{}, fmt.Errorf("ipfs: multipart write: %w", err)
	}
	if err := mp.Close(); err != nil {
		return publish.Result{}, fmt.Errorf("ipfs: multipart close: %w", err)
	}

	q := url.Values{}
	q.Set("cid-version", "1")
	q.Set("raw-leaves", "true")
	q.Set("hash", "sha2-256")
	q.Set("pin", boolStr(p.pin))
	endpoint := p.apiURL + "/api/v0/add?" + q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, body)
	if err != nil {
		return publish.Result{}, fmt.Errorf("ipfs: build request: %w", err)
	}
	req.Header.Set("Content-Type", mp.FormDataContentType())
	if p.authHeader != "" {
		req.Header.Set("Authorization", p.authHeader)
	}

	resp, err := p.http.Do(req)
	if err != nil {
		return publish.Result{}, fmt.Errorf("ipfs: request: %w", err)
	}
	defer resp.Body.Close()
	respBytes, _ := io.ReadAll(resp.Body)
	if resp.StatusCode/100 != 2 {
		return publish.Result{}, fmt.Errorf("ipfs: %s -> %d: %s",
			endpoint, resp.StatusCode, truncate(respBytes, 500))
	}

	// Kubo emits one JSON object per added entry, separated by newlines.
	// A single-file upload yields exactly one object; we tolerate trailing
	// directory wrapper entries by taking the last non-empty line.
	var add kuboAddResponse
	for _, line := range bytes.Split(respBytes, []byte("\n")) {
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}
		var entry kuboAddResponse
		if err := json.Unmarshal(line, &entry); err != nil {
			return publish.Result{}, fmt.Errorf("ipfs: parse add response %q: %w", line, err)
		}
		if entry.Hash != "" {
			add = entry
		}
	}
	if add.Hash == "" {
		return publish.Result{}, fmt.Errorf("ipfs: empty CID in response: %s", truncate(respBytes, 500))
	}

	res := publish.Result{
		Network:  "ipfs",
		Locator:  add.Hash,
		Size:     int64(len(data)),
		MimeType: mimeType,
	}
	if p.gateway != "" {
		res.URL = fmt.Sprintf("%s/ipfs/%s", p.gateway, add.Hash)
	}
	return res, nil
}

func boolStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

func truncate(b []byte, n int) string {
	if len(b) <= n {
		return string(b)
	}
	return string(b[:n]) + "..."
}
