package reflection

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/Espectro0/AuroraProject/internal/conversation"
	"github.com/Espectro0/AuroraProject/internal/decision"
	"github.com/Espectro0/AuroraProject/internal/identity"
	"github.com/Espectro0/AuroraProject/internal/llm"
	"github.com/Espectro0/AuroraProject/internal/memory"
	"github.com/Espectro0/AuroraProject/internal/proposals"
	"github.com/google/uuid"
)

type Reflector struct {
	llm       llm.Provider
	decision  decision.Provider
	proposals proposals.System
	memory    memory.Store
	identity  *identity.Core
	config    Config
}

type Config struct {
	Interval              int
	MaxHistory            int
	GateThreshold         float64
	WorthKeepingThreshold float64
}

const maxDecisionConcurrency = 4

func New(llm llm.Provider, dec decision.Provider, p proposals.System, mem memory.Store, id *identity.Core, cfg Config) *Reflector {
	if cfg.Interval == 0 {
		cfg.Interval = 5
	}
	if cfg.MaxHistory == 0 {
		cfg.MaxHistory = 20
	}
	if cfg.GateThreshold == 0 {
		cfg.GateThreshold = 0.4
	}
	if cfg.WorthKeepingThreshold == 0 {
		cfg.WorthKeepingThreshold = 0.5
	}
	return &Reflector{
		llm:       llm,
		decision:  dec,
		proposals: p,
		memory:    mem,
		identity:  id,
		config:    cfg,
	}
}

func (r *Reflector) Interval() int {
	return r.config.Interval
}

func (r *Reflector) Analyze(ctx context.Context, userID string) error {
	return r.AnalyzeAs(ctx, userID, userID)
}

func (r *Reflector) AnalyzeAs(ctx context.Context, userID, owner string) error {
	history := r.memory.History(userID)
	if len(history) == 0 {
		return nil
	}

	history = filterSkillTurns(history)
	if len(history) == 0 {
		return nil
	}

	if len(history) > r.config.MaxHistory {
		history = history[len(history)-r.config.MaxHistory:]
	}

	name := "el usuario"
	if sp, ok := conversation.SpeakerFrom(ctx); ok && sp.Name != "" {
		name = sp.Name
	}

	if !r.worthReflecting(ctx, history, name) {
		return nil
	}

	msgs := []conversation.Message{
		conversation.NewMessage(conversation.System, systemPrompt),
		conversation.NewMessage(conversation.System, r.contextMessage(name)),
	}
	msgs = append(msgs, history...)

	prop, err := r.propose(ctx, msgs)
	if err != nil {
		return err
	}

	prop.Owner = owner
	r.enrich(ctx, &prop)

	if err := r.proposals.Process(ctx, prop); err != nil {
		return fmt.Errorf("reflection: process: %w", err)
	}

	log.Printf("[reflection] Analized conversation of %s: %s", userID, prop.Summary)
	return nil
}

func (r *Reflector) propose(ctx context.Context, msgs []conversation.Message) (proposals.Proposal, error) {
	response, err := r.llm.Chat(ctx, msgs)
	if err != nil {
		return proposals.Proposal{}, fmt.Errorf("reflection: chat: %w", err)
	}

	prop, err := r.parseResponse(response)
	if err != nil {
		retryMsgs := append([]conversation.Message{}, msgs...)
		retryMsgs = append(retryMsgs, conversation.NewMessage(conversation.System, "Tu respuesta anterior no fue JSON valido. Responde SOLO con el JSON, sin texto adicional."))

		retryResponse, retryErr := r.llm.Chat(ctx, retryMsgs)
		if retryErr != nil {
			return proposals.Proposal{}, fmt.Errorf("reflection: chat retry: %w", retryErr)
		}

		prop, retryErr = r.parseResponse(retryResponse)
		if retryErr != nil {
			return proposals.Proposal{}, fmt.Errorf("reflection: parse: %w", retryErr)
		}
	}
	return prop, nil
}

var nodeTypes = map[string]string{
	string(memory.NodePerson):  "Una persona mencionada",
	string(memory.NodeConcept): "Un tema, tecnología, lugar u objeto",
	string(memory.NodeEvent):   "Un evento o suceso puntual",
	string(memory.NodeProject): "Un proyecto en curso",
}

const edgeNone = "none"

var edgeTypes = map[string]string{
	string(memory.EdgeParticipates): "una persona participa en un evento",
	string(memory.EdgeMentions):     "una entidad menciona a otra",
	string(memory.EdgeRelates):      "entidades relacionadas temáticamente",
	string(memory.EdgePrefers):      "una persona prefiere a otra entidad",
	string(memory.EdgeLeadsTo):      "una entidad conduce a otra",
	string(memory.EdgeSentiment):    "una entidad tiene carga emocional hacia otra",
	edgeNone:                        "no hay una relación clara, directa y duradera entre ellas",
}

const (
	scopePersonal = "personal"
	scopeGeneral  = "general"
)

