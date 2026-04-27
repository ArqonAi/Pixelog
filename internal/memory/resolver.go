package memory

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// CapsuleResolver fetches a capsule by `pixe://capsule/<hash>` URI,
// transparently spanning local SSD → IPFS → Arweave. Each backend is
// optional; a Resolver with only a local store still works (returning
// CapsuleStoreErrNotFound for misses).
//
// The resolver is the substrate that makes the fractal storage tiering
// real: an agent never knows or cares which tier holds a given era.
type CapsuleResolver struct {
	local *CapsuleStore

	// Backend list, queried in order. First successful fetch wins;
	// successful remote fetches are written back to local for next time.
	backends []ResolverBackend

	http *http.Client
	mu   sync.Mutex
}

// ResolverBackend is a remote source for capsule bytes. Implementations
// resolve a hash to raw JSON bytes (the EraCapsule serialization).
type ResolverBackend interface {
	// Fetch returns the raw capsule body for hash, or an error.
	Fetch(ctx context.Context, hash string) ([]byte, error)
	// Name identifies the backend for diagnostics ("ipfs", "arweave", ...).
	Name() string
}

// NewCapsuleResolver builds a resolver with a local store and an
// ordered list of remote backends.
func NewCapsuleResolver(local *CapsuleStore, backends ...ResolverBackend) *CapsuleResolver {
	return &CapsuleResolver{
		local:    local,
		backends: backends,
		http:     &http.Client{Timeout: 30 * time.Second},
	}
}

// Resolve returns a parsed EraCapsule for the supplied URI.
// Successful remote hits are written to the local store for warm reuse.
func (r *CapsuleResolver) Resolve(ctx context.Context, uri string) (*EraCapsule, error) {
	hash, err := hashFromURI(uri)
	if err != nil {
		return nil, err
	}

	// 1. Local fast path.
	if r.local != nil {
		if ec, err := r.local.GetEra(ctx, hash); err == nil {
			return ec, nil
		} else if !errors.Is(err, CapsuleStoreErrNotFound) {
			return nil, err
		}
	}

	// 2. Walk backends; first success wins.
	var lastErr error
	for _, b := range r.backends {
		body, err := b.Fetch(ctx, hash)
		if err != nil {
			lastErr = fmt.Errorf("%s: %w", b.Name(), err)
			continue
		}
		var ec EraCapsule
		if err := json.Unmarshal(body, &ec); err != nil {
			lastErr = fmt.Errorf("%s: parse: %w", b.Name(), err)
			continue
		}
		// Warm the local cache via PutEra so the index is updated, not
		// just the blob. Cache failure is non-fatal.
		if r.local != nil {
			_, _ = r.local.PutEra(&ec)
		}
		return &ec, nil
	}

	if lastErr == nil {
		return nil, CapsuleStoreErrNotFound
	}
	return nil, fmt.Errorf("resolver: all backends failed: %w", lastErr)
}

// hashFromURI extracts the hash component from pixe://capsule/<hash>.
func hashFromURI(uri string) (string, error) {
	const prefix = "pixe://capsule/"
	if !strings.HasPrefix(uri, prefix) {
		return "", fmt.Errorf("resolver: not a capsule URI: %q", uri)
	}
	hash := strings.TrimPrefix(uri, prefix)
	if hash == "" {
		return "", fmt.Errorf("resolver: empty hash in URI %q", uri)
	}
	// Strip any trailing path segments (defence against malformed URIs).
	if i := strings.IndexByte(hash, '/'); i >= 0 {
		hash = hash[:i]
	}
	return hash, nil
}

// IPFSGatewayBackend reads capsules from an HTTP IPFS gateway. The
// resolver matches by the hash component of pixe://capsule/<hash>; the
// caller must have published the capsule under that same content
// address (CID) or supply a HashToCID translator.
type IPFSGatewayBackend struct {
	GatewayURL string // e.g. "http://127.0.0.1:8080"
	HashToCID  func(hash string) (cid string, err error)
	HTTPClient *http.Client
}

// Name implements ResolverBackend.
func (b *IPFSGatewayBackend) Name() string { return "ipfs" }

// Fetch implements ResolverBackend.
func (b *IPFSGatewayBackend) Fetch(ctx context.Context, hash string) ([]byte, error) {
	cid, err := b.HashToCID(hash)
	if err != nil {
		return nil, fmt.Errorf("hash→cid: %w", err)
	}
	url := strings.TrimRight(b.GatewayURL, "/") + "/ipfs/" + cid
	return doGET(ctx, b.HTTPClient, url)
}

// ArweaveBackend reads capsules from an Arweave HTTP node by Arweave tx-id.
type ArweaveBackend struct {
	NodeURL    string // e.g. "https://arweave.net"
	HashToTxID func(hash string) (txID string, err error)
	HTTPClient *http.Client
}

// Name implements ResolverBackend.
func (b *ArweaveBackend) Name() string { return "arweave" }

// Fetch implements ResolverBackend.
func (b *ArweaveBackend) Fetch(ctx context.Context, hash string) ([]byte, error) {
	txID, err := b.HashToTxID(hash)
	if err != nil {
		return nil, fmt.Errorf("hash→txID: %w", err)
	}
	url := strings.TrimRight(b.NodeURL, "/") + "/" + txID
	return doGET(ctx, b.HTTPClient, url)
}

func doGET(ctx context.Context, hc *http.Client, url string) ([]byte, error) {
	if hc == nil {
		hc = &http.Client{Timeout: 30 * time.Second}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := hc.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode/100 != 2 {
		return nil, fmt.Errorf("GET %s -> %d: %s", url, resp.StatusCode, truncateForErr(body, 200))
	}
	return body, nil
}

func truncateForErr(b []byte, n int) string {
	if len(b) <= n {
		return string(b)
	}
	return string(b[:n]) + "..."
}
