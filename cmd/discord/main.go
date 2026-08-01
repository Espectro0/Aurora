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
	"github.com/Espectro0/AuroraProject/internal/discord/commands"
	embedopenai "github.com/Espectro0/AuroraProject/internal/embedder/openai"
	"github.com/Espectro0/AuroraProject/internal/identity"
	"github.com/Espectro0/AuroraProject/internal/llm/openai"
	"github.com/Espectro0/AuroraProject/internal/localai"
	"github.com/Espectro0/AuroraProject/internal/memory"
	"github.com/Espectro0/AuroraProject/internal/memory/chromem"
	"github.com/Espectro0/AuroraProject/internal/proposals"
	"github.com/Espectro0/AuroraProject/internal/reflection"
	"github.com/Espectro0/AuroraProject/internal/transcription/whispercpp"
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

	manager := localai.New(localai.Options{
		BinPath:     cfg.LlamaBinPath,
		ChatModel:   cfg.LlamaChatModel,
		EmbedModel:  cfg.LlamaEmbedModel,
		CodeModel:   cfg.LlamaCodeModel,
		ChatPort:    cfg.LlamaPortChat,
		EmbedPort:   cfg.LlamaPortEmbed,
		CodePort:    cfg.LlamaPortCode,
		Context:     cfg.LlamaContext,
		IdleTimeout: time.Duration(cfg.LlamaIdleMin) * time.Minute,
	})
	defer manager.Close()

	llmClient := openai.New("aurora-chat", time.Duration(id.LLM.ChatTimeoutSeconds)*time.Second)
	llmClient.SetMaxTokens(2048)
	llmClient.SetBaseURLProvider(manager.EnsureChat)
	codeLLM := openai.New("aurora-code", time.Duration(id.LLM.ReflectionTimeoutSeconds)*time.Second)
	codeLLM.SetMaxTokens(8192)
	codeLLM.SetBaseURLProvider(manager.EnsureCode)

	emb := embedopenai.New("aurora-embed", time.Duration(id.LLM.EmbedderTimeoutSeconds)*time.Second)
	emb.SetBaseURLProvider(manager.EnsureEmbed)
	memStore, err := chromem.NewStore("data/aurora", emb)
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
	a := agent.NewAgent(llmClient, idCore, mem, memStore, reflector)
	transProvider := whispercpp.New(cfg.SttBinPath, cfg.SttModelPath, cfg.SttLanguage, cfg.FfmpegBinPath)
	cmds := commands.New(commands.Deps{
		Identity:    idCore,
		Memory:      memStore,
		Manager:     manager,
		Chat:        llmClient,
		JournalPath: "data/journal.md",
	})
	bot := discord.NewBot(cfg.DiscordToken, a, transProvider, time.Duration(id.LLM.TranscriptionTimeoutSeconds)*time.Second, cmds)

	log.Println("Aurora is running...")
	if err := bot.Run(ctx); err != nil {
		log.Fatal(err)
	}

	<-ctx.Done()
	log.Println("Aurora's shutting down...")
	a.Wait()

}
