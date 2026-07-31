package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	DiscordToken string
	NvidiaApiKey string
	NvidiaModel  string
}

func LoadConfig() (*Config, error) {
	godotenv.Load()

	return &Config{
		DiscordToken: mustGetenv("DISCORD_TOKEN"),
		NvidiaApiKey: mustGetenv("NVIDIA_API_KEY"),
		NvidiaModel:  mustGetenv("NVIDIA_MODEL"),
	}, nil
}

func mustGetenv(key string) string {
	value, ok := os.LookupEnv(key)
	if !ok {
		log.Default().Fatalf("Environment variable %s required...", key)
	}

	return value
}
