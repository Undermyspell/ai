"""
MCP Server for Knowledge Base RAG Tool
This server exposes the search_knowledge_base tool via the Model Context Protocol.
"""

from mcp.server.fastmcp import FastMCP
from langchain_chroma import Chroma
from langchain_ollama import OllamaEmbeddings
import os

# Initialize FastMCP server
mcp = FastMCP("KnowledgeBase")

# Get absolute paths based on script location
script_dir = os.path.dirname(os.path.abspath(__file__))
project_root = os.path.dirname(script_dir)
PERSIST_DIRECTORY = os.path.join(project_root, "chromadb")

# Initialize embeddings and vector store
embedding_model = OllamaEmbeddings(model="llama3.1:8b")
vector_store = Chroma(
    collection_name="my_docs",
    embedding_function=embedding_model,
    persist_directory=PERSIST_DIRECTORY
)

# Create retriever
num_results = 5
retriever = vector_store.as_retriever(
    search_type="mmr",
    search_kwargs={"k": num_results}
)


@mcp.tool()
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


if __name__ == "__main__":
    print("Starting Knowledge Base MCP Server...")
    print("Server name: KnowledgeBase")
    print("Tools available: search_knowledge_base")
    print("Transport: stdio")
    mcp.run(transport="stdio")
