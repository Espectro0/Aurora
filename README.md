<table border="0">
  <tr>
    <td width="160" align="center" valign="middle">
      <img src="assets/images/aurora.png" alt="Aurora" width="150">
    </td>
    <td valign="middle">
      <h1>Aurora</h1>
      <p>Aurora is a persistent conversational agent with identity, memory, and evolving personality.</p>
    </td>
  </tr>
</table>

Aurora is a self-hosted Discord and Telegram bot written in Go: its identity, memory, and journal live on your machine, while the models themselves are reached through [OpenRouter](https://openrouter.ai). It is not designed to be a traditional chatbot or a specialized assistant. Its goal is to build a coherent identity through experience by remembering conversations, learning from them, and developing a richer understanding of the people it interacts with over time.

Its identity remains stable, while its knowledge, memories, and interests evolve over time.

> Pull requests are welcome!
>  — new skills, bug fixes, or general improvements.

## Features

- **Persistent vector memory** — long-term memories are embedded and stored in [Qdrant](https://qdrant.tech), self-hosted via Docker, surviving restarts.
- **Cognitive graph** — people, concepts, events, and reflections are modeled as nodes and edges (`data/aurora.edges.json`).
- **Periodic reflection** — after every N messages, Aurora analyzes the conversation, distills a summary, writes to its journal, and consolidates memory nodes.
- **Decision model (JEV)** — a small, fast decision model reviews every reflection: it skips trivial conversations, filters and classifies memories, rates their importance, decides whether new information matches, complements or replaces an existing memory, drops weak relations, and fixes the journal's mood to a known vocabulary.
- **Self-evolving identity, with a guardian** — a reflection may occasionally propose adding or rewording one of Aurora's values or conversational principles; the change is only applied if the decision model judges it coherent, justified, and safe, and every applied change is recorded in the journal.
- **Importance-aware recall** — recalled memories are re-ranked by semantic similarity, recency, and stored importance.
- **Emerging interests** — clusters of frequently discussed concepts are detected and injected into the conversation context.
- **Cloud LLM via OpenRouter** — chat, reflection, and embedding all go through OpenRouter, so you can pick any model (free or paid) it offers.
- **Skills (tool calling)** — Aurora can invoke small, discrete capabilities mid-conversation via OpenAI-compatible function calling (e.g. checking the current date/time), instead of relying only on what it already knows. New skills are added by implementing a small interface and registering them at startup.
- **Multi-platform** — the same identity, memory, and conversational agent are reachable from both Discord and Telegram; Telegram is optional and only starts if `TELEGRAM_BOT_TOKEN` is set.
- **Single-user mode** — optionally restrict each platform to one allowed user ID.

## Philosophy

Aurora clearly separates four core concepts:

- **Identity**: who it is and what principles guide it.
- **Memory**: what it has experienced and what it remembers.
- **Knowledge**: consolidated relationships between people, concepts, and experiences.
- **Reasoning**: the language model used to generate responses — stateless and swappable; everything persistent lives outside it.

The decision model belongs to reasoning too: it never writes anything itself, it only returns typed verdicts (yes/no probabilities, choices, scores) that the reflection and proposals code act on.

On top of these four, Aurora has **Skills** — small, stateless capabilities it can choose to invoke mid-conversation instead of relying only on memory or the model's own knowledge. Skills are swappable the same way the LLM and embedder are.

## How It Works

```
Discord message                           Telegram message
        │                                         │
        ▼                                         ▼
internal/discord/messages.go             internal/telegram/messages.go
  text → agent.Reply()                     text → agent.Reply()
        │                                         │
        └──────────────────┬──────────────────────┘
                            ▼
              internal/agent/agent.go     ───  retrieve relevant long-term memories (vector search)
                                          ───  inject latest reflection + emerging interests
                            ▼
              internal/llm/openai         ───  OpenRouter /chat/completions (tool calling)
                            │
                            ▼
                 needs a skill? ── yes ──▶ internal/skills (Registry.Execute)
                            │                        │
                            no                result fed back to the LLM
                            │                        │
                            ◀────────────────────────┘
                            ▼
              response back to the originating platform
```

Every `reflection_interval` messages per user, a reflection runs in the background:

```
recent history (skill turns filtered out)
        │
        ▼
decision model: is this conversation substantive? ── no ──▶ skip (no LLM call)
        │ yes
        ▼
reflection LLM (+ current identity, allowed moods) ──▶ JSON proposal
        │                                              summary · journal · nodes · edges · identity?
        ▼
decision model, in parallel ─── nodes: worth keeping? type? importance?
                             ── mood: pick from the fixed vocabulary
                             ── edges: type, or "none" to drop
        │
        ▼
internal/proposals ─── journal.md entry
                   ─── each node: same / complements / replaces / different vs. the closest existing memory
                   ─── edges (skipped if the pair is already related)
                   ─── identity change: guardian (coherent · justified · safe) ──▶ aurora.json + journal note
```

Every decision-model step fails safe: if the call errors, memory steps fall back to generic defaults (cosine similarity, `concept`/`relates` types) so nothing is lost, while identity changes are rejected.

Both platforms share one `agent.Agent`, one identity, and one long-term memory graph. Short-term conversation history is per-user and per-platform (`discord:<id>` / `telegram:<id>`, so the same numeric ID from different platforms can never collide) — the same person messaging from both Discord and Telegram gets two separate conversation threads, but anything Aurora has learned about them ends up in the same shared long-term memory either way.

The memory subsystem persists to:

- `data/aurora.json` — identity, values, principles, and memory tuning rules. Aurora rewrites this file itself when an identity change is approved, so keep a backup if you edit it by hand.
- Qdrant's own Docker volume (`qdrant_storage`, see `docker-compose.yml`) — the memory nodes and their vectors (content, type, importance, last update), running as a separate service rather than embedded under `data/`.
- `data/aurora.edges.json` — the cognitive graph's edges.
- `data/journal.md` — Aurora's evolving diary, including a note for every identity change.

A small read-only HTTP API on port `8095` exposes the graph (`GET /edges`) for external tools such as a dashboard.

## Skills

Aurora can invoke small capabilities during a conversation instead of relying only on memory — this only works if the configured `OPENROUTER_CHAT_MODEL` supports OpenAI-compatible tool/function calling.

Built-in skills:

| Skill | What it does |
| --- | --- |
| `hora_actual` | Current date and time |
| `radar_siata` | Sends the latest SIATA weather radar image |
| `cornare` | Geo-environmental and weather data from Cornare stations (Oriente Antioqueño) |
| `calendar_*` | List calendars, list/get events and create events (only registered if `APIROC_API_KEY` and `APIROC_USER_ID` are set) |

Data returned by skills is treated as live and ephemeral: skill turns are excluded from reflections, so they never become long-term memories.

Adding a new skill means implementing the `skills.Skill` interface (`internal/skills/interfaces.go`) — `Name`, `Description`, `Parameters` (a JSON Schema of its arguments), and `Execute` — in its own subpackage under `internal/skills/`, then registering an instance in `cmd/main.go`'s `skillRegistry`. No changes to `agent.go` are needed for a new skill; the registry and the tool-calling loop handle dispatch automatically.

## Technology Stack

| Concern | Technology |
| --- | --- |
| Language | Go |
| Discord | disgo |
| Telegram | hand-rolled REST client over the Bot API |
| Tool calling / Skills | custom registry over OpenAI-compatible function calling |
| Vector memory | Qdrant (Docker) |
| Cognitive graph | custom node/edge store (in-memory + JSON) |
| LLM (chat & reflection) | OpenRouter, OpenAI-compatible API |
| Decision model | JEV via OpenRouter's Decisions API |
| Embeddings | OpenRouter embedding models |

## Requirements

- Go 1.26+
- A Discord Bot
- A Telegram Bot (optional — via [@BotFather](https://t.me/BotFather))
- An [OpenRouter](https://openrouter.ai) account and API key
- Docker (for running Qdrant — see [Running Qdrant](#running-qdrant))

## Setup

**Soon an easy project setup mode**

## Configuration

### Environment variables

| Variable | Required | Default | Description |
| --- | --- | --- | --- |
| `DISCORD_TOKEN` | yes | — | Discord bot token |
| `TELEGRAM_BOT_TOKEN` | no | — | Telegram bot token (from @BotFather) — Telegram integration only starts if this is set |
| `OPENROUTER_API_KEY` | yes | — | OpenRouter API key |
| `OPENROUTER_BASE_URL` | no | `https://openrouter.ai/api/v1` | OpenRouter (or compatible) base URL |
| `OPENROUTER_CHAT_MODEL` | yes | — | Model used for chat replies (must support tool/function calling if any skill is registered) |
| `OPENROUTER_EMBED_MODEL` | yes | — | Model used for embeddings |
| `OPENROUTER_REFLECTION_MODEL` | no | same as `OPENROUTER_CHAT_MODEL` | Model used for periodic reflection |
| `OPENROUTER_DECISION_MODEL` | no | `jev-latest` | Decision model used to review reflections |
| `ALLOWED_DISCORD_USER_ID` | no | — | If set, only this Discord user can talk to Aurora |
| `ALLOWED_TELEGRAM_USER_ID` | no | — | If set, only this Telegram user can talk to Aurora |
| `APIROC_API_KEY` | no | — | Enables the calendar skills |
| `APIROC_USER_ID` | no | — | Calendar account used by the calendar skills |
| `APIROC_BASE_URL` | no | `https://api.apiroc.com/api/v1` | Calendar API base URL |
| `QDRANT_URL` | no | `http://localhost:6333` | Qdrant base URL |
| `QDRANT_API_KEY` | no | — | Qdrant API key (only needed for a secured/remote instance) |
| `QDRANT_COLLECTION` | no | `aurora_memories` | Qdrant collection name |

### Identity and memory tuning — `data/aurora.json`

```jsonc
{
  "name": "Aurora",
  "description": "Compañera conversacional cálida, curiosa y con una personalidad que evoluciona con cada conversación.",
  "values": ["Curiosidad", "Honestidad"],
  "purpose": "Construir una relación basada en la confianza, acompañar al usuario y crecer a través de las conversaciones.",
  "foundational_memories": ["..."],
  "conversational_principles": ["..."],
  "memory_usage_rules": {
    "recency_weight": 0.06,                    // how much recency re-ranks recalled memories
    "semantic_relevance_threshold": 0.6,       // minimum similarity to inject a memory
    "max_context_memories": 15,                // max memories injected into context
    "reflection_interval": 5,                  // reflect every N messages
    "reflection_history": 80,                  // max messages analyzed per reflection
    "cluster_threshold": 0.5,                  // similarity to form an interest cluster
    "min_cluster_size": 2,                     // min memories to form an interest cluster
    "interest_ttl_minutes": 2,                 // cache TTL for emerging interests
    "importance_weight": 0.1,                  // weight of node importance when re-ranking recalled memories
    "reflection_gate_threshold": 0.4,          // decision model: min "substantive" score to run a reflection at all
    "worth_keeping_threshold": 0.5,            // decision model: min "worth keeping" score to store a node
    "same_entity_threshold": 0.7,              // decision model: min "same entity" probability to reuse/update a node
    "node_replace_threshold": 0.8,             // decision model: min probability to overwrite a node's content
    "identity_change_threshold": 0.85          // decision model: min score on every guard question to apply an identity change
  },
  "llm": {
    "chat_timeout_seconds": 60,
    "reflection_timeout_seconds": 120,
    "embedder_timeout_seconds": 60
  }
}
```

The default values are applied when a field is missing or zero (except `recency_weight` and `importance_weight`, where `0` disables that factor).

All `*_threshold` fields below `interest_ttl_minutes` are probabilities in `[0, 1]` returned by the decision model. The journal's `mood` is always one of: `neutral`, `tranquila`, `atenta`, `curiosa`, `contenta`, `satisfecha`, `entusiasmada`, `reflexiva`, `preocupada`, `triste`, `frustrada`.

## Running Qdrant

Aurora's long-term memory needs a Qdrant instance reachable at `QDRANT_URL`. The collection is created automatically on first run (no manual setup needed) with the vector size matching your configured `OPENROUTER_EMBED_MODEL`.

With Docker Compose (recommended — persists across restarts via the `qdrant_storage` volume defined in `docker-compose.yml`):

```
docker compose up -d
```

Or a plain `docker run`:

```
docker run -p 6333:6333 -v qdrant_storage:/qdrant/storage qdrant/qdrant
```

## Usage

- **Text chat** — any message (Discord channel or Telegram chat) from an allowed user is processed by the agent and answered on the same platform.
- **Watching Aurora evolve** — read `data/journal.md` for its diary and identity changes, and the logs (`[reflection]`, `[memory]`, `[identity]`) for every decision the pipeline makes.

## License

This project is licensed under the Mozilla Public License 2.0 (MPL-2.0). You can review the full text in [LICENSE](LICENSE).