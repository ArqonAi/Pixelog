// Package publish defines the BlobPublisher contract used by the
// archival pipeline to persist .pixe capsules to durable, content-addressed
// networks (IPFS, Arweave, ...).
//
// Implementations are expected to be:
//   - Real: actually upload the bytes; never mock or stub.
//   - Idempotent: re-publishing identical bytes returns the same locator.
//   - Network-tagged: Network() returns a stable string ("ipfs", "arweave").
//   - Context-respecting: cancellation and deadlines are honored.
package publish

import "context"

// Publisher uploads opaque bytes to a content-addressed durability layer
// and returns a network-specific locator (CID, transaction ID, ...).
type Publisher interface {
	// Publish uploads data and returns a locator that can later be resolved
	// to the same bytes. mimeType is advisory; some networks (Arweave) store
	// it as a tag, others (IPFS) ignore it.
	Publish(ctx context.Context, data []byte, mimeType string) (Result, error)

	// Network returns a stable identifier for this publisher
	// (e.g. "ipfs", "arweave"). Used as a key in PublishResult maps.
	Network() string
}

// Result describes a single successful publication.
type Result struct {
	Network  string `json:"network"`            // "ipfs", "arweave"
	Locator  string `json:"locator"`            // CID, tx-id, ...
	URL      string `json:"url,omitempty"`      // optional gateway URL
	Size     int64  `json:"size"`               // bytes uploaded
	MimeType string `json:"mime_type,omitempty"`
}
