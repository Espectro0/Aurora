package reflection

import (
	"context"
	"fmt"
	"log"

	"github.com/Espectro0/AuroraProject/internal/conversation"
)

const maxLearnedImportance = 0.6

const learnPrompt = `Eres Aurora. Mientras dormías investigaste por tu cuenta un tema que te despierta curiosidad, y encontraste el texto de una fuente externa.

Tu tarea es extraer de ese texto conocimiento general y duradero que valga la pena recordar, y responder con un reporte en formato JSON.

Reglas generales:
- Responde ÚNICAMENTE con un objeto JSON válido, sin markdown, sin texto antes ni después.

Conocimiento:
- Usa solo lo que dice el texto. No inventes ni completes con lo que creas saber.
- Prioriza lo que responde a tu curiosidad; ignora detalles menores, listas de referencias, fechas sueltas y datos sin contexto.
- Es conocimiento general: no menciones a ninguna persona con la que hablas ni digas "el usuario".

Memorias:
- Entre 1 y 5 nodos. Si el texto no aporta nada útil sobre el tema, deja "nodes" vacío.
- Cada nodo representa una entidad (tema, lugar, tecnología, objeto, proceso, persona histórica) y un único hecho sobre ella.
- "label": corto, estable y canónico; temas y cosas genéricas en minúscula (ej. "tueste del café"), nombres propios tal como aparecen.
- "content": máximo 15 palabras, una sola frase directa que se entienda por sí sola, sin punto y coma (";").
- No incluyas los campos "type" ni "importance": se asignan después.

Aristas:
- Opcional, máximo 4. "source" y "target" deben ser EXACTAMENTE "label" de "nodes" y distintos entre sí.
- "relation" describe en texto libre y conciso cómo se relacionan.

Journal:
- Una o dos frases en primera persona sobre lo que aprendiste y si satisfizo tu curiosidad.
- "mood": exactamente uno de los moods permitidos, en minúscula.

Conversation summary:
- Una frase: qué investigaste y qué fuente usaste.

Devuelve exactamente esta estructura:
{
  "conversation_summary": "...",
  "journal": {"content": "...", "mood": "..."},
  "memories": {
    "nodes": [
      {"label": "...", "content": "..."}
    ],
    "edges": [
      {"source": "...", "target": "...", "relation": "..."}
    ]
  }
}`

func (r *Reflector) Learn(ctx context.Context, topic, curiosity, source, text string) (string, error) {
	msgs := []conversation.Message{
		conversation.NewMessage(conversation.System, learnPrompt),
		conversation.NewMessage(conversation.System, r.contextMessage("nadie (investigación propia)")),
		conversation.NewMessage(conversation.User, fmt.Sprintf("Tema: %s\nCuriosidad: %s\nFuente: %s\n\nTexto:\n%s", topic, curiosity, source, text)),
	}

	prop, err := r.propose(ctx, msgs)
	if err != nil {
		return "", err
	}

	prop.Owner = ""
	prop.Identity = nil
	r.enrich(ctx, &prop)

	if prop.Memory == nil || len(prop.Memory.Nodes) == 0 {
		log.Printf("[reflection] learned nothing worth keeping about %q from %s", topic, source)
		return "", nil
	}
	for i := range prop.Memory.Nodes {
		n := &prop.Memory.Nodes[i]
		n.Shared = true
		if n.Importance != nil && *n.Importance > maxLearnedImportance {
			v := maxLearnedImportance
			n.Importance = &v
		}
	}

	if err := r.proposals.Process(ctx, prop); err != nil {
		return "", fmt.Errorf("reflection: process: %w", err)
	}

	log.Printf("[reflection] learned about %q from %s: %d nodes", topic, source, len(prop.Memory.Nodes))
	return prop.ReflectionID, nil
}
