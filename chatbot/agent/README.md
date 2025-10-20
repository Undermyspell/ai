# AI Agent with RAG

This implementation follows the [LangChain Agents tutorial](https://python.langchain.com/docs/tutorials/agents/) to build an intelligent agent-based RAG (Retrieval Augmented Generation) chatbot.

## Overview

This agent-based implementation provides the same functionality as the barebones version but with key improvements:

### Key Differences from Barebones

**Barebones Approach:**
- Always retrieves documents for every query
- Simple prompt stuffing with retrieved context
- No intelligent decision-making about when to use RAG

**Agent Approach:**
- **Intelligent tool usage**: The agent decides when to search the knowledge base
- **ReAct pattern**: The agent can reason, act, and observe in a loop
- **Memory**: Maintains conversation history across multiple turns
- **Tool calling**: Uses LangChain's tool framework for structured interactions
- **Streaming**: Streams both messages and tokens in real-time

## Architecture

### Components

1. **ingest_database.py**: RAG pipeline for document ingestion
   - Loads PDFs from `../data` directory
   - Splits documents into chunks (300 chars with 100 overlap)
   - Stores embeddings in ChromaDB at `../chromadb`

2. **chatbot.py**: Agent-based chatbot
   - Uses `create_react_agent` from LangGraph
   - Custom `search_knowledge_base` tool for RAG retrieval
   - Memory-enabled conversations via `MemorySaver`
   - Gradio interface for user interaction

### The Agent Pattern

The agent uses the **ReAct (Reasoning + Acting)** pattern:

1. **Reason**: The LLM analyzes the user's question
2. **Act**: Decides whether to use the `search_knowledge_base` tool
3. **Observe**: Processes the tool's output
4. **Respond**: Generates a final answer based on retrieved context

This is more sophisticated than simple RAG because the agent can:
- Answer general questions without searching
- Combine multiple tool calls
- Reason about when retrieval is necessary
- Handle follow-up questions using conversation memory

## Usage

### 1. Ingest Documents

First, place your PDF files in the `../data` directory, then run:

```bash
cd agent
python ingest_database.py
```

### 2. Start the Chatbot

```bash
python chatbot.py
```

This will launch a Gradio interface where you can interact with the agent.

## Features

### Tool: search_knowledge_base

The agent has access to a custom tool that:
- Searches the vector database using MMR (Maximum Marginal Relevance)
- Returns top 5 diverse, relevant document chunks
- Formats results for the LLM to process

### Memory Management

- Uses `MemorySaver` from LangGraph for conversation persistence
- Maintains context across multiple messages
- Currently uses a static thread_id (can be extended for multi-user sessions)

### Streaming

The chatbot streams responses in real-time:
- Filters agent reasoning steps to show only final text
- Provides immediate feedback as the agent thinks and responds

## Configuration

Key parameters you can adjust:

```python
# In chatbot.py

# Model selection - IMPORTANT: Use ChatOllama, not OllamaLLM for tool support
embedding_model = OllamaEmbeddings(model="llama3.1:8b")
llm = ChatOllama(model="llama3.1:8b", temperature=0.5)  # ChatOllama supports bind_tools()

# Retrieval settings
num_results = 5  # Number of documents to retrieve
search_type = "mmr"  # Can be "similarity", "mmr", or "similarity_score_threshold"

# Chunking settings (in ingest_database.py)
chunk_size = 300
chunk_overlap = 100
```

## Technical Details

### Why LangGraph?

LangGraph provides:
- **State management**: Tracks conversation and agent state
- **Checkpointing**: Enables memory and resumable conversations
- **Observability**: Built-in tracing and debugging
- **Flexibility**: Easy to extend with more tools and custom logic

### Agent vs Simple RAG

| Feature | Simple RAG (Barebones) | Agent (This Implementation) |
|---------|------------------------|----------------------------|
| Retrieval | Always | Only when needed |
| Decision making | None | LLM decides tool usage |
| Memory | Manual history formatting | Built-in checkpointing |
| Tool orchestration | N/A | Multiple tools possible |
| Extensibility | Limited | Highly extensible |
| Reasoning | Direct prompt | ReAct loop |

## Future Enhancements

Possible improvements:
1. **Multiple tools**: Add web search, calculator, database queries
2. **Custom memory**: Implement user-specific thread_ids for multi-user support
3. **Tool validation**: Add input validation and error handling
4. **Advanced retrieval**: Implement query rewriting, re-ranking
5. **Observability**: Add LangSmith integration for debugging
6. **Persistence**: Use SQLite or Postgres checkpointer for long-term memory

## Dependencies

```
langchain-community
langchain-chroma
langchain-ollama
langchain-text-splitters
langgraph
langgraph-checkpoint-sqlite
gradio
```

## References

- [LangChain Agents Tutorial](https://python.langchain.com/docs/tutorials/agents/)
- [LangGraph Documentation](https://langchain-ai.github.io/langgraph/)
- [ReAct Paper](https://arxiv.org/abs/2210.03629)
