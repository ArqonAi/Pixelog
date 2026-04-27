package memory

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// TestResolver_LocalFastPath: a hash present locally never hits the network.
func TestResolver_LocalFastPath(t *testing.T) {
	store, _ := NewCapsuleStore(t.TempDir())
	ec := NewEraCapsule("ns", LevelDay, []ChildRef{mkChild("x", 1)})
	ec.L0 = "local-only"
	hash, _ := store.PutEra(ec)

	failBackend := &countingBackend{
		fetchFn: func(ctx context.Context, h string) ([]byte, error) {
			return nil, errors.New("should not be called")
		},
		name: "ipfs",
	}
	r := NewCapsuleResolver(store, failBackend)
	got, err := r.Resolve(context.Background(), "pixe://capsule/"+hash)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got.L0 != "local-only" {
		t.Errorf("L0 = %q", got.L0)
	}
	if failBackend.calls != 0 {
		t.Errorf("backend called %d times; should be zero", failBackend.calls)
	}
}

// TestResolver_FallthroughAndCacheBackfill: a missing local hash triggers
// the IPFS backend; on success the body is cached locally for next time.
func TestResolver_FallthroughAndCacheBackfill(t *testing.T) {
	store, _ := NewCapsuleStore(t.TempDir())
	remoteEra := NewEraCapsule("ns", LevelDay, []ChildRef{mkChild("y", 1)})
	remoteEra.L0 = "remote-only"
	remoteEra.Finalize()
	body, _ := json.Marshal(remoteEra)

	backend := &countingBackend{
		fetchFn: func(ctx context.Context, h string) ([]byte, error) {
			if h != remoteEra.Hash {
				return nil, fmt.Errorf("unknown hash %s", h)
			}
			return body, nil
		},
		name: "ipfs",
	}
	r := NewCapsuleResolver(store, backend)

	got, err := r.Resolve(context.Background(), "pixe://capsule/"+remoteEra.Hash)
	if err != nil {
		t.Fatalf("first Resolve: %v", err)
	}
	if got.L0 != "remote-only" {
		t.Errorf("L0 = %q", got.L0)
	}
	if backend.calls != 1 {
		t.Errorf("backend.calls = %d, want 1", backend.calls)
	}
	if !store.HasHash(remoteEra.Hash) {
		t.Error("local cache not warmed after remote hit")
	}

	// Second resolve must come from local cache, not the backend.
	if _, err := r.Resolve(context.Background(), "pixe://capsule/"+remoteEra.Hash); err != nil {
		t.Fatalf("second Resolve: %v", err)
	}
	if backend.calls != 1 {
		t.Errorf("backend.calls after cache hit = %d, want still 1", backend.calls)
	}
}

// TestResolver_AllBackendsFail surfaces a useful aggregated error.
func TestResolver_AllBackendsFail(t *testing.T) {
	r := NewCapsuleResolver(nil,
		&countingBackend{fetchFn: func(ctx context.Context, h string) ([]byte, error) { return nil, errors.New("ipfs down") }, name: "ipfs"},
		&countingBackend{fetchFn: func(ctx context.Context, h string) ([]byte, error) { return nil, errors.New("arweave down") }, name: "arweave"},
	)
	_, err := r.Resolve(context.Background(), "pixe://capsule/missing")
	if err == nil {
		t.Fatal("expected error when all backends fail")
	}
}

// TestResolver_BadURI rejects non-capsule URIs.
func TestResolver_BadURI(t *testing.T) {
	r := NewCapsuleResolver(nil)
	if _, err := r.Resolve(context.Background(), "http://example.com/x"); err == nil {
		t.Error("expected error for non-pixe URI")
	}
}

// TestIPFSGatewayBackend_Fetch covers the real HTTP path against an
// httptest server emulating an IPFS gateway.
func TestIPFSGatewayBackend_Fetch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/ipfs/Qmtestcid" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(`{"version":1,"hash":"abc"}`))
	}))
	defer srv.Close()

	b := &IPFSGatewayBackend{
		GatewayURL: srv.URL,
		HashToCID:  func(hash string) (string, error) { return "Qmtestcid", nil },
		HTTPClient: &http.Client{Timeout: 5 * time.Second},
	}
	got, err := b.Fetch(context.Background(), "abc")
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if string(got) != `{"version":1,"hash":"abc"}` {
		t.Errorf("body = %q", got)
	}
	if b.Name() != "ipfs" {
		t.Errorf("Name = %q", b.Name())
	}
}

// TestArweaveBackend_Fetch covers the Arweave HTTP path.
func TestArweaveBackend_Fetch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/txABC" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(`{"v":2}`))
	}))
	defer srv.Close()

	b := &ArweaveBackend{
		NodeURL:    srv.URL,
		HashToTxID: func(hash string) (string, error) { return "txABC", nil },
	}
	got, err := b.Fetch(context.Background(), "any")
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if string(got) != `{"v":2}` {
		t.Errorf("body = %q", got)
	}
}

// countingBackend is a minimal ResolverBackend for unit tests.
type countingBackend struct {
	fetchFn func(ctx context.Context, hash string) ([]byte, error)
	name    string
	calls   int
}

func (c *countingBackend) Fetch(ctx context.Context, hash string) ([]byte, error) {
	c.calls++
	return c.fetchFn(ctx, hash)
}

func (c *countingBackend) Name() string { return c.name }
