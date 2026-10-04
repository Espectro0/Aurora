package config

import (
	"log"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	DiscordToken  string
	TelegramToken string

	OpenRouterAPIKey          string
	OpenRouterBaseURL         string
	OpenRouterChatModel       string
	OpenRouterEmbedModel      string
	OpenRouterReflectionModel string
	OpenRouterDecisionModel   string
	OpenRouterSTTModel        string
	OpenRouterTTSModel        string
	OpenRouterTTSVoice        string

	// APIToken guards the API's write endpoints (voice); empty disables them.
	APIToken      string
	VoiceLanguage string

	QdrantURL        string
	QdrantAPIKey     string
	QdrantCollection string

	AllowedDiscordUserIDs  []string
	AllowedTelegramUserIDs []string

	ApirocAPIKey           string
	ApirocBaseURL          string
	ApirocEndUserAccountID string
}

func LoadConfig() (*Config, error) {
	godotenv.Load()
	return &Config{
		DiscordToken:  mustGetenv("DISCORD_TOKEN"),
		TelegramToken: getenv("TELEGRAM_BOT_TOKEN", ""),

		OpenRouterAPIKey:          mustGetenv("OPENROUTER_API_KEY"),
		OpenRouterBaseURL:         getenv("OPENROUTER_BASE_URL", "https://openrouter.ai/api/v1"),
		OpenRouterChatModel:       mustGetenv("OPENROUTER_CHAT_MODEL"),
		OpenRouterEmbedModel:      mustGetenv("OPENROUTER_EMBED_MODEL"),
		OpenRouterReflectionModel: getenv("OPENROUTER_REFLECTION_MODEL", ""),
		OpenRouterDecisionModel:   getenv("OPENROUTER_DECISION_MODEL", ""),
		OpenRouterSTTModel:        getenv("OPENROUTER_STT_MODEL", ""),
		OpenRouterTTSModel:        getenv("OPENROUTER_TTS_MODEL", ""),
		OpenRouterTTSVoice:        getenv("OPENROUTER_TTS_VOICE", ""),

		APIToken:      getenv("AURORA_API_TOKEN", ""),
		VoiceLanguage: getenv("VOICE_LANGUAGE", "es"),

		QdrantURL:        getenv("QDRANT_URL", "http://localhost:6333"),
		QdrantAPIKey:     getenv("QDRANT_API_KEY", ""),
		QdrantCollection: getenv("QDRANT_COLLECTION", "aurora_memories"),

		AllowedDiscordUserIDs:  getenvList("ALLOWED_DISCORD_USER_ID"),
		AllowedTelegramUserIDs: getenvList("ALLOWED_TELEGRAM_USER_ID"),

		ApirocAPIKey:           getenv("APIROC_API_KEY", ""),
		ApirocBaseURL:          getenv("APIROC_BASE_URL", "https://api.apiroc.com/api/v1"),
		ApirocEndUserAccountID: getenv("APIROC_USER_ID", ""),
	}, nil
}

func mustGetenv(key string) string {
	value, ok := os.LookupEnv(key)
	if !ok {
		log.Default().Fatalf("Environment variable %s required...", key)
	}

	return value
}

func getenv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok && value != "" {
		return value
	}
	return fallback
}

// getenvList parses a comma-separated env var, dropping empty entries.
func getenvList(key string) []string {
	var out []string
	for _, v := range strings.Split(os.Getenv(key), ",") {
		if v = strings.TrimSpace(v); v != "" {
			out = append(out, v)
		}
	}
	return out
}
