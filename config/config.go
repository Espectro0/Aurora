package config

import (
	"log"
	"os"

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

	QdrantURL        string
	QdrantAPIKey     string
	QdrantCollection string
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

		QdrantURL:        getenv("QDRANT_URL", "http://localhost:6333"),
		QdrantAPIKey:     getenv("QDRANT_API_KEY", ""),
		QdrantCollection: getenv("QDRANT_COLLECTION", "aurora_memories"),
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
