package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/Espectro0/AuroraProject/config"
	"github.com/Espectro0/AuroraProject/internal/agent"
	"github.com/Espectro0/AuroraProject/internal/discord"
	embednvidia "github.com/Espectro0/AuroraProject/internal/embedder/nvidia"
	"github.com/Espectro0/AuroraProject/internal/identity"
	llmnvidia "github.com/Espectro0/AuroraProject/internal/llm/nvidia"
	"github.com/Espectro0/AuroraProject/internal/memory"
	"github.com/Espectro0/AuroraProject/internal/memory/chromem"
	"github.com/Espectro0/AuroraProject/internal/proposals"
	"github.com/Espectro0/AuroraProject/internal/reflection"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	cfg, err := config.LoadConfig()
	if err != nil {
		return
	}

	llmClient := llmnvidia.New(cfg.NvidiaApiKey, cfg.NvidiaModel, 60)
	reflectLLM := llmnvidia.New(cfg.NvidiaApiKey, cfg.ReflectionModel, 120)

	emb := embednvidia.New(cfg.NvidiaApiKey, cfg.EmbedderModel, cfg.EmbedderBaseURL, 30)
	memStore, err := chromem.NewStore("aurora", emb)
	if err != nil {
		log.Fatal(err)
	}
	defer memStore.Close()

	mem := memory.NewInMemory()
	idCore := identity.New("aurora.json")
	threshold := idCore.Get().MemoryUsageRules.SemanticRelevanceThreshold
	propSystem := proposals.NewMemoryProcessor(memStore, "journal.md", threshold)
	reflector := reflection.New(reflectLLM, propSystem, mem, reflection.Config{Interval: 2})
	a := agent.NewAgent(llmClient, idCore, mem, memStore, reflector)
	bot := discord.NewBot(cfg.DiscordToken, a)

	log.Println("Aurora Iniciada...")
	if err := bot.Run(context.Background()); err != nil {
		log.Fatal(err)
	}

	<-ctx.Done()
	log.Println("Aurora detenida")

}
