"""
Test script to verify the agent setup is working correctly.
"""

print("Testing imports...")

try:
    from langchain_chroma import Chroma
    print("✓ langchain_chroma imported successfully")
except ImportError as e:
    print(f"✗ Error importing langchain_chroma: {e}")

try:
    from langchain_ollama import OllamaEmbeddings, OllamaLLM
    print("✓ langchain_ollama imported successfully")
except ImportError as e:
    print(f"✗ Error importing langchain_ollama: {e}")

try:
    from langchain_core.tools import tool
    print("✓ langchain_core.tools imported successfully")
except ImportError as e:
    print(f"✗ Error importing langchain_core.tools: {e}")

try:
    from langgraph.prebuilt import create_react_agent
    print("✓ langgraph.prebuilt imported successfully")
except ImportError as e:
    print(f"✗ Error importing langgraph.prebuilt: {e}")

try:
    from langgraph.checkpoint.memory import MemorySaver
    print("✓ langgraph.checkpoint.memory imported successfully")
except ImportError as e:
    print(f"✗ Error importing langgraph.checkpoint.memory: {e}")

try:
    import gradio as gr
    print("✓ gradio imported successfully")
except ImportError as e:
    print(f"✗ Error importing gradio: {e}")

print("\n" + "="*50)
print("Testing tool definition...")

try:
    @tool
    def test_tool(query: str) -> str:
        """A test tool."""
        return f"Received: {query}"
    
    print("✓ Tool definition works")
    print(f"  Tool name: {test_tool.name}")
    print(f"  Tool description: {test_tool.description}")
except Exception as e:
    print(f"✗ Error creating tool: {e}")

print("\n" + "="*50)
print("All tests completed!")
