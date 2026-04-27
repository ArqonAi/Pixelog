package memory

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"sync"
)

// CapsuleStoreErrNotFound is returned when a hash is not present in the store.
var CapsuleStoreErrNotFound = errors.New("capsule store: not found")

// CapsuleStore is a content-addressed local store for EraCapsules and
// any other JSON-serialised capsule shape we want to keep on disk.
//
// Layout:
//
//	<root>/
//	  capsules/
//	    <2-byte prefix>/
//	      <full-hash>.json
//	  index.json   // hash -> Level, Namespace, StartedAt, EndedAt
//
// The 2-byte fanout prevents huge directory listings at fleet scale
// (256 buckets, ≈1k entries each at 256k capsules total).
type CapsuleStore struct {
	root  string
	mu    sync.RWMutex
	index map[string]CapsuleIndex
}

// CapsuleIndex is the lightweight metadata kept in memory for fast
// listing without re-parsing every capsule.
type CapsuleIndex struct {
	Hash      string   `json:"hash"`
	Namespace string   `json:"namespace"`
	Level     EraLevel `json:"level"`
	StartedAt int64    `json:"started_at"` // unix nanos
	EndedAt   int64    `json:"ended_at"`
	CreatedAt int64    `json:"created_at"`
}

// NewCapsuleStore opens or creates a store rooted at dir.
func NewCapsuleStore(dir string) (*CapsuleStore, error) {
	if err := os.MkdirAll(filepath.Join(dir, "capsules"), 0o755); err != nil {
		return nil, fmt.Errorf("capsule store: mkdir: %w", err)
	}
	cs := &CapsuleStore{root: dir, index: map[string]CapsuleIndex{}}
	if err := cs.loadIndex(); err != nil {
		return nil, err
	}
	return cs, nil
}

// PutEra serialises ec, computes its hash if missing, writes it under
// the bucket directory, and updates the index. Returns the canonical hash.
func (cs *CapsuleStore) PutEra(ec *EraCapsule) (string, error) {
	if ec.Hash == "" {
		if _, err := ec.Finalize(); err != nil {
			return "", err
		}
	}
	body, err := json.MarshalIndent(ec, "", "  ")
	if err != nil {
		return "", fmt.Errorf("capsule store: marshal: %w", err)
	}
	if err := cs.writeBlob(ec.Hash, body); err != nil {
		return "", err
	}
	cs.mu.Lock()
	cs.index[ec.Hash] = CapsuleIndex{
		Hash:      ec.Hash,
		Namespace: ec.Namespace,
		Level:     ec.Level,
		StartedAt: ec.StartedAt.UnixNano(),
		EndedAt:   ec.EndedAt.UnixNano(),
		CreatedAt: ec.CreatedAt.UnixNano(),
	}
	cs.mu.Unlock()
	if err := cs.persistIndex(); err != nil {
		return "", err
	}
	return ec.Hash, nil
}

// GetEra reads, parses, and returns an EraCapsule by hash.
// Returns CapsuleStoreErrNotFound if the capsule is not present locally.
func (cs *CapsuleStore) GetEra(_ context.Context, hash string) (*EraCapsule, error) {
	body, err := cs.readBlob(hash)
	if err != nil {
		return nil, err
	}
	var ec EraCapsule
	if err := json.Unmarshal(body, &ec); err != nil {
		return nil, fmt.Errorf("capsule store: parse %s: %w", hash, err)
	}
	return &ec, nil
}

// HasHash reports whether a hash is present locally without reading the body.
func (cs *CapsuleStore) HasHash(hash string) bool {
	cs.mu.RLock()
	_, ok := cs.index[hash]
	cs.mu.RUnlock()
	return ok
}

// ListByLevel returns indices for all capsules at the given level in the
// given namespace, sorted by StartedAt ascending.
func (cs *CapsuleStore) ListByLevel(namespace string, level EraLevel) []CapsuleIndex {
	cs.mu.RLock()
	defer cs.mu.RUnlock()
	var out []CapsuleIndex
	for _, idx := range cs.index {
		if idx.Namespace == namespace && idx.Level == level {
			out = append(out, idx)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].StartedAt < out[j].StartedAt })
	return out
}

// Delete removes a capsule from disk and the index. Idempotent.
func (cs *CapsuleStore) Delete(hash string) error {
	path := cs.blobPath(hash)
	if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("capsule store: remove: %w", err)
	}
	cs.mu.Lock()
	delete(cs.index, hash)
	cs.mu.Unlock()
	return cs.persistIndex()
}

// PutBlob stores arbitrary content-addressed bytes under the supplied
// hash. Used by the resolver to cache capsules fetched from IPFS/Arweave.
func (cs *CapsuleStore) PutBlob(hash string, body []byte) error {
	return cs.writeBlob(hash, body)
}

// GetBlob returns raw bytes for a hash without parsing. Suitable when
// the caller only needs to forward the body to a publisher.
func (cs *CapsuleStore) GetBlob(hash string) ([]byte, error) {
	return cs.readBlob(hash)
}

func (cs *CapsuleStore) writeBlob(hash string, body []byte) error {
	if len(hash) < 4 {
		return fmt.Errorf("capsule store: hash too short %q", hash)
	}
	dir := filepath.Join(cs.root, "capsules", hash[:2])
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("capsule store: mkdir bucket: %w", err)
	}
	path := filepath.Join(dir, hash+".json")
	if err := os.WriteFile(path, body, 0o600); err != nil {
		return fmt.Errorf("capsule store: write: %w", err)
	}
	return nil
}

func (cs *CapsuleStore) readBlob(hash string) ([]byte, error) {
	body, err := os.ReadFile(cs.blobPath(hash))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, CapsuleStoreErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("capsule store: read %s: %w", hash, err)
	}
	return body, nil
}

func (cs *CapsuleStore) blobPath(hash string) string {
	if len(hash) < 2 {
		return filepath.Join(cs.root, "capsules", hash+".json")
	}
	return filepath.Join(cs.root, "capsules", hash[:2], hash+".json")
}

func (cs *CapsuleStore) indexPath() string {
	return filepath.Join(cs.root, "index.json")
}

func (cs *CapsuleStore) loadIndex() error {
	body, err := os.ReadFile(cs.indexPath())
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("capsule store: read index: %w", err)
	}
	var idx map[string]CapsuleIndex
	if err := json.Unmarshal(body, &idx); err != nil {
		return fmt.Errorf("capsule store: parse index: %w", err)
	}
	cs.index = idx
	return nil
}

func (cs *CapsuleStore) persistIndex() error {
	cs.mu.RLock()
	body, err := json.MarshalIndent(cs.index, "", "  ")
	cs.mu.RUnlock()
	if err != nil {
		return fmt.Errorf("capsule store: marshal index: %w", err)
	}
	tmp := cs.indexPath() + ".tmp"
	if err := os.WriteFile(tmp, body, 0o600); err != nil {
		return fmt.Errorf("capsule store: write index tmp: %w", err)
	}
	// Atomic rename: avoids torn writes if the process dies mid-write.
	return os.Rename(tmp, cs.indexPath())
}
