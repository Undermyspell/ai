"""
Agent-based RAG Chatbot using LangChain and MCP.
This chatbot dynamically loads tools from MCP servers instead of hardcoding them.
"""

import gradio as gr
import asyncio
import os
import sys
from langchain.chat_models import init_chat_model
from langchain_mcp_adapters.client import MultiServerMCPClient
from langgraph.prebuilt import create_react_agent
from langgraph.checkpoint.memory import MemorySaver


# Initialize LLM
llm = init_chat_model("llama3.1:8b", model_provider="ollama", temperature=0.5)

# Get absolute path to the MCP server and Python executable
current_dir = os.path.dirname(os.path.abspath(__file__))
mcp_server_path = os.path.join(current_dir, "mcp_knowledge_server.py")
python_executable = sys.executable  # Get the current Python interpreter path

# Configure MCP client with knowledge base server
mcp_client = MultiServerMCPClient(
    {
        "knowledge_base": {
            "transport": "stdio",  # Local subprocess communication
            "command": python_executable,  # Use the current Python executable
            "args": [mcp_server_path],
        }
    }
)

# Agent executor will be created asynchronously
agent_executor = None
memory = MemorySaver()


async def initialize_agent():
    """Initialize the agent with tools from MCP servers."""
    global agent_executor
    
    print("Connecting to MCP servers and loading tools...")
    
    # Get tools from all MCP servers
    tools = await mcp_client.get_tools()
    
    print(f"✓ Loaded {len(tools)} tool(s) from MCP servers:")
    for tool in tools:
        print(f"  - {tool.name}: {tool.description}")
    
    # Create the agent with dynamically loaded tools
    agent_executor = create_react_agent(llm, tools, checkpointer=memory)
    print("✓ Agent initialized successfully!")


async def async_stream_response(message, history):
    """
    Stream responses from the agent (async version).
    
    Args:
        message: The user's message
        history: Conversation history from Gradio (list of tuples)
        
    Yields:
        Partial responses as they are generated
    """
    if not message or not agent_executor:
        return
    
    # Use a thread_id based on session
    config = {"configurable": {"thread_id": "gradio_session_001"}}
    
    partial_message = ""
    
    try:
        # Stream the agent's response
        async for step, metadata in agent_executor.astream(
            {"messages": [{"role": "user", "content": message}]},
            config,
            stream_mode="messages"
        ):
            # Only stream the agent's text responses (not tool calls)
            if metadata.get("langgraph_node") == "agent" and hasattr(step, 'text'):
                text = step.text()
                if text:
                    partial_message += text
                    yield partial_message
                    
    except Exception as e:
        error_msg = f"Error: {str(e)}"
        yield error_msg


def stream_response(message, history):
    """
    Synchronous wrapper for async_stream_response.
    Gradio requires a synchronous generator.
    """
    # Run async function in event loop
    loop = asyncio.new_event_loop()
    asyncio.set_event_loop(loop)
    
    try:
        async_gen = async_stream_response(message, history)
        while True:
            try:
                result = loop.run_until_complete(async_gen.__anext__())
                yield result
            except StopAsyncIteration:
                break
    finally:
        loop.close()


# Create Gradio interface
chatbot = gr.ChatInterface(
    stream_response,
    textbox=gr.Textbox(
        placeholder="Ask me anything about the ingested documents...",
        container=False,
        scale=7
    ),
    title="🤖 AI Agent with MCP (Model Context Protocol)",
    description="""
    This chatbot uses an AI agent that dynamically loads tools from MCP servers.
    
    **Connected MCP Servers:**
    - Knowledge Base Server (stdio): Provides search_knowledge_base tool
    
    The agent can intelligently decide when to use these tools to answer your questions.
    Tools are loaded dynamically - no hardcoded tool definitions!
    """,
    examples=[
        "What information do you have in your knowledge base?",
        "Can you summarize the main topics from the documents?",
        "What is discussed about [your topic]?",
    ]
)


if __name__ == "__main__":
    print("="*60)
    print("AI Agent with MCP (Model Context Protocol)")
    print("="*60)
    
    # Initialize agent with MCP tools
    loop = asyncio.new_event_loop()
    asyncio.set_event_loop(loop)
    loop.run_until_complete(initialize_agent())
    loop.close()
    
    print("\nLaunching Gradio interface...")
    chatbot.launch()
