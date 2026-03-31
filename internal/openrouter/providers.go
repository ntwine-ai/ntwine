package openrouter

import "strings"

type Provider struct {
	ID          string
	Name        string
	BaseURL     string
	AuthHeader  string
	AuthPrefix  string
	Prefixes    []string
	StripPrefix bool
	NoTools     bool // provider does not support tool calling
	NoStream    bool // provider cannot stream when tools are present
}

var AllProviders = []Provider{
	{
		ID:         "openrouter",
		Name:       "OpenRouter",
		BaseURL:    "https://openrouter.ai/api/v1/chat/completions",
		AuthHeader: "Authorization",
		AuthPrefix: "Bearer ",
	},
	{
		ID:          "openai",
		Name:        "OpenAI",
		BaseURL:     "https://api.openai.com/v1/chat/completions",
		AuthHeader:  "Authorization",
		AuthPrefix:  "Bearer ",
		Prefixes:    []string{"openai/", "gpt-"},
		StripPrefix: true,
	},
	{
		ID:          "google",
		Name:        "Google",
		BaseURL:     "https://generativelanguage.googleapis.com/v1beta/openai/chat/completions",
		AuthHeader:  "Authorization",
		AuthPrefix:  "Bearer ",
		Prefixes:    []string{"google/", "gemini-"},
		StripPrefix: true,
	},
	{
		ID:          "xai",
		Name:        "xAI",
		BaseURL:     "https://api.x.ai/v1/chat/completions",
		AuthHeader:  "Authorization",
		AuthPrefix:  "Bearer ",
		Prefixes:    []string{"x-ai/", "grok-"},
		StripPrefix: true,
	},
	{
		ID:          "deepseek",
		Name:        "DeepSeek",
		BaseURL:     "https://api.deepseek.com/chat/completions",
		AuthHeader:  "Authorization",
		AuthPrefix:  "Bearer ",
		Prefixes:    []string{"deepseek/"},
		StripPrefix: true,
	},
	{
		ID:          "mistral",
		Name:        "Mistral",
		BaseURL:     "https://api.mistral.ai/v1/chat/completions",
		AuthHeader:  "Authorization",
		AuthPrefix:  "Bearer ",
		Prefixes:    []string{"mistral/", "mistralai/", "mixtral-"},
		StripPrefix: true,
	},
	{
		ID:          "groq",
		Name:        "Groq",
		BaseURL:     "https://api.groq.com/openai/v1/chat/completions",
		AuthHeader:  "Authorization",
		AuthPrefix:  "Bearer ",
		Prefixes:    []string{"groq/"},
		StripPrefix: true,
	},
	{
		ID:          "together",
		Name:        "Together",
		BaseURL:     "https://api.together.xyz/v1/chat/completions",
		AuthHeader:  "Authorization",
		AuthPrefix:  "Bearer ",
		Prefixes:    []string{"together/"},
		StripPrefix: true,
	},
	{
		ID:          "minimax",
		Name:        "MiniMax",
		BaseURL:     "https://api.minimax.io/v1/chat/completions",
		AuthHeader:  "Authorization",
		AuthPrefix:  "Bearer ",
		Prefixes:    []string{"minimax/"},
		StripPrefix: true,
	},
	{
		ID:          "fireworks",
		Name:        "Fireworks",
		BaseURL:     "https://api.fireworks.ai/inference/v1/chat/completions",
		AuthHeader:  "Authorization",
		AuthPrefix:  "Bearer ",
		Prefixes:    []string{"fireworks/"},
		StripPrefix: true,
	},
	{
		ID:          "cerebras",
		Name:        "Cerebras",
		BaseURL:     "https://api.cerebras.ai/v1/chat/completions",
		AuthHeader:  "Authorization",
		AuthPrefix:  "Bearer ",
		Prefixes:    []string{"cerebras/"},
		StripPrefix: true,
	},
	{
		ID:          "sambanova",
		Name:        "SambaNova",
		BaseURL:     "https://api.sambanova.ai/v1/chat/completions",
		AuthHeader:  "Authorization",
		AuthPrefix:  "Bearer ",
		Prefixes:    []string{"sambanova/"},
		StripPrefix: true,
	},
	{
		ID:          "moonshot",
		Name:        "Moonshot",
		BaseURL:     "https://api.moonshot.cn/v1/chat/completions",
		AuthHeader:  "Authorization",
		AuthPrefix:  "Bearer ",
		Prefixes:    []string{"moonshot/", "moonshotai/", "kimi-"},
		StripPrefix: true,
	},
}

var providerIndex map[string]Provider

func init() {
	providerIndex = make(map[string]Provider, len(AllProviders))
	for _, p := range AllProviders {
		providerIndex[p.ID] = p
	}
}

func GetProvider(id string) Provider {
	if p, ok := providerIndex[id]; ok {
		return p
	}
	return providerIndex["openrouter"]
}

func DetectProvider(modelID string) string {
	lower := strings.ToLower(modelID)
	for _, p := range AllProviders {
		for _, prefix := range p.Prefixes {
			if strings.HasPrefix(lower, prefix) {
				return p.ID
			}
		}
	}
	return "openrouter"
}

func stripModelPrefix(modelID string) string {
	lower := strings.ToLower(modelID)
	for _, p := range AllProviders {
		if !p.StripPrefix {
			continue
		}
		for _, prefix := range p.Prefixes {
			if strings.HasPrefix(lower, prefix) {
				return modelID[len(prefix):]
			}
		}
	}
	return modelID
}
