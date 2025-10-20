"""
Agent-based RAG Chatbot using LangChain and LangGraph.
This chatbot uses an agent that can decide when to use a RAG retrieval tool
to answer questions based on ingested documents.
"""

import gradio as gr
import os
from langchain_chroma import Chroma
from langchain_ollama import OllamaEmbeddings
from langchain.chat_models import init_chat_model
from langchain_core.tools import tool
from langgraph.prebuilt import create_react_agent
from langgraph.checkpoint.memory import MemorySaver


# Get absolute paths based on script location
script_dir = os.path.dirname(os.path.abspath(__file__))
project_root = os.path.dirname(script_dir)
PERSIST_DIRECTORY = os.path.join(project_root, "chromadb")

# Initialize embeddings and LLM
embedding_model = OllamaEmbeddings(model="llama3.1:8b")
llm = init_chat_model("llama3.1:8b", model_provider="ollama", temperature=0.5)

# Initialize vector store
vector_store = Chroma(
    collection_name="my_docs",
    embedding_function=embedding_model,
    persist_directory=PERSIST_DIRECTORY
)

# Create retriever
num_results = 5
retriever = vector_store.as_retriever(
    search_type="mmr",  # Use MMR for diverse results
    search_kwargs={"k": num_results}
)


# Define custom RAG tool
@tool
def search_knowledge_base(query: str) -> str:
    """
    Search the knowledge base for relevant information about the query.
    Use this tool when you need to find specific information from ingested documents.
    
    Args:
        query: The search query to find relevant documents
        
    Returns:
        A string containing relevant information from the knowledge base
    """
    docs = retriever.invoke(query)
    
    if not docs:
        return "No relevant information found in the knowledge base."
    
    # Format the retrieved documents
    knowledge = "Found the following relevant information:\n\n"
    for i, doc in enumerate(docs, 1):
        knowledge += f"--- Document {i} ---\n{doc.page_content}\n\n"
    
    return knowledge.strip()


# Create the agent with memory
tools = [search_knowledge_base]
memory = MemorySaver()
agent_executor = create_react_agent(llm, tools, checkpointer=memory)


def stream_response(message, history):
    """
    Stream responses from the agent.
    
    Args:
        message: The user's message
        history: Conversation history from Gradio (list of tuples)
        
    Yields:
        Partial responses as they are generated
    """
    if not message:
        return
    
    # Use a thread_id based on session (here we use a simple static one)
    # In production, you'd want to generate this per user session
    config = {"configurable": {"thread_id": "gradio_session_001"}}
    
    partial_message = ""
    
    try:
        # Stream the agent's response
        for step, metadata in agent_executor.stream(
            {"messages": [{"role": "user", "content": message}]},
            config,
            stream_mode="messages"
        ):
            # Only stream the agent's text responses (not tool calls)
            if metadata.get("langgraph_node") == "agent" and (text := step.text()):
                partial_message += text
                yield partial_message
                
    except Exception as e:
        error_msg = f"Error: {str(e)}"
        yield error_msg


# Create Gradio interface
chatbot = gr.ChatInterface(
    stream_response,
    textbox=gr.Textbox(
        placeholder="Ask me anything about the ingested documents...",
        container=False,
        scale=7
    ),
    title="🤖 AI Agent with RAG",
    description="""
    This chatbot uses an AI agent that can intelligently decide when to search 
    the knowledge base. The agent has access to ingested documents and will 
    retrieve relevant information when needed to answer your questions.
    
    The agent remembers conversation history within the session.
    """,
    examples=[
        "What information do you have in your knowledge base?",
        "Can you summarize the main topics from the documents?",
        "What is discussed about [your topic]?",
    ]
)


if __name__ == "__main__":
    print("Starting Agent-based RAG Chatbot...")
    print(f"Agent has access to {len(tools)} tool(s)")
    print("Launching Gradio interface...")
    chatbot.launch()
