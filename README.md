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

Aurora is a self-hosted Discord and Telegram bot written in Go. Its identity, memory, and journal live on your machine; the models are reached through [OpenRouter](https://openrouter.ai). It isn't a traditional chatbot: it remembers conversations, learns from them, and builds a richer understanding of the people it talks to, while its identity stays stable.
 
> Pull requests are welcome — new skills, bug fixes, or general improvements.
 
## Features
 
- **Persistent memory** — long-term memories embedded in [Qdrant](https://qdrant.tech), plus a cognitive graph of people, concepts, and events.
- **Reflection** — every N messages Aurora summarizes the conversation, writes its journal, and consolidates memories.
- **Decision model (JEV)** — a small, fast model filters and classifies memories, rates importance, and guards identity changes.
- **Evolving identity** — reflections may propose new values or principles; they're only applied if the decision model judges them coherent and safe.
- **Skills** — tool calling for live data (time, SIATA radar, Cornare stations, calendar).
- **MCP servers** — plug in any [MCP](https://modelcontextprotocol.io) server through `mcp.json`; its tools become skills.
- **Guard** — every tool call is logged, checked against fixed rules, and reviewed by the decision model. Secrets are redacted locally.
- **Discord and Telegram** — one identity and memory across both, with optional single-user mode.
## How it works
 
```
Discord / Telegram ──▶ agent.Reply ── recall memories · inject reflection & interests
                            │
                            ▼
                     LLM (OpenRouter) ── needs a tool? ──▶ Registry ──▶ Guard ──▶ skill or MCP tool
                            ▲                                                          │
                            └──────────────────── result ◀─────────────────────────────┘
 
every N messages ──▶ reflection LLM ──▶ decision model (keep? type? importance? mood?) ──▶ memory · journal · identity
```
 
The LLM is stateless and swappable; everything persistent lives in:
 
| Path | Contents |
| --- | --- |
| `data/aurora.json` | Identity, values, principles, and memory tuning (Aurora edits it when an identity change is approved) |
| `data/aurora.edges.json` | Cognitive graph edges |
| `data/journal.md` | Aurora's diary, including every identity change |
| `data/guard.log` | One JSON line per tool call and the guard's decision |
| Qdrant storage | Memory nodes and vectors |
 
A read-only HTTP API on port `8095` exposes the graph (`GET /api/v1/edges`) and the journal (`GET /api/v1/journal`).
 
## Skills
 
Built-in: `hora_actual`, `radar_siata`, `cornare`, `calendar_*` (if `APIROC_*` is set), and `listar_capacidades` (lists everything Aurora can do). Requires a chat model with tool calling.
 
To add one, implement `skills.Skill` (`internal/skills/interfaces.go`) in a subpackage and register it in `cmd/main.go`. Skill results are treated as live data and never become memories.
 
## MCP servers
 
Declare servers in `mcp.json` (same `mcpServers` format as most MCP clients; see `mcp.example.json` for every field). Without the file, Aurora starts without MCP.
 
```json
{
  "mcpServers": {
    "time": {
      "command": "uvx",
      "args": ["mcp-server-time", "--local-timezone=America/Bogota"],
      "allow": ["*"]
    },
    "remote": {
      "url": "https://example.com/mcp",
      "headers": { "Authorization": "Bearer ${REMOTE_TOKEN}" },
      "allow": ["search_*"],
      "confirm": ["delete_*"]
    }
  }
}
```
 
- Transports: stdio (`command`), Streamable HTTP (`url`), or legacy SSE (`"type": "sse"`).
- `allow` lists the tools exposed to the model — **empty means none**. `confirm` lists tools that need confirmation (blocked for now).
- `${VAR}` is expanded from the environment. Tools are named `server__tool`.
- Servers connect in the background and reconnect automatically.
> stdio servers inherit Aurora's environment, including API keys. Only allow the tools you need.
 
## Guard
 
Every tool call goes through `internal/guard`:
 
1. Tools in `confirm` are not executed.
2. For MCP tools, the decision model checks whether the user asked for it, its risk (read, write, destructive, third parties, money, system, credentials), and whether it carries sensitive data. It fails closed.
3. API keys, tokens, passwords, and card numbers are redacted locally before reaching the decision model or `data/guard.log`.
Blocked calls return the reason to the LLM so it can tell the user.
 
## Requirements
 
- Go 1.26+
- A Discord bot, and optionally a Telegram bot
- An [OpenRouter](https://openrouter.ai) API key
- Docker (Qdrant, and the image includes Node.js and `uv` for MCP servers)
## Configuration
 
| Variable | Required | Default | Description |
| --- | --- | --- | --- |
| `DISCORD_TOKEN` | yes | — | Discord bot token |
| `TELEGRAM_BOT_TOKEN` | no | — | Enables Telegram |
| `OPENROUTER_API_KEY` | yes | — | OpenRouter API key |
| `OPENROUTER_BASE_URL` | no | `https://openrouter.ai/api/v1` | OpenRouter (or compatible) base URL |
| `OPENROUTER_CHAT_MODEL` | yes | — | Chat model (must support tool calling) |
| `OPENROUTER_EMBED_MODEL` | yes | — | Embedding model |
| `OPENROUTER_REFLECTION_MODEL` | no | chat model | Reflection model |
| `OPENROUTER_DECISION_MODEL` | no | `jev-latest` | Decision model |
| `ALLOWED_DISCORD_USER_ID` | no | — | Restrict Discord to one user |
| `ALLOWED_TELEGRAM_USER_ID` | no | — | Restrict Telegram to one user |
| `APIROC_API_KEY`, `APIROC_USER_ID` | no | — | Enable the calendar skills |
| `APIROC_BASE_URL` | no | `https://api.apiroc.com/api/v1` | Calendar API base URL |
| `QDRANT_URL` | no | `http://localhost:6333` | Qdrant base URL |
| `QDRANT_API_KEY` | no | — | Qdrant API key |
| `QDRANT_COLLECTION` | no | `aurora_memories` | Qdrant collection |
 
Identity and memory tuning live in `data/aurora.json` (values, principles, and thresholds for recall, reflection, and the decision model). Missing or zero fields use defaults.
 
## Running
 
```
docker compose up -d --build
docker compose logs -f aurora
```
 
Qdrant's collection is created automatically on first run. Watch the logs (`[reflection]`, `[memory]`, `[identity]`, `[mcp]`) and `data/journal.md` to see Aurora evolve.
 
## License
 
[Mozilla Public License 2.0](LICENSE).