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

It is not designed to be a traditional chatbot or a specialized assistant. Its goal is to build a coherent identity through experience by remembering conversations, learning from them, and developing a richer understanding of the people it interacts with over time.

Its identity remains stable, while its knowledge, memories, and interests evolve over time.

## Philosophy

Aurora clearly separates four core concepts:

- **Identity**: who it is and what principles guide it.
- **Memory**: what it has experienced and what it remembers.
- **Knowledge**: consolidated relationships between people, concepts, and experiences.
- **Reasoning**: the language model used to generate responses.

The language model is interchangeable and does not store permanent state. All persistence lives outside the LLM.

## Technology Stack

- **Go** as the primary language.
- **chromem-go** for embedded vector memory.
- **Cayley** or a custom implementation for the cognitive graph.
- Interchangeable LLM providers.

## Roadmap

- [x] Basic Discord bot implementation
- [ ] Embedding system
- [ ] chromem-go integration
- [ ] Semantic memory retrieval
- [ ] Automatic reflection on memories
- [ ] Persistent cognitive graph
- [ ] Emergent interest detection
- [ ] Knowledge consolidation
- [ ] Inspection and visualization tools for cognitive state
- [ ] Dashboard for exploring memory, relationships, and interests
- [ ] Physical robot prototype

## License

This project is licensed under the Mozilla Public License 2.0 (MPL-2.0). You can review the full text in [LICENSE](LICENSE).