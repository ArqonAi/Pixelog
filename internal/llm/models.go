package llm

// GetDefaultChatModel returns the latest/best LLM model for each provider
func GetDefaultChatModel(provider string) string {
	switch provider {
	case "openai":
		return "gpt-4.5-turbo" // Latest GPT (GPT-5 not released yet, 4.5 is latest)
	case "openrouter":
		return "deepseek/deepseek-r1" // Latest DeepSeek R1 (reasoning model)
	case "google", "gemini":
		return "gemini-2.0-flash-exp" // Latest Gemini 2.0 (2.5 not public yet)
	case "anthropic", "claude":
		return "claude-4.5-sonnet-20250514" // Latest Claude 4.5
	case "xai", "grok":
		return "grok-3" // Latest Grok 3
	default:
		return "deepseek/deepseek-r1" // Default to cheapest high-quality
	}
}

// GetModelCost returns approximate cost per million tokens (input/output average)
func GetModelCost(model string) float64 {
	costs := map[string]float64{
		// OpenAI
		"gpt-4.5-turbo":               0.50,  // $0.50/1M tokens
		"gpt-4o":                      0.75,  // $0.75/1M tokens
		
		// DeepSeek (OpenRouter)
		"deepseek/deepseek-r1":        0.14,  // $0.14/1M tokens (cheapest reasoning!)
		"deepseek/deepseek-chat":      0.14,  // $0.14/1M tokens
		
		// Gemini
		"gemini-2.0-flash-exp":        0.00,  // FREE during preview!
		"gemini-2.0-pro-exp":          0.00,  // FREE during preview!
		
		// Claude
		"claude-4.5-sonnet-20250514":  3.00,  // $3/1M tokens (most expensive, best quality)
		"claude-3.5-sonnet":           3.00,  // $3/1M tokens
		
		// Grok
		"grok-3":                      5.00,  // $5/1M tokens (via xAI)
		"grok-2":                      5.00,  // $5/1M tokens
		
		// Other popular models
		"anthropic/claude-3.5-sonnet": 3.00,  // via OpenRouter
		"google/gemini-2.0-flash-exp": 0.00,  // via OpenRouter (free)
		"meta-llama/llama-3.3-70b":    0.18,  // $0.18/1M tokens
	}
	
	if cost, ok := costs[model]; ok {
		return cost
	}
	return 1.00 // Default estimate
}

// RecommendedModels returns list of recommended models by use case
type ModelRecommendation struct {
	Model       string
	Provider    string
	Cost        float64
	Description string
}

func GetRecommendedModels() []ModelRecommendation {
	return []ModelRecommendation{
		{
			Model:       "gemini-2.0-flash-exp",
			Provider:    "google",
			Cost:        0.00,
			Description: "FREE + Fast + Latest Gemini 2.0 (best value)",
		},
		{
			Model:       "deepseek/deepseek-r1",
			Provider:    "openrouter",
			Cost:        0.14,
			Description: "Cheapest reasoning model ($0.14/1M tokens)",
		},
		{
			Model:       "gpt-4.5-turbo",
			Provider:    "openai",
			Cost:        0.50,
			Description: "Latest GPT with great quality",
		},
		{
			Model:       "claude-4.5-sonnet-20250514",
			Provider:    "anthropic",
			Cost:        3.00,
			Description: "Highest quality, best reasoning (expensive)",
		},
		{
			Model:       "grok-3",
			Provider:    "xai",
			Cost:        5.00,
			Description: "Latest xAI model with real-time data",
		},
	}
}
