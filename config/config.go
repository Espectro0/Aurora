package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	DiscordToken    string
	NvidiaApiKey    string
	NvidiaModel     string
	EmbedderModel   string
	EmbedderBaseURL string
}

func LoadConfig() (*Config, error) {
	godotenv.Load()

	embedderBaseURL := os.Getenv("EMBEDDER_BASE_URL")
	if embedderBaseURL == "" {
		embedderBaseURL = "https://integrate.api.nvidia.com/v1"
	}

	return &Config{
		DiscordToken:    mustGetenv("DISCORD_TOKEN"),
		NvidiaApiKey:    mustGetenv("NVIDIA_API_KEY"),
		NvidiaModel:     mustGetenv("NVIDIA_MODEL"),
		EmbedderModel:   mustGetenv("NVIDIA_EMBEDDER_MODEL"),
		EmbedderBaseURL: embedderBaseURL,
	}, nil
}

func mustGetenv(key string) string {
	value, ok := os.LookupEnv(key)
	if !ok {
		log.Default().Fatalf("Environment variable %s required...", key)
	}

	return value
}
