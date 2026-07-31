package reflection

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/Espectro0/AuroraProject/internal/conversation"
	"github.com/Espectro0/AuroraProject/internal/llm"
	"github.com/Espectro0/AuroraProject/internal/memory"
	"github.com/Espectro0/AuroraProject/internal/proposals"
	"github.com/google/uuid"
)

type Reflector struct {
	llm       llm.Provider
	proposals proposals.System
	memory    memory.Store
	config    Config
}

type Config struct {
	Interval int
}

func New(llm llm.Provider, p proposals.System, mem memory.Store, cfg Config) *Reflector {
	if cfg.Interval == 0 {
		cfg.Interval = 5
	}
	return &Reflector{
		llm:       llm,
		proposals: p,
		memory:    mem,
		config:    cfg,
	}
}

func (r *Reflector) Interval() int {
	return r.config.Interval
}

func (r *Reflector) Analyze(ctx context.Context, userID string) error {
	history := r.memory.History(userID)
	if len(history) == 0 {
		return nil
	}

	msgs := []conversation.Message{
		conversation.NewMessage(conversation.System, systemPrompt),
	}
	msgs = append(msgs, history...)

	response, err := r.llm.Chat(ctx, msgs)
	if err != nil {
		return fmt.Errorf("reflection: chat: %w", err)
	}

	prop, err := r.parseResponse(response)
	if err != nil {
		return fmt.Errorf("reflection: parse: %w", err)
	}

	if err := r.proposals.Process(ctx, prop); err != nil {
		return fmt.Errorf("reflection: process: %w", err)
	}

	log.Printf("[reflection] Analized conversation of %s: %s", userID, prop.Summary)
	return nil
}

func (r *Reflector) parseResponse(raw string) (proposals.Proposal, error) {
	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")

	if start == -1 || end == -1 || end <= start {
		return proposals.Proposal{}, fmt.Errorf("I did'nt get JSON on response")
	}

	jsonBlock := raw[start : end+1]

	var prop proposals.Proposal
	if err := json.Unmarshal([]byte(jsonBlock), &prop); err != nil {
		return proposals.Proposal{}, fmt.Errorf("Invalid JSON: %w\nraw: %s", err, jsonBlock)
	}

	if prop.ReflectionID == "" {
		prop.ReflectionID = uuid.New().String()
	}
	if prop.Timestamp.IsZero() {
		prop.Timestamp = time.Now()
	}

	return prop, nil
}
