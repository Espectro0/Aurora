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

const maxHistory = 20

func (r *Reflector) Analyze(ctx context.Context, userID string) error {
	history := r.memory.History(userID)
	if len(history) == 0 {
		return nil
	}

	if len(history) > maxHistory {
		history = history[len(history)-maxHistory:]
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
		retryMsgs := append([]conversation.Message{}, msgs...)
		retryMsgs = append(retryMsgs, conversation.NewMessage(conversation.System, "Tu respuesta anterior no fue JSON valido. Responde SOLO con el JSON, sin texto adicional."))

		retryResponse, retryErr := r.llm.Chat(ctx, retryMsgs)
		if retryErr != nil {
			return fmt.Errorf("reflection: chat retry: %w", retryErr)
		}

		prop, retryErr = r.parseResponse(retryResponse)
		if retryErr != nil {
			return fmt.Errorf("reflection: parse: %w", retryErr)
		}
	}

	if err := r.proposals.Process(ctx, prop); err != nil {
		return fmt.Errorf("reflection: process: %w", err)
	}

	log.Printf("[reflection] Analized conversation of %s: %s", userID, prop.Summary)
	return nil
}

func (r *Reflector) parseResponse(raw string) (proposals.Proposal, error) {
	raw = stripCodeFences(raw)

	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")

	if start == -1 || end == -1 || end <= start {
		return proposals.Proposal{}, fmt.Errorf("no JSON object in response")
	}

	jsonBlock := raw[start : end+1]

	var prop proposals.Proposal
	if err := json.Unmarshal([]byte(jsonBlock), &prop); err != nil {
		return proposals.Proposal{}, fmt.Errorf("Invalid JSON: %w\nraw: %s", err, jsonBlock)
	}

	if prop.ReflectionID == "" {
		prop.ReflectionID = uuid.New().String()
	}
	prop.Timestamp = time.Now()

	return prop, nil
}

func stripCodeFences(raw string) string {
	raw = strings.TrimSpace(raw)
	if strings.HasPrefix(raw, "```") {
		if nl := strings.Index(raw, "\n"); nl != -1 {
			raw = raw[nl+1:]
		} else {
			raw = ""
		}
		if idx := strings.LastIndex(raw, "```"); idx != -1 {
			raw = raw[:idx]
		}
	}
	return strings.TrimSpace(raw)
}
