package memory

import "testing"

func TestParseURI_Capsule(t *testing.T) {
	uri, err := ParseURI("pixe://capsule/abc123def456")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if uri.Kind != URIKindCapsule {
		t.Errorf("expected kind %s, got %s", URIKindCapsule, uri.Kind)
	}
	if uri.Hash != "abc123def456" {
		t.Errorf("expected hash abc123def456, got %s", uri.Hash)
	}
}

func TestParseURI_Memory(t *testing.T) {
	uri, err := ParseURI("pixe://memory/ns-42/preference/pref-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if uri.Kind != URIKindMemory {
		t.Errorf("expected kind %s, got %s", URIKindMemory, uri.Kind)
	}
	if uri.Namespace != "ns-42" {
		t.Errorf("expected namespace ns-42, got %s", uri.Namespace)
	}
	if uri.Category != "preference" {
		t.Errorf("expected category preference, got %s", uri.Category)
	}
	if uri.EntryID != "pref-1" {
		t.Errorf("expected entryID pref-1, got %s", uri.EntryID)
	}
}

func TestParseURI_Agent(t *testing.T) {
	uri, err := ParseURI("pixe://agent/42/capsule/3")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if uri.Kind != URIKindAgent {
		t.Errorf("expected kind %s, got %s", URIKindAgent, uri.Kind)
	}
	if uri.TokenID != 42 {
		t.Errorf("expected tokenID 42, got %d", uri.TokenID)
	}
	if uri.Version != 3 {
		t.Errorf("expected version 3, got %d", uri.Version)
	}
}

func TestParseURI_AgentNoVersion(t *testing.T) {
	uri, err := ParseURI("pixe://agent/7")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if uri.TokenID != 7 {
		t.Errorf("expected tokenID 7, got %d", uri.TokenID)
	}
	if uri.Version != 0 {
		t.Errorf("expected version 0, got %d", uri.Version)
	}
}

func TestParseURI_Arweave(t *testing.T) {
	uri, err := ParseURI("pixe://arweave/txABC123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if uri.Kind != URIKindArweave {
		t.Errorf("expected kind %s, got %s", URIKindArweave, uri.Kind)
	}
	if uri.ArweaveTxID != "txABC123" {
		t.Errorf("expected arweaveTxID txABC123, got %s", uri.ArweaveTxID)
	}
}

func TestParseURI_Errors(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"empty", ""},
		{"wrong scheme", "http://capsule/abc"},
		{"unknown kind", "pixe://unknown/abc"},
		{"capsule no hash", "pixe://capsule/"},
		{"memory missing parts", "pixe://memory/ns"},
		{"memory invalid category", "pixe://memory/ns/badcat/id"},
		{"agent invalid tokenID", "pixe://agent/notanumber"},
		{"agent invalid version", "pixe://agent/1/capsule/notanumber"},
		{"arweave no txID", "pixe://arweave/"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseURI(tt.input)
			if err == nil {
				t.Errorf("expected error for input %q", tt.input)
			}
		})
	}
}

func TestBuildURIs(t *testing.T) {
	tests := []struct {
		name     string
		build    func() string
		expected string
	}{
		{"capsule", func() string { return BuildCapsuleURI("abc") }, "pixe://capsule/abc"},
		{"memory", func() string { return BuildMemoryURI("ns-1", CategoryFact, "f1") }, "pixe://memory/ns-1/fact/f1"},
		{"agent", func() string { return BuildAgentURI(42) }, "pixe://agent/42"},
		{"agent capsule", func() string { return BuildAgentCapsuleURI(42, 3) }, "pixe://agent/42/capsule/3"},
		{"arweave", func() string { return BuildArweaveURI("tx123") }, "pixe://arweave/tx123"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.build()
			if got != tt.expected {
				t.Errorf("got %q, want %q", got, tt.expected)
			}

			parsed, err := ParseURI(got)
			if err != nil {
				t.Fatalf("failed to parse built URI %q: %v", got, err)
			}
			if parsed.String() != tt.expected {
				t.Errorf("round-trip failed: %q -> %q", got, parsed.String())
			}
		})
	}
}

func TestPixeURI_String(t *testing.T) {
	uri := &PixeURI{Kind: URIKindCapsule, Hash: "abc123"}
	if uri.String() != "pixe://capsule/abc123" {
		t.Errorf("unexpected String(): %s", uri.String())
	}

	uri = &PixeURI{Kind: URIKindAgent, TokenID: 5, Version: 2}
	if uri.String() != "pixe://agent/5/capsule/2" {
		t.Errorf("unexpected String(): %s", uri.String())
	}

	uri = &PixeURI{Kind: URIKindAgent, TokenID: 5}
	if uri.String() != "pixe://agent/5" {
		t.Errorf("unexpected String(): %s", uri.String())
	}
}
