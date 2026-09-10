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

Aurora is a Discord and Telegram bot written in Go that lives entirely on your machine. It is not designed to be a traditional chatbot or a specialized assistant. Its goal is to build a coherent identity through experience by remembering conversations, learning from them, and developing a richer understanding of the people it interacts with over time.

Its identity remains stable, while its knowledge, memories, and interests evolve over time.

> Pull requests are welcome!
>  — new skills, bug fixes, or general improvements.

## Features

- **Persistent vector memory** — long-term memories are embedded and stored in [Qdrant](https://qdrant.tech), self-hosted via Docker, surviving restarts.
- **Cognitive graph** — people, concepts, events, and reflections are modeled as nodes and edges (`data/aurora.edges.json`).
- **Periodic reflection** — after every N messages, Aurora analyzes the conversation, distills a summary, writes to its journal, and consolidates memory nodes.
- **Emerging interests** — clusters of frequently discussed concepts are detected and injected into the conversation context.
- **Voice note transcription** — audio attachments are transcribed locally with whisper.cpp (ffmpeg handles Opus/Ogg conversion).
- **Cloud LLM via OpenRouter** — chat, reflection, and embedding all go through OpenRouter, so you can pick any model (free or paid) it offers.
- **Skills (tool calling)** — Aurora can invoke small, discrete capabilities mid-conversation via OpenAI-compatible function calling (e.g. checking the current date/time), instead of relying only on what it already knows. New skills are added by implementing a small interface and registering them at startup.
- **Multi-platform** — the same identity, memory, and conversational agent are reachable from both Discord and Telegram; Telegram is optional and only starts if `TELEGRAM_BOT_TOKEN` is set.

## Philosophy

Aurora clearly separates four core concepts:

- **Identity**: who it is and what principles guide it.
- **Memory**: what it has experienced and what it remembers.
- **Knowledge**: consolidated relationships between people, concepts, and experiences.
- **Reasoning**: the language model used to generate responses.

On top of these four, Aurora has **Skills** — small, stateless capabilities it can choose to invoke mid-conversation instead of relying only on memory or the model's own knowledge. Skills are swappable the same way the LLM and embedder are.

## How It Works

```
Discord message                           Telegram message
        │                                         │
        ▼                                         ▼
internal/discord/messages.go             internal/telegram/messages.go
  text → agent.Reply()                     text → agent.Reply()
  audio → whisper.cpp → agent.Reply()      voice → whisper.cpp → agent.Reply()
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

Both platforms share one `agent.Agent`, one identity, and one long-term memory graph. Short-term conversation history is per-user and per-platform (`discord:<id>` / `telegram:<id>`, so the same numeric ID from different platforms can never collide) — the same person messaging from both Discord and Telegram gets two separate conversation threads, but anything Aurora has learned about them ends up in the same shared long-term memory either way.

The memory subsystem persists to:

- `aurora.json` — identity, values, principles, and memory tuning rules.
- Qdrant's own Docker volume (`qdrant_storage`, see `docker-compose.yml`) — the vector database, running as a separate service rather than embedded under `data/`.
- `data/aurora.edges.json` — the cognitive graph (nodes/edges).
- `data/journal.md` — Aurora's evolving diary.

## Skills

Aurora can invoke small capabilities during a conversation instead of relying only on memory — this only works if the configured `OPENROUTER_CHAT_MODEL` supports OpenAI-compatible tool/function calling.

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
| Embeddings | OpenRouter embedding models |
| Speech-to-text | whisper.cpp `whisper-cli` |
| Audio conversion | ffmpeg (Opus/Ogg → WAV 16 kHz) |

## Requirements

- Go 1.26+
- A Discord Bot
- A Telegram Bot (optional — via [@BotFather](https://t.me/BotFather))
- An [OpenRouter](https://openrouter.ai) account and API key
- Docker (for running Qdrant — see [Running Qdrant](#running-qdrant))
- Local binaries in `tools/` (gitignored): `whisper-cli`, `ffmpeg`
- NVIDIA GPU recommended for faster local transcription; CPU works but is slower

## Setup

**Soon an easy project setup mode**

## Configuration

### Environment variables

| Variable | Required | Default | Description |
| --- | --- | --- | --- |
| `DISCORD_TOKEN` | yes | — | Discord bot token |
| `TELEGRAM_BOT_TOKEN` | no | — | Telegram bot token (from @BotFather) — Telegram integration only starts if this is set |
| `STT_BIN_PATH` | yes | — | Path to `whisper-cli` |
| `STT_MODEL_PATH` | yes | — | Path to the whisper GGML model |
| `STT_LANGUAGE` | no | `es` | Whisper language code |
| `FFMPEG_BIN_PATH` | no | `tools/ffmpeg/ffmpeg.exe` | Path to ffmpeg |
| `OPENROUTER_API_KEY` | yes | — | OpenRouter API key |
| `OPENROUTER_BASE_URL` | no | `https://openrouter.ai/api/v1` | OpenRouter (or compatible) base URL |
| `OPENROUTER_CHAT_MODEL` | yes | — | Model used for chat replies (must support tool/function calling if any skill is registered) |
| `OPENROUTER_EMBED_MODEL` | yes | — | Model used for embeddings |
| `OPENROUTER_REFLECTION_MODEL` | no | same as `OPENROUTER_CHAT_MODEL` | Model used for periodic reflection |
| `QDRANT_URL` | no | `http://localhost:6333` | Qdrant base URL |
| `QDRANT_API_KEY` | no | — | Qdrant API key (only needed for a secured/remote instance) |
| `QDRANT_COLLECTION` | no | `aurora_memories` | Qdrant collection name |
| `AURORA_KEEP_WAV` | no | — | Keep temp WAV files for debugging |

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
    "interest_ttl_minutes": 2                  // cache TTL for emerging interests
  },
  "llm": {
    "chat_timeout_seconds": 60,
    "reflection_timeout_seconds": 120,
    "embedder_timeout_seconds": 60,
    "transcription_timeout_seconds": 120
  }
}
```

The default values are applied when a field is missing or zero.

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

- **Text chat** — any message (Discord channel or Telegram chat) is processed by the agent and answered on the same platform.
- **Voice notes / audio** — Discord attachments with an audio content type (`.ogg`, `.opus`, `.mp3`, `.wav`, `.flac`, `.m4a`, `.mp4`, `.aac`) and Telegram voice/audio messages are downloaded, converted, and transcribed before being sent to the agent.

## License

This project is licensed under the Mozilla Public License 2.0 (MPL-2.0). You can review the full text in [LICENSE](LICENSE).