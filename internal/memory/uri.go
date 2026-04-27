// Package memory — URI scheme for Pixelog knowledge artifacts.
//
// pixe:// URIs identify content-addressed pixe capsules, typed memory entries,
// agent-bound versions, and Arweave-pinned blobs.
package memory

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// URIScheme is the Pixelog knowledge artifact URI scheme.
const URIScheme = "pixe"

// URIKind identifies the type of resource a pixe:// URI points to.
type URIKind string

const (
	URIKindCapsule URIKind = "capsule" // pixe://capsule/{hash}
	URIKindMemory  URIKind = "memory"  // pixe://memory/{namespace}/{category}/{id}
	URIKindAgent   URIKind = "agent"   // pixe://agent/{tokenID}/capsule/{version}
	URIKindArweave URIKind = "arweave" // pixe://arweave/{txID}
)

// PixeURI represents a parsed pixe:// URI.
type PixeURI struct {
	Kind        URIKind
	Raw         string
	Hash        string // capsule content hash
	Namespace   string // memory namespace (per-agent or per-tenant)
	Category    string // memory category
	EntryID     string // memory entry ID
	TokenID     uint64 // agent NFT token ID (optional)
	Version     int    // capsule version number
	ArweaveTxID string
}

// ParseURI parses a pixe:// URI string into a structured PixeURI.
func ParseURI(raw string) (*PixeURI, error) {
	if raw == "" {
		return nil, fmt.Errorf("empty URI")
	}

	u, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("invalid URI: %w", err)
	}

	if u.Scheme != URIScheme {
		return nil, fmt.Errorf("expected scheme %q, got %q", URIScheme, u.Scheme)
	}

	host := u.Host
	path := strings.TrimPrefix(u.Path, "/")
	parts := strings.Split(path, "/")

	switch URIKind(host) {
	case URIKindCapsule:
		return parseCapsuleURI(raw, parts)
	case URIKindMemory:
		return parseMemoryURI(raw, parts)
	case URIKindAgent:
		return parseAgentURI(raw, parts)
	case URIKindArweave:
		return parseArweaveURI(raw, parts)
	default:
		return nil, fmt.Errorf("unknown URI kind: %q", host)
	}
}

func parseCapsuleURI(raw string, parts []string) (*PixeURI, error) {
	if len(parts) < 1 || parts[0] == "" {
		return nil, fmt.Errorf("capsule URI requires hash: pixe://capsule/{hash}")
	}
	return &PixeURI{
		Kind: URIKindCapsule,
		Raw:  raw,
		Hash: parts[0],
	}, nil
}

func parseMemoryURI(raw string, parts []string) (*PixeURI, error) {
	if len(parts) < 3 {
		return nil, fmt.Errorf("memory URI requires: pixe://memory/{namespace}/{category}/{id}")
	}
	category := parts[1]
	if !ValidCategory(category) {
		return nil, fmt.Errorf("invalid memory category: %q", category)
	}
	return &PixeURI{
		Kind:      URIKindMemory,
		Raw:       raw,
		Namespace: parts[0],
		Category:  category,
		EntryID:   parts[2],
	}, nil
}

func parseAgentURI(raw string, parts []string) (*PixeURI, error) {
	if len(parts) < 1 || parts[0] == "" {
		return nil, fmt.Errorf("agent URI requires tokenID: pixe://agent/{tokenID}")
	}
	tokenID, err := strconv.ParseUint(parts[0], 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid tokenID %q: %w", parts[0], err)
	}
	uri := &PixeURI{
		Kind:    URIKindAgent,
		Raw:     raw,
		TokenID: tokenID,
	}
	if len(parts) >= 3 && parts[1] == "capsule" {
		version, err := strconv.Atoi(parts[2])
		if err != nil {
			return nil, fmt.Errorf("invalid version %q: %w", parts[2], err)
		}
		uri.Version = version
	}
	return uri, nil
}

func parseArweaveURI(raw string, parts []string) (*PixeURI, error) {
	if len(parts) < 1 || parts[0] == "" {
		return nil, fmt.Errorf("arweave URI requires txID: pixe://arweave/{txID}")
	}
	return &PixeURI{
		Kind:        URIKindArweave,
		Raw:         raw,
		ArweaveTxID: parts[0],
	}, nil
}

// String returns the canonical URI string.
func (u *PixeURI) String() string {
	if u.Raw != "" {
		return u.Raw
	}
	switch u.Kind {
	case URIKindCapsule:
		return fmt.Sprintf("pixe://capsule/%s", u.Hash)
	case URIKindMemory:
		return fmt.Sprintf("pixe://memory/%s/%s/%s", u.Namespace, u.Category, u.EntryID)
	case URIKindAgent:
		if u.Version > 0 {
			return fmt.Sprintf("pixe://agent/%d/capsule/%d", u.TokenID, u.Version)
		}
		return fmt.Sprintf("pixe://agent/%d", u.TokenID)
	case URIKindArweave:
		return fmt.Sprintf("pixe://arweave/%s", u.ArweaveTxID)
	default:
		return u.Raw
	}
}

// BuildCapsuleURI constructs pixe://capsule/{hash}.
func BuildCapsuleURI(hash string) string {
	return fmt.Sprintf("pixe://capsule/%s", hash)
}

// BuildMemoryURI constructs pixe://memory/{namespace}/{category}/{id}.
func BuildMemoryURI(namespace string, category MemoryCategory, entryID string) string {
	return fmt.Sprintf("pixe://memory/%s/%s/%s", namespace, category, entryID)
}

// BuildAgentURI constructs pixe://agent/{tokenID}.
func BuildAgentURI(tokenID uint64) string {
	return fmt.Sprintf("pixe://agent/%d", tokenID)
}

// BuildAgentCapsuleURI constructs pixe://agent/{tokenID}/capsule/{version}.
func BuildAgentCapsuleURI(tokenID uint64, version int) string {
	return fmt.Sprintf("pixe://agent/%d/capsule/%d", tokenID, version)
}

// BuildArweaveURI constructs pixe://arweave/{txID}.
func BuildArweaveURI(txID string) string {
	return fmt.Sprintf("pixe://arweave/%s", txID)
}
