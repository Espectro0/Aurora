package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Espectro0/AuroraProject/config"
	"github.com/Espectro0/AuroraProject/internal/agent"
	"github.com/Espectro0/AuroraProject/internal/discord"
	embedopenai "github.com/Espectro0/AuroraProject/internal/embedder/openai"
	"github.com/Espectro0/AuroraProject/internal/httpclient"
	"github.com/Espectro0/AuroraProject/internal/identity"
	"github.com/Espectro0/AuroraProject/internal/llm/openai"
	"github.com/Espectro0/AuroraProject/internal/memory"
	"github.com/Espectro0/AuroraProject/internal/memory/qdrant"
	"github.com/Espectro0/AuroraProject/internal/proposals"
	"github.com/Espectro0/AuroraProject/internal/reflection"
	"github.com/Espectro0/AuroraProject/internal/skills"
	"github.com/Espectro0/AuroraProject/internal/skills/clock"
	"github.com/Espectro0/AuroraProject/internal/skills/cornare"
	"github.com/Espectro0/AuroraProject/internal/skills/siata"
	"github.com/Espectro0/AuroraProject/internal/telegram"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	cfg, err := config.LoadConfig()
	if err != nil {
		return
	}

	idCore := identity.New("data/aurora.json")
	id := idCore.Get()
	rules := id.MemoryUsageRules

	reflectionModel := cfg.OpenRouterReflectionModel
	if reflectionModel == "" {
		reflectionModel = cfg.OpenRouterChatModel
	}

	llmClient := openai.New(cfg.OpenRouterChatModel, time.Duration(id.LLM.ChatTimeoutSeconds)*time.Second)
	llmClient.SetMaxTokens(2048)
	llmClient.SetBaseURL(cfg.OpenRouterBaseURL)
	llmClient.SetAPIKey(cfg.OpenRouterAPIKey)

	codeLLM := openai.New(reflectionModel, time.Duration(id.LLM.ReflectionTimeoutSeconds)*time.Second)
	codeLLM.SetMaxTokens(8192)
	codeLLM.SetBaseURL(cfg.OpenRouterBaseURL)
	codeLLM.SetAPIKey(cfg.OpenRouterAPIKey)

	emb := embedopenai.New(cfg.OpenRouterEmbedModel, time.Duration(id.LLM.EmbedderTimeoutSeconds)*time.Second)
	emb.SetBaseURL(cfg.OpenRouterBaseURL)
	emb.SetAPIKey(cfg.OpenRouterAPIKey)
	memStore, err := qdrant.NewStore(qdrant.Config{
		BaseURL:    cfg.QdrantURL,
		APIKey:     cfg.QdrantAPIKey,
		Collection: cfg.QdrantCollection,
		EdgesPath:  "data/aurora.edges.json",
	}, emb)
	if err != nil {
		log.Fatal(err)
	}
	defer memStore.Close()
	memStore.SetClusterThreshold(rules.ClusterThreshold)

	mem := memory.NewInMemory()
	threshold := rules.SemanticRelevanceThreshold
	propSystem := proposals.NewMemoryProcessor(memStore, "data/journal.md", threshold)
	reflector := reflection.New(codeLLM, propSystem, mem, reflection.Config{
		Interval:   rules.ReflectionInterval,
		MaxHistory: rules.ReflectionHistory,
	})

	siataClient := siata.NewClient(httpclient.New(30 * time.Second))
	cornareClient := cornare.NewClient(httpclient.New(30 * time.Second))

	skillRegistry := skills.NewRegistry()
	skillRegistry.Register(clock.New())
	skillRegistry.Register(siata.New(siataClient))
	skillRegistry.Register(cornare.New(cornareClient))

	a := agent.NewAgent(llmClient, idCore, mem, memStore, reflector, skillRegistry)

	discordBot := discord.NewBot(cfg.DiscordToken, a)
	if err := discordBot.Run(ctx); err != nil {
		log.Fatal(err)
	}
	log.Println("Aurora is running on Discord...")

	if cfg.TelegramToken != "" {
		telegramBot := telegram.NewBot(cfg.TelegramToken, a)
		if err := telegramBot.Run(ctx); err != nil {
			log.Fatal(err)
		}
		log.Println("Aurora is running on Telegram...")
	}

	<-ctx.Done()
	log.Println("Aurora's shutting down...")
	a.Wait()
}
