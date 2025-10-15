
from langchain.document_loaders import PyPDFDirectoryLoader
from langchain_chroma import Chroma
from langchain_ollama import OllamaEmbeddings, OllamaLLM
from langchain_text_splitters import RecursiveCharacterTextSplitter
from uuid import uuid4

def main():
    print("Hello from ai!")
    embeddings = OllamaEmbeddings(model="llama3.1:8b")
    try:
        result = embeddings.embed_query("What is the meaning of life?")
        print("-----EMBEDDING-----")
        print(result)
    except Exception as e:
        print(f"Error generating embedding: {e}")

    # llm = OllamaLLM(model="llama3.1:8b")
    # try:
    #     result2 = llm.invoke("What is the meaning of life?")
    #     print("-----PROMPT RESULT-----")
    #     print(result2)
    # except Exception as e:
    #     print(f"Error generating llm chat result: {e}")
    DATA_PATH =r"chromadb"
    vector_store = Chroma(
        collection_name="my_docs",
        embedding_function=embeddings,
        persist_directory=r"chromadb"
    )

    loader = PyPDFDirectoryLoader(DATA_PATH)
    raw_documents = loader.load()

    text_splitter = RecursiveCharacterTextSplitter(
        chunk_size=300,
        chunk_overlap=100,
        length_function=len,
        is_separator_regex=False,
    )

    chunks = text_splitter.split_documents(raw_documents)
    print(f"Splitted into {len(chunks)} chunks.")

    uuids = [str(uuid4() ) for _ in range(len(chunks))]

    vector_store.add_documents(chunks, ids=uuids)


if __name__ == "__main__":
    main()
