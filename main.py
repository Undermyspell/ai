
from langchain_ollama import OllamaEmbeddings, OllamaLLM

def main():
    print("Hello from ai!")
    embeddings = OllamaEmbeddings(model="llama3.1:8b")
    try:
        result = embeddings.embed_query("What is the meaning of life?")
        print("-----EMBEDDING-----")
        print(result)
    except Exception as e:
        print(f"Error generating embedding: {e}")

    llm = OllamaLLM(model="llama3.1:8b")
    try:
        result2 = llm.invoke("What is the meaning of life?")
        print("-----PROMPT RESULT-----")
        print(result2)
    except Exception as e:
        print(f"Error generating llm chat result: {e}")

if __name__ == "__main__":
    main()
