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

Aurora is a Discord bot written in Go that lives entirely on your machine. It is not designed to be a traditional chatbot or a specialized assistant. Its goal is to build a coherent identity through experience by remembering conversations, learning from them, and developing a richer understanding of the people it interacts with over time.

Its identity remains stable, while its knowledge, memories, and interests evolve over time.

## Features

- **Persistent vector memory** — long-term memories are embedded and stored locally with `chromem-go`, surviving restarts.
- **Cognitive graph** — people, concepts, events, and reflections are modeled as nodes and edges (`data/aurora.edges.json`).
- **Periodic reflection** — after every N messages, Aurora analyzes the conversation, distills a summary, writes to its journal, and consolidates memory nodes.
- **Emerging interests** — clusters of frequently discussed concepts are detected and injected into the conversation context.
- **Voice note transcription** — audio attachments are transcribed locally with whisper.cpp (ffmpeg handles Opus/Ogg conversion).
- **100% local AI** — chat, reflection, and embedding run through llama.cpp servers started on demand, with no external API keys.

## Philosophy

Aurora clearly separates four core concepts:

- **Identity**: who it is and what principles guide it.
- **Memory**: what it has experienced and what it remembers.
- **Knowledge**: consolidated relationships between people, concepts, and experiences.
- **Reasoning**: the language model used to generate responses.

The language model is interchangeable and does not store permanent state. All persistence lives outside the LLM. Aurora talks to its models through the OpenAI-compatible HTTP API exposed by a local `llama-server`, so any OpenAI-compatible endpoint can be swapped in.

## How It Works

```
Discord message (text or audio)
        │
        ▼
internal/discord/bot.go     ───  text → agent.Reply()
                              ───  audio → download → whisper.cpp → agent.Reply()
        │
        ▼
internal/agent/agent.go     ───  retrieve relevant long-term memories (vector search)
        │                      ───  inject latest reflection + emerging interests
        ▼
internal/localai/server.go  ───  ensures llama-server is up (chat / embed), /health
        │
        ▼
OpenAI-compatible /chat/completions  →  response back to Discord
        │
        ▼
Reflection every N messages → journal.md + memory node consolidation
```

The memory subsystem persists to three files under `data/`:

- `aurora.json` — identity, values, principles, and memory tuning rules.
- `aurora.vec/` — embedded vector database.
- `aurora.edges.json` — the cognitive graph (nodes/edges).
- `journal.md` — Aurora's evolving diary.

## Technology Stack

| Concern | Technology |
| --- | --- |
| Language | Go |
| Discord | disgo |
| Vector memory | chromem-go |
| Cognitive graph | custom node/edge store (chromem + JSON) |
| LLM (chat & reflection) | llama.cpp `llama-server`, OpenAI-compatible API |
| Embeddings | llama.cpp `llama-server --embeddings` |
| Speech-to-text | whisper.cpp `whisper-cli` |
| Audio conversion | ffmpeg (Opus/Ogg → WAV 16 kHz) |

## Requirements

- Go 1.26+
- A Discord Bot
- Local binaries in `tools/` (gitignored): `llama-server`, `whisper-cli`, `ffmpeg`
- GGUF model files for chat and embedding (an optional code model is reserved)
- NVIDIA GPU recommended (CUDA build of llama.cpp); CPU works but is slower

## Setup

**Soon an easy project setup mode**

## Configuration

### Environment variables

| Variable | Required | Default | Description |
| --- | --- | --- | --- |
| `DISCORD_TOKEN` | yes | — | Discord bot token |
| `STT_BIN_PATH` | yes | — | Path to `whisper-cli` |
| `STT_MODEL_PATH` | yes | — | Path to the whisper GGML model |
| `STT_LANGUAGE` | no | `es` | Whisper language code |
| `FFMPEG_BIN_PATH` | no | `tools/ffmpeg/ffmpeg.exe` | Path to ffmpeg |
| `LLAMA_BIN_PATH` | no | `tools/llama/llama-server.exe` | Path to llama-server |
| `LLAMA_CHAT_MODEL_PATH` | no | — | Chat GGUF model |
| `LLAMA_EMBED_MODEL_PATH` | no | — | Embedding GGUF model |
| `LLAMA_CODE_MODEL_PATH` | no | — | Reserved code GGUF model |
| `LLAMA_PORT_CHAT` | no | `8080` | Chat/reflection server port |
| `LLAMA_PORT_EMBED` | no | `8081` | Embedding server port |
| `LLAMA_CONTEXT` | no | `4096` | Context window size |
| `LLAMA_IDLE_TIMEOUT_MINUTES` | no | `10` | Minutes of idle before the server stops (`0` = keep alive) |
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

## Local AI On Demand

`internal/localai` manages the llama.cpp servers:

- Servers are **started lazily** on the first request and reused afterwards.
- Chat and reflection share one server (`LLAMA_PORT_CHAT`, default `8080`).
- Embeddings run on a second server with `--embeddings` (`LLAMA_PORT_EMBED`, default `8081`).
- Startup waits on the `/health` endpoint before serving requests.
- Idle servers are stopped after `LLAMA_IDLE_TIMEOUT_MINUTES`; both are killed on shutdown.

## Usage

- **Text chat** — any message in a server channel is processed by the agent and answered in the same channel.
- **Voice notes / audio** — attachments with an audio content type (`.ogg`, `.opus`, `.mp3`, `.wav`, `.flac`, `.m4a`, `.mp4`, `.aac`) are downloaded, converted, and transcribed before being sent to the agent.
- **Inspect** — `go run ./cmd/inspect` prints the current nodes, edges, emerging interests, and journal tail.

## License

This project is licensed under the Mozilla Public License 2.0 (MPL-2.0). You can review the full text in [LICENSE](LICENSE).