package llm

// Top 10 Latest Models (all via OpenRouter)
const (
	// Top tier - Latest models
	ModelGPT5         = "openai/gpt-5"                    // Latest GPT-5
	ModelGemini25Pro  = "google/gemini-2.5-pro-latest"   // Latest Gemini 2.5 Pro
	ModelClaude45     = "anthropic/claude-4.5-sonnet"    // Latest Claude 4.5
	ModelDeepSeekR1   = "deepseek/deepseek-r1"           // Latest DeepSeek R1 (reasoning)
	ModelGrok3        = "x-ai/grok-3"                     // Latest Grok 3
	
	// High value models
	ModelGemini25Flash = "google/gemini-2.5-flash-latest" // Fast Gemini 2.5
	ModelLlama33      = "meta-llama/llama-3.3-70b-instruct" // Latest Llama
	ModelQwen25       = "qwen/qwen-2.5-72b-instruct"     // Latest Qwen 2.5
	ModelMistralLarge = "mistralai/mistral-large-latest" // Latest Mistral
	ModelGPT4o        = "openai/gpt-4o"                  // GPT-4o (fallback)
)

// GetDefaultModel returns the best default model (cheapest with great quality)
func GetDefaultModel() string {
	return ModelDeepSeekR1 // $0.14/1M tokens, excellent reasoning
}

// GetTop10Models returns the top 10 latest models
func GetTop10Models() []ModelInfo {
	return []ModelInfo{
		{Model: ModelDeepSeekR1, Name: "DeepSeek R1", Cost: 0.14, Speed: "Fast", Quality: "Excellent", Description: "Best value - reasoning model"},
		{Model: ModelGemini25Flash, Name: "Gemini 2.5 Flash", Cost: 0.00, Speed: "Very Fast", Quality: "Excellent", Description: "FREE! Latest Google"},
		{Model: ModelGemini25Pro, Name: "Gemini 2.5 Pro", Cost: 0.50, Speed: "Medium", Quality: "Best", Description: "Latest Gemini, best quality"},
		{Model: ModelGPT5, Name: "GPT-5", Cost: 2.50, Speed: "Medium", Quality: "Best", Description: "Latest OpenAI flagship"},
		{Model: ModelClaude45, Name: "Claude 4.5 Sonnet", Cost: 3.00, Speed: "Medium", Quality: "Best", Description: "Latest Anthropic, best reasoning"},
		{Model: ModelGrok3, Name: "Grok 3", Cost: 5.00, Speed: "Fast", Quality: "Excellent", Description: "Latest xAI with real-time data"},
		{Model: ModelLlama33, Name: "Llama 3.3 70B", Cost: 0.18, Speed: "Fast", Quality: "Excellent", Description: "Open source, great quality"},
		{Model: ModelQwen25, Name: "Qwen 2.5 72B", Cost: 0.18, Speed: "Fast", Quality: "Excellent", Description: "Alibaba's latest, multilingual"},
		{Model: ModelMistralLarge, Name: "Mistral Large", Cost: 2.00, Speed: "Fast", Quality: "Excellent", Description: "European flagship"},
		{Model: ModelGPT4o, Name: "GPT-4o", Cost: 0.75, Speed: "Fast", Quality: "Excellent", Description: "Multimodal OpenAI"},
	}
}

type ModelInfo struct {
	Model       string
	Name        string
	Cost        float64 // Cost per 1M tokens
	Speed       string
	Quality     string
	Description string
}

// GetModelCost returns cost per million tokens
func GetModelCost(model string) float64 {
	for _, m := range GetTop10Models() {
		if m.Model == model {
			return m.Cost
		}
	}
	return 1.00 // Default estimate
}
