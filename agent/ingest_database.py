"""
RAG Pipeline - Ingests PDF documents into a vector database.
This script loads PDFs from a directory, splits them into chunks, 
and stores embeddings in ChromaDB for retrieval.
"""

from langchain_community.document_loaders import PyPDFDirectoryLoader
from langchain_chroma import Chroma
from langchain_ollama import OllamaEmbeddings
from langchain_text_splitters import RecursiveCharacterTextSplitter
from uuid import uuid4

def ingest_documents():
    """Ingest PDF documents into the vector database."""
    print("Starting document ingestion...")
    
    # Initialize embeddings model
    embeddings = OllamaEmbeddings(model="llama3.1:8b")
    
    # Configuration
    DATA_PATH = r"../data"
    PERSIST_DIRECTORY = r"../chromadb"
    
    # Initialize vector store
    vector_store = Chroma(
        collection_name="my_docs",
        embedding_function=embeddings,
        persist_directory=PERSIST_DIRECTORY
    )
    
    # Load documents from PDF directory
    print(f"Loading PDFs from {DATA_PATH}...")
    loader = PyPDFDirectoryLoader(DATA_PATH)
    raw_documents = loader.load()
    print(f"Loaded {len(raw_documents)} documents.")
    
    # Split documents into chunks
    text_splitter = RecursiveCharacterTextSplitter(
        chunk_size=300,
        chunk_overlap=100,
        length_function=len,
        is_separator_regex=False,
    )
    
    chunks = text_splitter.split_documents(raw_documents)
    print(f"Split into {len(chunks)} chunks.")
    
    # Generate unique IDs for each chunk
    uuids = [str(uuid4()) for _ in range(len(chunks))]
    
    # Add documents to vector store
    print("Adding documents to vector store...")
    vector_store.add_documents(chunks, ids=uuids)
    print(f"✓ Successfully ingested {len(chunks)} chunks into ChromaDB!")

if __name__ == "__main__":
    ingest_documents()
