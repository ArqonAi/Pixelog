package enj

import (
	"crypto/ed25519"
	"encoding/hex"
	"time"
)

// AgentManifest is the top-level schema for an .enj file.
// An .enj file is a Pixelog .pixe video (MP4 + QR frames + AES-256-GCM)
// whose payload is a JSON-serialized AgentManifest instead of raw documents.
// This makes an Engajer avatar a portable, encrypted, self-contained AI persona.
type AgentManifest struct {
	// Format identification
	Format  string `json:"format"`  // "enj"
	Version string `json:"version"` // "1.0.0"

	// Identity — ed25519 keypair for cryptographic identity
	Identity AgentIdentity `json:"identity"`

	// Model configuration — which LLM to use and how
	Model AgentModel `json:"model"`

	// Knowledge — references to .pixe knowledge base files
	Knowledge []KnowledgeRef `json:"knowledge"`

	// Memory — conversation history, learned preferences, context
	Memory AgentMemory `json:"memory"`

	// Permissions — what this agent is allowed to do
	Permissions AgentPermissions `json:"permissions"`

	// Skills — registered capabilities and tool definitions
	Skills []AgentSkill `json:"skills"`

	// Metadata
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	CreatedBy string    `json:"created_by"` // hex public key of creator
}

// AgentIdentity holds the cryptographic identity of an Engajer avatar.
// The private key is encrypted with the owner's password before storage.
type AgentIdentity struct {
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
	PublicKey   string `json:"public_key"`  // hex-encoded ed25519 public key
	PrivateKey  string `json:"private_key"` // hex-encoded, encrypted with owner password
	Avatar      string `json:"avatar"`      // base64-encoded image or URL
	Bio         string `json:"bio"`
}

// AgentModel defines the LLM configuration for this agent.
type AgentModel struct {
	Alias        string  `json:"alias"`         // e.g. "engajer-1"
	BaseModel    string  `json:"base_model"`    // e.g. "deepseek-r1-32b"
	Quantization string  `json:"quantization"`  // e.g. "Q4_K_M"
	GGUFFile     string  `json:"gguf_file"`     // filename in models dir
	ContextLen   int     `json:"context_length"`
	Temperature  float64 `json:"temperature"`
	SystemPrompt string  `json:"system_prompt"` // custom system prompt for this agent
	GPULayers    int     `json:"gpu_layers"`
}

// KnowledgeRef references a .pixe knowledge base file stored in a Hyperdrive.
type KnowledgeRef struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	PixePath    string    `json:"pixe_path"`   // path within Hyperdrive
	DriveKey    string    `json:"drive_key"`   // Hyperdrive key containing this file
	Hash        string    `json:"hash"`        // SHA-256 of the .pixe file
	Size        int64     `json:"size"`
	IndexedAt   time.Time `json:"indexed_at"`
	ChunkCount  int       `json:"chunk_count"` // number of vector chunks indexed
}

// AgentMemory stores conversation history and learned context.
type AgentMemory struct {
	MaxEntries   int           `json:"max_entries"`
	Entries      []MemoryEntry `json:"entries"`
	Preferences  map[string]string `json:"preferences"`  // learned user preferences
	SystemNotes  []string          `json:"system_notes"` // persistent instructions
}

// MemoryEntry is a single memory item (conversation summary, fact, preference).
type MemoryEntry struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"` // "conversation", "fact", "preference", "instruction"
	Content   string    `json:"content"`
	Source    string    `json:"source"`    // where this memory came from
	CreatedAt time.Time `json:"created_at"`
	Weight    float64   `json:"weight"`    // importance score 0-1
}

// AgentPermissions defines what this agent is allowed to do.
type AgentPermissions struct {
	TrustLevel     string   `json:"trust_level"`      // "platinum", "gold", "silver", "bronze", "restricted"
	AllowedModules []string `json:"allowed_modules"`   // e.g. ["web_search", "file_read", "code_exec"]
	BlockedModules []string `json:"blocked_modules"`
	AllowRemote    bool     `json:"allow_remote"`      // can this agent serve remote inference
	AllowP2P       bool     `json:"allow_p2p"`         // can join P2P data rooms
	MaxTokens      int      `json:"max_tokens"`        // max tokens per response
	RateLimit      int      `json:"rate_limit_per_min"` // max requests per minute
}

// AgentSkill represents a registered capability.
type AgentSkill struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Type        string                 `json:"type"` // "tool", "workflow", "plugin"
	Schema      map[string]interface{} `json:"schema,omitempty"` // JSON schema for tool parameters
	Enabled     bool                   `json:"enabled"`
}

// GenerateIdentity creates a new ed25519 keypair for an agent.
func GenerateIdentity(name, displayName string) (*AgentIdentity, ed25519.PrivateKey) {
	pub, priv, _ := ed25519.GenerateKey(nil)
	return &AgentIdentity{
		Name:        name,
		DisplayName: displayName,
		PublicKey:   hex.EncodeToString(pub),
	}, priv
}

// DefaultManifest creates a new AgentManifest with sensible defaults for Engajer 1.
func DefaultManifest(name, displayName string) *AgentManifest {
	identity, priv := GenerateIdentity(name, displayName)
	identity.PrivateKey = hex.EncodeToString(priv) // will be encrypted before storage

	return &AgentManifest{
		Format:  "enj",
		Version: "1.0.0",
		Identity: *identity,
		Model: AgentModel{
			Alias:        "engajer-1",
			BaseModel:    "deepseek-r1-32b",
			Quantization: "Q4_K_M",
			GGUFFile:     "deepseek-r1-32b-Q4_K_M.gguf",
			ContextLen:   4096,
			Temperature:  0.7,
			GPULayers:    99,
		},
		Knowledge: []KnowledgeRef{},
		Memory: AgentMemory{
			MaxEntries:  1000,
			Entries:     []MemoryEntry{},
			Preferences: make(map[string]string),
			SystemNotes: []string{},
		},
		Permissions: AgentPermissions{
			TrustLevel:     "gold",
			AllowedModules: []string{"web_search", "file_read", "file_write", "code_exec", "calculator"},
			BlockedModules: []string{},
			AllowRemote:    true,
			AllowP2P:       true,
			MaxTokens:      4096,
			RateLimit:      60,
		},
		Skills:    []AgentSkill{},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		CreatedBy: identity.PublicKey,
	}
}
