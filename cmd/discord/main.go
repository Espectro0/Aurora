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

	idCore := identity.New("aurora.json")
	id := idCore.Get()
	rules := id.MemoryUsageRules

	llmClient := llmnvidia.New(cfg.NvidiaApiKey, cfg.NvidiaModel, time.Duration(id.LLM.ChatTimeoutSeconds))
	reflectLLM := llmnvidia.New(cfg.NvidiaApiKey, cfg.ReflectionModel, time.Duration(id.LLM.ReflectionTimeoutSeconds))

	emb := embednvidia.New(cfg.NvidiaApiKey, cfg.EmbedderModel, cfg.EmbedderBaseURL, time.Duration(id.LLM.EmbedderTimeoutSeconds))
	memStore, err := chromem.NewStore("aurora", emb)
	if err != nil {
		log.Fatal(err)
	}
	defer memStore.Close()
	memStore.SetClusterThreshold(rules.ClusterThreshold)

	mem := memory.NewInMemory()
	threshold := rules.SemanticRelevanceThreshold
	propSystem := proposals.NewMemoryProcessor(memStore, "journal.md", threshold)
	reflector := reflection.New(reflectLLM, propSystem, mem, reflection.Config{
		Interval:   rules.ReflectionInterval,
		MaxHistory: rules.ReflectionHistory,
	})
	a := agent.NewAgent(llmClient, idCore, mem, memStore, reflector)
	bot := discord.NewBot(cfg.DiscordToken, a)

	log.Println("Aurora Iniciada...")
	if err := bot.Run(ctx); err != nil {
		log.Fatal(err)
	}

	<-ctx.Done()
	log.Println("Aurora detenida")
	a.Wait()

}
