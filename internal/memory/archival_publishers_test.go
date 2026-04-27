package memory

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"testing"

	"github.com/ArqonAi/Pixelog/pkg/publish"
)

// fakePublisher is a deterministic, in-memory BlobPublisher for exercising
// the fan-out path without touching the network.
type fakePublisher struct {
	mu        sync.Mutex
	network   string
	calls     int
	failWith  error
	locatorFn func([]byte) string
}

func (f *fakePublisher) Network() string { return f.network }

func (f *fakePublisher) Publish(_ context.Context, data []byte, mime string) (publish.Result, error) {
	f.mu.Lock()
	f.calls++
	f.mu.Unlock()
	if f.failWith != nil {
		return publish.Result{}, f.failWith
	}
	return publish.Result{
		Network:  f.network,
		Locator:  f.locatorFn(data),
		Size:     int64(len(data)),
		MimeType: mime,
	}, nil
}

// TestArchive_FansOutToBlobPublishers confirms that the Archive phase
// reads the capsule and dispatches it to every configured publisher.
func TestArchive_FansOutToBlobPublishers(t *testing.T) {
	tmp := t.TempDir()
	capsule := filepath.Join(tmp, "capsule.pixe")
	payload := []byte("pixelog capsule bytes — arbitrary content")
	if err := os.WriteFile(capsule, payload, 0o600); err != nil {
		t.Fatalf("write capsule: %v", err)
	}

	ipfs := &fakePublisher{network: "ipfs", locatorFn: func(b []byte) string { return "bafyIPFS" }}
	arw := &fakePublisher{network: "arweave", locatorFn: func(b []byte) string { return "txARWEAVE" }}

	p := NewArchivalPipeline(&ArchivalConfig{
		Namespace: "test-ns",
		Converter: func(_ context.Context, path string) (string, error) {
			return path, nil // pass-through: treat input as the capsule
		},
		BlobPublishers: []publish.Publisher{ipfs, arw},
	})

	compressed := &CompressResult{Summary: "summary text"}
	res, err := p.Archive(context.Background(), capsule, compressed)
	if err != nil {
		t.Fatalf("Archive: %v", err)
	}

	if ipfs.calls != 1 || arw.calls != 1 {
		t.Errorf("publisher call counts = ipfs:%d arweave:%d; want 1 each",
			ipfs.calls, arw.calls)
	}
	if len(res.Publications) != 2 {
		t.Fatalf("got %d publications, want 2", len(res.Publications))
	}

	// Order is non-deterministic (parallel fan-out); sort by Network for comparison.
	sort.Slice(res.Publications, func(i, j int) bool {
		return res.Publications[i].Network < res.Publications[j].Network
	})
	if res.Publications[0].Network != "arweave" || res.Publications[0].Locator != "txARWEAVE" {
		t.Errorf("arweave publication = %+v", res.Publications[0])
	}
	if res.Publications[1].Network != "ipfs" || res.Publications[1].Locator != "bafyIPFS" {
		t.Errorf("ipfs publication = %+v", res.Publications[1])
	}
	if len(res.PublishErrors) != 0 {
		t.Errorf("unexpected publish errors: %v", res.PublishErrors)
	}
}

// TestArchive_RecordsPartialFailure ensures one publisher's failure
// surfaces in PublishErrors without aborting the pipeline or losing the
// other publisher's success.
func TestArchive_RecordsPartialFailure(t *testing.T) {
	tmp := t.TempDir()
	capsule := filepath.Join(tmp, "capsule.pixe")
	_ = os.WriteFile(capsule, []byte("x"), 0o600)

	good := &fakePublisher{network: "ipfs", locatorFn: func(b []byte) string { return "ok" }}
	bad := &fakePublisher{network: "arweave", failWith: errors.New("node unreachable")}

	p := NewArchivalPipeline(&ArchivalConfig{
		Converter:      func(_ context.Context, path string) (string, error) { return path, nil },
		BlobPublishers: []publish.Publisher{good, bad},
	})
	res, err := p.Archive(context.Background(), capsule, &CompressResult{Summary: "s"})
	if err != nil {
		t.Fatalf("Archive: %v", err)
	}
	if len(res.Publications) != 1 || res.Publications[0].Network != "ipfs" {
		t.Errorf("expected 1 ipfs publication, got %+v", res.Publications)
	}
	if res.PublishErrors["arweave"] == "" {
		t.Errorf("expected PublishErrors[arweave] to be recorded, got %v", res.PublishErrors)
	}
}

// TestArchive_NoPublishersIsNoop guards the common case where no
// durability layer is configured: no errors, no Publications.
func TestArchive_NoPublishersIsNoop(t *testing.T) {
	tmp := t.TempDir()
	capsule := filepath.Join(tmp, "capsule.pixe")
	_ = os.WriteFile(capsule, []byte("x"), 0o600)

	p := NewArchivalPipeline(&ArchivalConfig{
		Converter: func(_ context.Context, path string) (string, error) { return path, nil },
	})
	res, err := p.Archive(context.Background(), capsule, &CompressResult{Summary: "s"})
	if err != nil {
		t.Fatalf("Archive: %v", err)
	}
	if len(res.Publications) != 0 || len(res.PublishErrors) != 0 {
		t.Errorf("no-publisher path should not populate publications/errors; got %+v", res)
	}
}
