package index

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// This embedder is provider-agnostic by construction. A provider is just a
// named EndpointSpec: base URL + wire dialect + auth style + default model.
// Nothing below privileges any vendor — local inference servers (vims, vLLM,
// Ollama, llama.cpp, TEI, LocalAI, LM Studio) and hosted APIs are all
// expressed through the same table, and every field can be overridden at
// runtime via flags or PIXE_EMBED_* environment variables.

// Wire dialects — request/response shapes that exist in the wild.
const (
	// dialectStandard: POST {model,input} -> {data:[{embedding:[...]}]}.
	// The de facto industry shape implemented by virtually every
	// inference server, local or hosted.
	dialectStandard = "standard"
	// dialectGemini: Google's embedContent shape.
	dialectGemini = "gemini"
	// dialectOllamaNative: POST /api/embeddings {model,prompt} ->
	// {embedding:[...]}. Fallback for older Ollama builds without the
	// standard route.
	dialectOllamaNative = "ollama-native"
)

// EndpointSpec fully describes how to reach an embedding endpoint.
type EndpointSpec struct {
	BaseURL      string            // scheme://host[:port][/prefix]
	Path         string            // route appended to BaseURL
	Dialect      string            // wire dialect (see constants above)
	AuthHeader   string            // header name carrying the credential ("" = none)
	AuthPrefix   string            // value prefix, e.g. "Bearer "
	RequireKey   bool              // endpoint rejects unauthenticated calls
	DefaultModel string            // model when caller passes "" or "auto"
	KnownDim     int               // expected dim for DefaultModel (0 = discover)
	ExtraHeaders map[string]string // static headers (e.g. attribution)
}

// envOr returns the first non-empty environment variable value.
func envOr(names ...string) string {
	for _, n := range names {
		if v := os.Getenv(n); v != "" {
			return v
		}
	}
	return ""
}

// normalizeBase trims trailing slashes so Path concatenation is stable.
func normalizeBase(u string) string {
	return strings.TrimRight(u, "/")
}

// providerSpec resolves a provider name to its EndpointSpec. Every value is
// env-overridable; unknown names fall back to a pure-custom spec driven
// entirely by PIXE_EMBED_* variables.
func providerSpec(provider string) EndpointSpec {
	base := envOr("PIXE_EMBED_BASE_URL")
	switch strings.ToLower(provider) {
	case "vims":
		if base == "" {
			base = envOr("VIMS_GATEWAY_URL")
		}
		if base == "" {
			base = "http://127.0.0.1:8089/v1"
		}
		return EndpointSpec{BaseURL: normalizeBase(base), Path: "/embeddings", Dialect: dialectStandard, AuthHeader: "Authorization", AuthPrefix: "Bearer ", DefaultModel: "auto"}
	case "vllm":
		if base == "" {
			base = envOr("VLLM_ENDPOINT")
		}
		if base == "" {
			base = "http://127.0.0.1:8000/v1"
		}
		return EndpointSpec{BaseURL: normalizeBase(base), Path: "/embeddings", Dialect: dialectStandard, AuthHeader: "Authorization", AuthPrefix: "Bearer ", DefaultModel: "auto"}
	case "ollama":
		if base == "" {
			base = envOr("OLLAMA_HOST")
		}
		if base == "" {
			base = "http://127.0.0.1:11434"
		}
		base = normalizeBase(base)
		if !strings.HasSuffix(base, "/v1") {
			base += "/v1"
		}
		return EndpointSpec{BaseURL: base, Path: "/embeddings", Dialect: dialectStandard, DefaultModel: envOr("PIXE_EMBED_MODEL", "OLLAMA_EMBED_MODEL")}
	case "openai":
		if base == "" {
			base = "https://api.openai.com/v1"
		}
		return EndpointSpec{BaseURL: normalizeBase(base), Path: "/embeddings", Dialect: dialectStandard, AuthHeader: "Authorization", AuthPrefix: "Bearer ", RequireKey: true, DefaultModel: "text-embedding-3-large", KnownDim: 3072}
	case "openrouter", "anthropic", "claude", "xai", "grok":
		// anthropic/xai expose no embedding endpoints; their keys ride
		// the OpenRouter route the CLI historically used for them.
		if base == "" {
			base = "https://openrouter.ai/api/v1"
		}
		return EndpointSpec{BaseURL: normalizeBase(base), Path: "/embeddings", Dialect: dialectStandard, AuthHeader: "Authorization", AuthPrefix: "Bearer ", RequireKey: true, DefaultModel: "openai/text-embedding-3-large", KnownDim: 3072, ExtraHeaders: map[string]string{"HTTP-Referer": "https://github.com/ArqonAi/Pixelog"}}
	case "google", "gemini":
		if base == "" {
			base = "https://generativelanguage.googleapis.com/v1beta"
		}
		return EndpointSpec{BaseURL: normalizeBase(base), Path: "", Dialect: dialectGemini, RequireKey: true, DefaultModel: "models/text-embedding-004", KnownDim: 768}
	default:
		// Fully custom endpoint: everything from environment.
		if base == "" {
			base = "http://127.0.0.1:8089/v1"
		}
		return EndpointSpec{BaseURL: normalizeBase(base), Path: "/embeddings", Dialect: envOr("PIXE_EMBED_DIALECT"), AuthHeader: "Authorization", AuthPrefix: "Bearer ", DefaultModel: envOr("PIXE_EMBED_MODEL")}
	}
}

