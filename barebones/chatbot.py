

import gradio as gr
import os
from langchain_chroma import Chroma
from langchain_ollama import OllamaEmbeddings, OllamaLLM

# Get absolute paths based on script location
script_dir = os.path.dirname(os.path.abspath(__file__))
project_root = os.path.dirname(script_dir)
PERSIST_DIRECTORY = os.path.join(project_root, "chromadb")

embedding_model = OllamaEmbeddings(model="llama3.1:8b")
llm = OllamaLLM(model="llama3.1:8b", temperature=0.5)    

vector_store = Chroma(
    collection_name="my_docs", 
    embedding_function=embedding_model, 
    persist_directory=PERSIST_DIRECTORY
)

num_results = 5
retriever = vector_store.as_retriever(search_kwargs={"k": num_results})

def stream_response(message, history):
    docs = retriever.invoke(message) 

    knowledge = ""

    for doc in docs:
        knowledge += doc.page_content + "\n"

    if message is not None: 
        partial_message = ""

        rag_prompt = f"""
        You are an assistant which answers questions based on knowledge isprovided to you.
        While answering, you don't use your internal knowledge,
        but solely the information in the "The knowledge" section below.
        You dont' mention anything to the user about the provided knowledge.

        The question: {message}

        Conversation history: {history}

        The knowledge: {knowledge}
        """

        for response in llm.stream(rag_prompt):
            partial_message += response
            yield partial_message 

chatbot = gr.ChatInterface(stream_response, 
            textbox=gr.Textbox(placeholder="Enter your message here...",  
                               container=False,
                               autoscroll=True,
                               scale=7),
            title="AI Chatbot with RAG",
            description="Ask questions based on the ingested documents.",
       )


chatbot.launch()
