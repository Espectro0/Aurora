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
	"github.com/Espectro0/AuroraProject/internal/identity"
	"github.com/Espectro0/AuroraProject/internal/llm/nvidia"
	"github.com/Espectro0/AuroraProject/internal/memory"
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

	llmClient := nvidia.New(cfg.NvidiaApiKey, cfg.NvidiaModel, 60)
	reflectLLM := nvidia.New(cfg.NvidiaApiKey, cfg.NvidiaModel, 120)

	mem := memory.NewInMemory()
	idCore := identity.New("aurora.json")
	propSystem := proposals.NewSimpleProcessor("journal.md")
	reflector := reflection.New(reflectLLM, propSystem, mem, reflection.Config{Interval: 2})
	a := agent.NewAgent(llmClient, idCore, mem, reflector)
	bot := discord.NewBot(cfg.DiscordToken, a)

	log.Println("Aurora Iniciada...")
	if err := bot.Run(context.Background()); err != nil {
		log.Fatal(err)
	}

	<-ctx.Done()
	log.Println("Aurora detenida")

}