// KeyRequired reports whether the given provider cannot operate without a
// credential. Local/self-hosted endpoints work keyless.
func KeyRequired(provider string) bool {
	return providerSpec(provider).RequireKey
}

// GetDefaultModel returns the default embedding model and expected dimension
// for a provider (0 dimension means "discovered on first call").
func GetDefaultModel(provider string) (string, int) {
	spec := providerSpec(provider)
	model := envOr("PIXE_EMBED_MODEL")
	if model == "" {
		model = spec.DefaultModel
	}
	return model, spec.KnownDim
}

// SimpleEmbedder talks to any embedding endpoint described by an
// EndpointSpec. Dimension is discovered from the first successful call when
// not statically known.
type SimpleEmbedder struct {
	provider string
	spec     EndpointSpec
	apiKey   string
	model    string
	dim      int
	client   *http.Client
}

// NewSimpleEmbedder creates an embedder for the named provider. The key is
// optional for endpoints that do not require one (local inference servers).
func NewSimpleEmbedder(provider, apiKey, model string) *SimpleEmbedder {
	spec := providerSpec(provider)
	if apiKey == "" {
		apiKey = envOr("PIXE_EMBED_API_KEY")
	}
	if model == "" || model == "auto" {
		model = envOr("PIXE_EMBED_MODEL")
	}
	dim := 0
	if model == "" || model == "auto" {
		model = spec.DefaultModel
		dim = spec.KnownDim
	} else {
		switch {
		case strings.Contains(model, "minilm"):
			dim = 384
		case strings.Contains(model, "3-small"):
			dim = 1536
		case strings.Contains(model, "3-large"):
			dim = 3072
		case strings.Contains(model, "embedding-004"):
			dim = 768
		}
	}
	return &SimpleEmbedder{
		provider: strings.ToLower(provider),
		spec:     spec,
		apiKey:   apiKey,
		model:    model,
		dim:      dim,
		client:   &http.Client{Timeout: 120 * time.Second},
	}
}

// Dim returns the vector dimension (0 until discovered for dynamic
// endpoints; callers reading it after a successful Embed always see the
// real value).
func (e *SimpleEmbedder) Dim() int {
	return e.dim
}

// resolveModel lazily discovers a concrete model when the caller asked for
// "auto" on an endpoint that has no static default. It queries the
// endpoint's own /models listing — the discovery route every inference
// server exposes — so no vendor-specific logic is involved.
func (e *SimpleEmbedder) resolveModel() string {
	if e.model != "" && e.model != "auto" {
		return e.model
	}
	req, err := http.NewRequest("GET", e.spec.BaseURL+"/models", nil)
	if err != nil {
		return e.model
	}
	if e.apiKey != "" && e.spec.AuthHeader != "" {
		req.Header.Set(e.spec.AuthHeader, e.spec.AuthPrefix+e.apiKey)
	}
	resp, err := e.client.Do(req)
	if err != nil {
		return e.model
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return e.model
	}
	var list struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if json.NewDecoder(resp.Body).Decode(&list) != nil || len(list.Data) == 0 {
		return e.model
	}
	// Prefer a model that self-describes as an embedder; otherwise use the
	// first listed model (single-model servers like vLLM serve exactly one).
	for _, m := range list.Data {
		lower := strings.ToLower(m.ID)
		if strings.Contains(lower, "embed") || strings.Contains(lower, "bge") || strings.Contains(lower, "minilm") || strings.Contains(lower, "e5") {
			e.model = m.ID
			return e.model
		}
	}
	e.model = list.Data[0].ID
	return e.model
}

