package config

import (
	"log"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	DiscordToken  string
	SttBinPath    string
	SttModelPath  string
	SttLanguage   string
	FfmpegBinPath string

	LlamaBinPath    string
	LlamaChatModel  string
	LlamaEmbedModel string
	LlamaCodeModel  string
	LlamaPortChat   int
	LlamaPortEmbed  int
	LlamaPortCode   int
	LlamaContext    int
	LlamaIdleMin    int
}

func LoadConfig() (*Config, error) {
	godotenv.Load()
	return &Config{
		DiscordToken:  mustGetenv("DISCORD_TOKEN"),
		SttBinPath:    mustGetenv("STT_BIN_PATH"),
		SttModelPath:  mustGetenv("STT_MODEL_PATH"),
		SttLanguage:   getenv("STT_LANGUAGE", "es"),
		FfmpegBinPath: getenv("FFMPEG_BIN_PATH", "tools/ffmpeg/ffmpeg.exe"),

		LlamaBinPath:    getenv("LLAMA_BIN_PATH", "tools/llama/llama-server.exe"),
		LlamaChatModel:  getenv("LLAMA_CHAT_MODEL_PATH", ""),
		LlamaEmbedModel: getenv("LLAMA_EMBED_MODEL_PATH", ""),
		LlamaCodeModel:  getenv("LLAMA_CODE_MODEL_PATH", ""),
		LlamaPortChat:   getenvInt("LLAMA_PORT_CHAT", 8080),
		LlamaPortEmbed:  getenvInt("LLAMA_PORT_EMBED", 8081),
		LlamaPortCode:   getenvInt("LLAMA_PORT_CODE", 8082),
		LlamaContext:    getenvInt("LLAMA_CONTEXT", 4096),
		LlamaIdleMin:    getenvInt("LLAMA_IDLE_TIMEOUT_MINUTES", 10),
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

func getenvInt(key string, fallback int) int {
	if value, ok := os.LookupEnv(key); ok && value != "" {
		if n, err := strconv.Atoi(value); err == nil {
			return n
		}
	}
	return fallback
}