var nodeScopes = map[string]string{
	scopePersonal: "trata de una persona concreta: su vida, gustos, proyectos, planes, salud, familia, amigos o algo que solo le concierne a ella",
	scopeGeneral:  "conocimiento general válido para cualquiera (qué es algo, datos de un lugar, cómo funciona una tecnología), sin datos privados de nadie",
}

var importanceLevels = []string{
	"trivial: detalle pasajero que no cambia nada si se olvida",
	"útil: dato que puede ayudar en alguna conversación futura",
	"importante: algo relevante en la vida, trabajo o gustos de la persona, o conocimiento general sólido y reutilizable",
	"central: parte esencial de quién es la persona o de su relación con Aurora, o conocimiento fundamental sobre un tema que le interesa a Aurora",
}

func (r *Reflector) worthReflecting(ctx context.Context, history []conversation.Message, name string) bool {
	if r.decision == nil {
		return true
	}

	answers, err := r.decision.Judge(ctx, transcript(history, name), map[string]decision.Question{
		"substantive": {
			Type:         decision.TypeNoul,
			Instructions: "¿Esta conversación contiene información duradera que valga la pena recordar (hechos sobre el usuario, sus proyectos, gustos, personas o eventos, o conocimiento general aprendido: qué es algo, cómo funciona una tecnología, datos de un lugar o tema), y no solo saludos, charla trivial o consultas pasajeras?",
		},
	})
	if err != nil {
		log.Printf("[reflection] gate decision error, reflecting anyway: %v", err)
		return true
	}
	ans, ok := answers["substantive"]
	if !ok {
		return true
	}
	if ans.Noul < r.config.GateThreshold {
		log.Printf("[reflection] skipped: conversation not substantive (%.2f)", ans.Noul)
		return false
	}
	return true
}

func transcript(history []conversation.Message, name string) string {
	var b strings.Builder
	for _, m := range history {
		switch m.Role {
		case conversation.User:
			fmt.Fprintf(&b, "%s: %s\n", name, m.Content)
		case conversation.Assistant:
			if m.Content != "" {
				fmt.Fprintf(&b, "Aurora: %s\n", m.Content)
			}
		}
	}
	return b.String()
}

func (r *Reflector) contextMessage(name string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Esta conversación fue con: %s\n\n", name)
	if r.identity != nil {
		id := r.identity.Get()
		b.WriteString("Tu identidad actual:\n")
		fmt.Fprintf(&b, "Propósito: %s\n", id.Purpose)
		b.WriteString("Valores:\n")
		for _, v := range id.Values {
			fmt.Fprintf(&b, "- %s\n", v)
		}
		b.WriteString("Principios conversacionales:\n")
		for _, p := range id.ConversationalPrinciples {
			fmt.Fprintf(&b, "- %s\n", p)
		}
		b.WriteString("\n")
	}
	fmt.Fprintf(&b, "Moods permitidos para \"journal.mood\": %s", strings.Join(sortedKeys(proposals.Moods), ", "))
	return b.String()
}

func (r *Reflector) enrich(ctx context.Context, prop *proposals.Proposal) {
	var wg sync.WaitGroup
	wg.Go(func() { r.resolveMood(ctx, prop.Journal) })
	if prop.Memory != nil {
		r.classifyNodes(ctx, prop.Memory)
		r.classifyEdges(ctx, prop.Memory)
	}
	wg.Wait()
}

func (r *Reflector) resolveMood(ctx context.Context, j *proposals.JournalProp) {
	if j == nil {
		return
	}
	mood := strings.ToLower(strings.TrimSpace(j.Mood))
	if _, ok := proposals.Moods[mood]; ok {
		j.Mood = mood
		return
	}

	j.Mood = proposals.DefaultMood
	if r.decision == nil || j.Content == "" {
		return
	}
	answers, err := r.decision.Judge(ctx, j.Content, map[string]decision.Question{
		"mood": {
			Type:         decision.TypeChoice,
			Instructions: "¿Qué estado de ánimo describe mejor el tono de esta entrada de diario?",
			Criteria:     proposals.Moods,
		},
	})
	if err != nil {
		log.Printf("[reflection] mood decision error, using %s: %v", j.Mood, err)
		return
	}
	if ans, ok := answers["mood"]; ok && validChoice(ans.Choice, proposals.Moods) {
		j.Mood = ans.Choice
	}
}