// Embed generates the embedding vector for the given text.
func (e *SimpleEmbedder) Embed(text string) ([]float32, error) {
	if len(text) > 8000 {
		text = text[:8000]
	}
	if e.spec.RequireKey && e.apiKey == "" {
		return nil, fmt.Errorf("provider %q requires an API key (set --api-key or PIXE_EMBED_API_KEY)", e.provider)
	}

	var vec []float32
	var err error
	switch e.spec.Dialect {
	case dialectGemini:
		vec, err = e.embedGemini(text)
	case dialectOllamaNative:
		vec, err = e.embedOllamaNative(text)
	default:
		vec, err = e.embedStandard(text)
		// Older Ollama builds lack the standard route — retry native.
		if err != nil && e.provider == "ollama" {
			if nvec, nerr := e.embedOllamaNative(text); nerr == nil {
				vec, err = nvec, nil
			}
		}
	}
	if err != nil {
		return nil, err
	}
	if len(vec) == 0 {
		return nil, fmt.Errorf("no embedding returned by %s", e.provider)
	}
	if e.dim == 0 {
		e.dim = len(vec)
	}
	return vec, nil
}

// post issues a JSON POST and returns the response body on HTTP 200.
func (e *SimpleEmbedder) post(url string, payload interface{}, auth bool) ([]byte, error) {
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if auth && e.apiKey != "" && e.spec.AuthHeader != "" {
		req.Header.Set(e.spec.AuthHeader, e.spec.AuthPrefix+e.apiKey)
	}
	for k, v := range e.spec.ExtraHeaders {
		req.Header.Set(k, v)
	}
	resp, err := e.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request to %s failed: %w", url, err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("endpoint %s returned %d: %s", url, resp.StatusCode, string(body))
	}
	return body, nil
}

// embedStandard implements the {model,input} -> {data:[{embedding}]} shape.
func (e *SimpleEmbedder) embedStandard(text string) ([]float32, error) {
	model := e.resolveModel()
	if model == "" {
		model = "auto"
	}
	body, err := e.post(e.spec.BaseURL+e.spec.Path, map[string]interface{}{
		"input": text,
		"model": model,
	}, true)
	if err != nil {
		return nil, err
	}
	var result struct {
		Data []struct {
			Embedding []float32 `json:"embedding"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}
	if len(result.Data) == 0 {
		return nil, fmt.Errorf("no embedding returned")
	}
	return result.Data[0].Embedding, nil
}

// embedGemini implements Google's embedContent shape.
func (e *SimpleEmbedder) embedGemini(text string) ([]float32, error) {
	model := e.model
	if model == "" || model == "auto" || !strings.Contains(model, "embedding") {
		model = "models/text-embedding-004"
	}
	url := fmt.Sprintf("%s/%s:embedContent?key=%s", e.spec.BaseURL, model, e.apiKey)
	body, err := e.post(url, map[string]interface{}{
		"content": map[string]interface{}{
			"parts": []map[string]string{{"text": text}},
		},
	}, false)
	if err != nil {
		return nil, err
	}
	var result struct {
		Embedding struct {
			Values []float32 `json:"values"`
		} `json:"embedding"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}
	return result.Embedding.Values, nil
}

// embedOllamaNative implements Ollama's pre-standard native route.
func (e *SimpleEmbedder) embedOllamaNative(text string) ([]float32, error) {
	base := strings.TrimSuffix(e.spec.BaseURL, "/v1")
	model := e.resolveModel()
	if model == "" || model == "auto" {
		return nil, fmt.Errorf("no model available on %s", base)
	}
	body, err := e.post(base+"/api/embeddings", map[string]interface{}{
		"model":  model,
		"prompt": text,
	}, false)
	if err != nil {
		return nil, err
	}
	var result struct {
		Embedding []float32 `json:"embedding"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}
	return result.Embedding, nil
}
