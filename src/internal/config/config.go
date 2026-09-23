package config

import (
	"fmt"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

const (
	ProviderAnthropic = "anthropic"
	ProviderGemini    = "gemini"
	ProviderOllama    = "ollama"
	ProviderSarvam    = "sarvam"

	defaultMaxTokens     = 16000
	defaultOllamaBaseURL = "http://localhost:11434"
	defaultSarvamBaseURL = "https://api.sarvam.ai"
	// Sarvam's default reasoning depth can consume the whole output budget on
	// a long generation, leaving no tokens for the answer.
	defaultSarvamEffort = "low"
)

var defaultModels = map[string]string{
	ProviderAnthropic: "claude-opus-5",
	ProviderGemini:    "gemini-3.6-flash",
	ProviderOllama:    "qwen2.5:3b",
	// The v2 models (deepseekv4-flash, glm5.3, gemma4) need beta access;
	// sarvam-105b is served on v1 and supports tool calling.
	ProviderSarvam: "sarvam-105b",
}

type Config struct {
	Provider  string
	Model     string
	MaxTokens int64
	APIKey    string
	BaseURL   string
	// ReasoningEffort is Sarvam-specific; other providers ignore it.
	ReasoningEffort string
}

func Load() (*Config, error) {
	// Absent .env is fine — the vars may already be exported in the shell.
	_ = godotenv.Load()

	provider := os.Getenv("LLM_PROVIDER")
	if provider == "" {
		provider = ProviderAnthropic
	}

	var apiKey, baseURL, effort string
	switch provider {
	case ProviderAnthropic:
		apiKey = os.Getenv("ANTHROPIC_API_KEY")
	case ProviderGemini:
		apiKey = os.Getenv("GEMINI_API_KEY")
	case ProviderOllama:
		// Runs locally — no credentials.
		baseURL = os.Getenv("OLLAMA_BASE_URL")
		if baseURL == "" {
			baseURL = defaultOllamaBaseURL
		}
	case ProviderSarvam:
		apiKey = os.Getenv("SARVAM_API_KEY")
		baseURL = os.Getenv("SARVAM_BASE_URL")
		if baseURL == "" {
			baseURL = defaultSarvamBaseURL
		}
		effort = os.Getenv("SARVAM_REASONING_EFFORT")
		if effort == "" {
			effort = defaultSarvamEffort
		}
	default:
		return nil, fmt.Errorf("unknown LLM_PROVIDER %q (want %q, %q, %q or %q)",
			provider, ProviderAnthropic, ProviderGemini, ProviderOllama, ProviderSarvam)
	}
	if provider != ProviderOllama && apiKey == "" {
		return nil, fmt.Errorf("no API key for provider %q (set it in .env)", provider)
	}

	model := os.Getenv("LLM_MODEL")
	if model == "" {
		model = defaultModels[provider]
	}

	maxTokens := int64(defaultMaxTokens)
	if raw := os.Getenv("LLM_MAX_TOKENS"); raw != "" {
		parsed, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("LLM_MAX_TOKENS must be an integer, got %q", raw)
		}
		maxTokens = parsed
	}

	return &Config{
		Provider:        provider,
		Model:           model,
		MaxTokens:       maxTokens,
		APIKey:          apiKey,
		BaseURL:         baseURL,
		ReasoningEffort: effort,
	}, nil
}
