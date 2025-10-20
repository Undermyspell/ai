# MCP Integration Example

## Overview

This example demonstrates how to use **Model Context Protocol (MCP)** with LangChain agents to dynamically load tools from MCP servers instead of hardcoding them.

## Files

### 1. `mcp_knowledge_server.py`
MCP server that exposes the `search_knowledge_base` tool using the FastMCP framework.

**Features:**
- Runs as a stdio server (subprocess)
- Exposes RAG tool for searching the vector database
- Uses `@mcp.tool()` decorator for automatic tool registration

### 2. `chatbot_mcp.py`
Agent-based chatbot that connects to MCP servers and dynamically loads tools.

**Features:**
- Uses `MultiServerMCPClient` to connect to multiple MCP servers
- Dynamically loads tools at runtime (no hardcoded tools!)
- Creates LangGraph agent with MCP-provided tools
- Gradio interface for user interaction

## Key Differences: Hardcoded vs MCP

### Hardcoded Tools (`chatbot.py`)
```python
# ❌ Static - tools defined in code
@tool
def search_knowledge_base(query: str) -> str:
    """Search knowledge base"""
    ...

tools = [search_knowledge_base]  # Hardcoded list
agent = create_react_agent(llm, tools)
```

### MCP Dynamic Tools (`chatbot_mcp.py`)
```python
# ✅ Dynamic - tools loaded from MCP servers
mcp_client = MultiServerMCPClient({
    "knowledge_base": {
        "transport": "stdio",
        "command": python_executable,
        "args": ["/path/to/mcp_knowledge_server.py"],
    }
})

tools = await mcp_client.get_tools()  # Dynamic loading!
agent = create_react_agent(llm, tools)
```

## Architecture

```
┌─────────────────────────────────────────────────┐
│           Gradio UI (chatbot_mcp.py)            │
└───────────────────┬─────────────────────────────┘
                    │
                    ▼
┌─────────────────────────────────────────────────┐
│          LangGraph Agent (ReAct)                │
│    - Decides when to use tools                  │
│    - Maintains conversation memory              │
└───────────────────┬─────────────────────────────┘
                    │
                    ▼
┌─────────────────────────────────────────────────┐
│      MultiServerMCPClient                       │
│    - Connects to MCP servers                    │
│    - Loads tools dynamically                    │
└───────────┬──────────────────┬──────────────────┘
            │                  │
            ▼                  ▼
┌──────────────────┐  ┌──────────────────┐
│ Knowledge Base   │  │  Future MCP      │
│  MCP Server      │  │  Servers         │
│  (stdio)         │  │  (http/sse)      │
│                  │  │                  │
│ Tool:            │  │ Tools:           │
│ - search_kb      │  │ - web_search     │
│                  │  │ - calculator     │
│                  │  │ - ...            │
└──────────────────┘  └──────────────────┘
```

## Benefits of MCP

1. **Modularity**: Tools are defined in separate MCP servers
2. **No Code Changes**: Add new tools by connecting to new MCP servers
3. **Reusability**: Same MCP server can be used by multiple agents
4. **Standardization**: MCP is an open protocol supported by many tools
5. **Flexibility**: Mix stdio (local) and HTTP (remote) MCP servers

## Usage

### 1. Start the MCP-based chatbot

```bash
cd agent
python chatbot_mcp.py
```

The chatbot will:
1. Connect to the Knowledge Base MCP server
2. Dynamically load the `search_knowledge_base` tool
3. Create an agent with the loaded tools
4. Launch the Gradio UI at http://127.0.0.1:7860

### 2. Test the MCP server independently

```bash
# Run the MCP server directly (for testing)
python mcp_knowledge_server.py
```

## Adding More MCP Servers

You can easily add more MCP servers to provide additional tools:

```python
mcp_client = MultiServerMCPClient({
    "knowledge_base": {
        "transport": "stdio",
        "command": python_executable,
        "args": ["/path/to/mcp_knowledge_server.py"],
    },
    "web_search": {
        "transport": "streamable_http",
        "url": "http://localhost:8000/mcp",
    },
    "calculator": {
        "transport": "stdio",
        "command": python_executable,
        "args": ["/path/to/mcp_calculator_server.py"],
    }
})
```

The agent will automatically have access to all tools from all connected servers!

## Creating Custom MCP Servers

Example MCP server structure:

```python
from mcp.server.fastmcp import FastMCP

mcp = FastMCP("ServerName")

@mcp.tool()
def my_tool(param: str) -> str:
    """Tool description for the LLM"""
    return f"Result: {param}"

if __name__ == "__main__":
    mcp.run(transport="stdio")  # or "streamable-http"
```

## Transport Types

MCP supports different transports:

1. **stdio** (subprocess):
   - Best for local tools
   - Server runs as subprocess
   - Communication via stdin/stdout

2. **streamable-http** (HTTP server):
   - Best for remote tools
   - Server runs independently
   - Supports multiple clients

3. **SSE** (Server-Sent Events):
   - Optimized for real-time streaming
   - Based on HTTP

## Comparison with Barebones

| Feature | Barebones | Agent | Agent + MCP |
|---------|-----------|-------|-------------|
| Tool definition | Inline | Inline | Separate server |
| Tool loading | Hardcoded | Hardcoded | Dynamic |
| Adding tools | Edit code | Edit code | Connect server |
| Tool reusability | No | No | Yes |
| Remote tools | No | No | Yes |
| Standardization | No | No | Yes (MCP) |

## Dependencies

```
langchain-mcp-adapters  # MCP client for LangChain
mcp                     # MCP server framework
langgraph              # Agent framework
langchain-chroma       # Vector database
langchain-ollama       # Ollama integration
gradio                 # UI framework
```

## Resources

- [MCP Documentation](https://modelcontextprotocol.io/introduction)
- [LangChain MCP Adapters](https://github.com/langchain-ai/langchain-mcp-adapters)
- [LangChain MCP Guide](https://docs.langchain.com/oss/python/langchain/mcp)
- [FastMCP Framework](https://github.com/modelcontextprotocol/fastmcp)

## Next Steps

1. Create additional MCP servers for different capabilities
2. Use HTTP transport for remote MCP servers
3. Implement stateful MCP sessions for complex workflows
4. Deploy MCP servers as microservices
5. Share MCP servers across multiple agents
