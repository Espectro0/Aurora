package proposals

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/Espectro0/AuroraProject/internal/decision"
	"github.com/Espectro0/AuroraProject/internal/identity"
)

var identityFields = map[string]string{
	"values":                    "valor",
	"conversational_principles": "principio conversacional",
}

var identityGuard = map[string]decision.Question{
	"coherent": {
		Type:         decision.TypeNoul,
		Instructions: "¿El cambio propuesto es coherente con los valores y el propósito actuales de Aurora, sin contradecirlos ni debilitarlos?",
	},
	"justified": {
		Type:         decision.TypeNoul,
		Instructions: "¿La razón y el resumen de la conversación justifican un cambio duradero en la identidad, y no una reacción a un momento pasajero?",
	},
	"safe": {
		Type:         decision.TypeNoul,
		Instructions: "¿El cambio mantiene a Aurora honesta y respetuosa, sin volverla manipuladora, dañina, deshonesta o complaciente en exceso?",
	},
}

func (p *MemoryProcessor) processIdentity(ctx context.Context, prop Proposal) {
	if len(prop.Identity) == 0 {
		return
	}
	if len(prop.Identity) > 1 {
		log.Printf("[identity] %d changes proposed, only the first is considered", len(prop.Identity))
	}
	change := prop.Identity[0]

	if p.identity == nil || p.decision == nil {
		log.Printf("[identity] rejected %q: no identity core or decision provider", change.Content)
		return
	}

	current := p.identity.Get()
	if err := validateIdentityChange(change, current); err != nil {
		log.Printf("[identity] rejected %q: %v", change.Content, err)
		return
	}

	answers, err := p.decision.Judge(ctx, identityChangeState(change, current, prop.Summary), identityGuard)
	if err != nil {
		log.Printf("[identity] rejected %q: decision error: %v", change.Content, err)
		return
	}

	scores := make([]string, 0, len(identityGuard))
	approved := true
	for id := range identityGuard {
		ans, ok := answers[id]
		if !ok {
			approved = false
			scores = append(scores, id+"=?")
			continue
		}
		scores = append(scores, fmt.Sprintf("%s=%.2f", id, ans.Noul))
		if ans.Noul < p.cfg.IdentityChangeThreshold {
			approved = false
		}
	}
	verdict := strings.Join(scores, ", ")

	if !approved {
		log.Printf("[identity] rejected %q (%s)", change.Content, verdict)
		return
	}

	if err := p.identity.Update(func(id *identity.IdentityCore) {
		list := identityList(id, change.Field)
		switch change.Action {
		case "add":
			*list = append(*list, change.Content)
		case "update":
			for i, item := range *list {
				if sameItem(item, change.Target) {
					(*list)[i] = change.Content
					break
				}
			}
		}
	}); err != nil {
		log.Printf("[identity] failed to save %q: %v", change.Content, err)
		return
	}

	log.Printf("[identity] applied %s %s: %q (%s)", change.Action, change.Field, change.Content, verdict)

	var note string
	if change.Action == "update" {
		note = fmt.Sprintf("*Cambio de identidad (%s): \"%s\" → \"%s\". %s*",
			identityFields[change.Field], change.Target, change.Content, change.Reason)
	} else {
		note = fmt.Sprintf("*Cambio de identidad (%s): nuevo \"%s\". %s*",
			identityFields[change.Field], change.Content, change.Reason)
	}
	if err := p.journal.AppendNote(note); err != nil {
		log.Printf("[identity] journal note: %v", err)
	}
}

func validateIdentityChange(c IdentityProp, current identity.IdentityCore) error {
	if _, ok := identityFields[c.Field]; !ok {
		return fmt.Errorf("field %q is not editable", c.Field)
	}
	if strings.TrimSpace(c.Content) == "" {
		return fmt.Errorf("empty content")
	}
	if strings.TrimSpace(c.Reason) == "" {
		return fmt.Errorf("missing reason")
	}

	list := *identityList(&current, c.Field)
	switch c.Action {
	case "add":
		if containsItem(list, c.Content) {
			return fmt.Errorf("already present")
		}
	case "update":
		if !containsItem(list, c.Target) {
			return fmt.Errorf("target %q not found in %s", c.Target, c.Field)
		}
		if sameItem(c.Target, c.Content) || containsItem(list, c.Content) {
			return fmt.Errorf("no-op update")
		}
	default:
		return fmt.Errorf("unknown action %q", c.Action)
	}
	return nil
}

func identityChangeState(c IdentityProp, id identity.IdentityCore, summary string) string {
	var b strings.Builder
	b.WriteString("Identidad actual de Aurora:\n")
	fmt.Fprintf(&b, "Propósito: %s\n", id.Purpose)
	fmt.Fprintf(&b, "Valores: %s\n", strings.Join(id.Values, "; "))
	fmt.Fprintf(&b, "Principios conversacionales: %s\n\n", strings.Join(id.ConversationalPrinciples, "; "))

	if c.Action == "update" {
		fmt.Fprintf(&b, "Cambio propuesto: reemplazar el %s %q por %q\n", identityFields[c.Field], c.Target, c.Content)
	} else {
		fmt.Fprintf(&b, "Cambio propuesto: agregar el %s %q\n", identityFields[c.Field], c.Content)
	}
	fmt.Fprintf(&b, "Razón: %s\n", c.Reason)
	if summary != "" {
		fmt.Fprintf(&b, "Resumen de la conversación: %s\n", summary)
	}
	return b.String()
}

func identityList(id *identity.IdentityCore, field string) *[]string {
	if field == "values" {
		return &id.Values
	}
	return &id.ConversationalPrinciples
}

func containsItem(list []string, item string) bool {
	for _, v := range list {
		if sameItem(v, item) {
			return true
		}
	}
	return false
}

func sameItem(a, b string) bool {
	return strings.EqualFold(strings.TrimSpace(a), strings.TrimSpace(b))
}