func (r *Reflector) classifyNodes(ctx context.Context, m *proposals.MemoryProp) {
	var speakerName string
	if sp, ok := conversation.SpeakerFrom(ctx); ok {
		speakerName = strings.ToLower(strings.TrimSpace(sp.Name))
	}

	keep := make([]bool, len(m.Nodes))
	forEachLimit(len(m.Nodes), func(i int) {
		n := &m.Nodes[i]
		n.Type = string(memory.NodeConcept)
		keep[i] = true
		if r.decision == nil {
			return
		}

		answers, err := r.decision.Judge(ctx, n.Label+": "+n.Content, map[string]decision.Question{
			"worth_keeping": {
				Type:         decision.TypeNoul,
				Instructions: "¿Vale la pena conservar esto como recuerdo a largo plazo (no es trivial ni pasajero)?",
			},
			"type": {
				Type:         decision.TypeChoice,
				Instructions: "¿Qué tipo de entidad es esta?",
				Criteria:     nodeTypes,
			},
			"importance": {
				Type:         decision.TypeScore,
				Instructions: "¿Qué tan importante es este recuerdo a largo plazo, ya sea para conocer a la persona o como conocimiento general que Aurora pueda reutilizar?",
				Levels:       importanceLevels,
			},
			"scope": {
				Type:         decision.TypeChoice,
				Instructions: "¿Este recuerdo es información personal de alguien o conocimiento general que se puede compartir con cualquier persona?",
				Criteria:     nodeScopes,
			},
		})
		if err != nil {
			log.Printf("[reflection] decision error on node %q, keeping it as %s: %v", n.Label, n.Type, err)
			return
		}

		if wk, ok := answers["worth_keeping"]; !ok {
			log.Printf("[reflection] missing worth_keeping answer for node %q, keeping it", n.Label)
		} else if wk.Noul < r.config.WorthKeepingThreshold {
			log.Printf("[reflection] dropping trivial node %q (%.2f)", n.Label, wk.Noul)
			keep[i] = false
			return
		}

		if t, ok := answers["type"]; ok && validChoice(t.Choice, nodeTypes) {
			n.Type = t.Choice
		} else {
			log.Printf("[reflection] missing/invalid type for node %q, using %s", n.Label, n.Type)
		}

		if imp, ok := answers["importance"]; ok {
			v := normalizeLevel(imp.Score, len(importanceLevels))
			n.Importance = &v
		}

		if sc, ok := answers["scope"]; ok && sc.Choice == scopeGeneral {
			n.Shared = true
		}
		if n.Type == string(memory.NodeProject) ||
			(speakerName != "" && strings.Contains(strings.ToLower(n.Label+" "+n.Content), speakerName)) {
			n.Shared = false
		}
	})

	kept := m.Nodes[:0]
	for i, n := range m.Nodes {
		if keep[i] {
			kept = append(kept, n)
		}
	}
	m.Nodes = kept
}

func (r *Reflector) classifyEdges(ctx context.Context, m *proposals.MemoryProp) {
	labels := make(map[string]bool, len(m.Nodes))
	for _, n := range m.Nodes {
		labels[strings.ToLower(strings.TrimSpace(n.Label))] = true
	}

	var edges []proposals.EdgeProp
	for _, e := range m.Edges {
		if labels[strings.ToLower(strings.TrimSpace(e.Source))] && labels[strings.ToLower(strings.TrimSpace(e.Target))] {
			edges = append(edges, e)
		}
	}

	keep := make([]bool, len(edges))
	forEachLimit(len(edges), func(i int) {
		e := &edges[i]
		e.Type = string(memory.EdgeRelates)
		keep[i] = true
		if r.decision == nil {
			return
		}

		answers, err := r.decision.Judge(ctx, e.Relation, map[string]decision.Question{
			"type": {
				Type:         decision.TypeChoice,
				Instructions: fmt.Sprintf("¿Qué tipo de relación hay entre %q y %q?", e.Source, e.Target),
				Criteria:     edgeTypes,
			},
		})
		if err != nil {
			log.Printf("[reflection] decision error on edge %s->%s, using %s: %v", e.Source, e.Target, e.Type, err)
			return
		}
		t, ok := answers["type"]
		switch {
		case !ok || !validChoice(t.Choice, edgeTypes):
			log.Printf("[reflection] missing/invalid type for edge %s->%s, using %s", e.Source, e.Target, e.Type)
		case t.Choice == edgeNone:
			log.Printf("[reflection] dropping weak edge %s->%s (%q)", e.Source, e.Target, e.Relation)
			keep[i] = false
		default:
			e.Type = t.Choice
		}
	})

	kept := edges[:0]
	for i, e := range edges {
		if keep[i] {
			kept = append(kept, e)
		}
	}
	m.Edges = kept
}

func forEachLimit(n int, fn func(i int)) {
	sem := make(chan struct{}, maxDecisionConcurrency)
	var wg sync.WaitGroup
	for i := range n {
		sem <- struct{}{}
		wg.Go(func() {
			defer func() { <-sem }()
			fn(i)
		})
	}
	wg.Wait()
}

func normalizeLevel(score float64, levels int) float64 {
	if levels < 2 {
		return 0
	}
	return math.Max(0, math.Min(1, score/float64(levels-1)))
}

func validChoice(choice string, criteria map[string]string) bool {
	_, ok := criteria[choice]
	return ok
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func filterSkillTurns(history []conversation.Message) []conversation.Message {
	filtered := make([]conversation.Message, 0, len(history))
	for _, m := range history {
		if m.SkillUsed {
			continue
		}
		filtered = append(filtered, m)
	}
	return filtered
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

	prop.ReflectionID = uuid.New().String()
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
