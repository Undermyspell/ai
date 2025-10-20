from typing import TypedDict, Annotated, Sequence
from langchain_core.messages import BaseMessage, HumanMessage
from langchain_core.tools import tool
from langgraph.graph import StateGraph, END
from langgraph.prebuilt import ToolNode
import operator


# LLM Tool: Search for user tool documentation
@tool
def search_user_tool_documentation(
    user_tool_name: Annotated[str, "Name of the user tool to search documentation for"],
    query_type: Annotated[str, "Type of query: 'install' or 'update'"]
) -> str:
    """
    Search for installation or update documentation for a given user tool.
    This tool will be used by the LLM agent to find relevant documentation.
    Can be extracted to an MCP server in the future.
    """
    # TODO: Implement actual web search using Tavily or similar
    # For now, return mock data
    return f"""
# Documentation for {user_tool_name} ({query_type})

## Installation Steps
1. Download {user_tool_name} from official website
2. Extract the archive
3. Run the installer
4. Verify installation with: {user_tool_name} --version

## Update Steps
1. Check current version: {user_tool_name} --version
2. Download latest version
3. Run update command
4. Verify update was successful

Version: 1.0.0
Source: https://example.com/{user_tool_name}
"""


# Define the agent state
class AgentState(TypedDict):
    """State that will be passed between nodes in the graph"""
    messages: Annotated[Sequence[BaseMessage], operator.add]
    user_tools_to_process: list[str]  # List of user tool names to gather docs for
    processed_user_tools: dict[str, dict]  # user_tool_name -> {install_steps, update_steps, version}
    final_markdown: str  # The compiled markdown output


# Node: Agent decides what to do next
def agent_node(state: AgentState) -> AgentState:
    """
    Main agent node that decides what action to take.
    Later this will use an LLM to decide whether to:
    - Search for user tool documentation
    - Compile the final markdown
    - End the workflow
    """
    print("Agent node - deciding next action...")
    
    # For now, just return the state
    return state


# Node: Execute LLM tools to search for user tool documentation
def llm_tool_node(state: AgentState) -> AgentState:
    """
    Execute LLM tools to fetch documentation for user tools.
    Processes one user tool at a time by calling the search_user_tool_documentation tool.
    """
    print("LLM Tool node - executing tools...")
    
    # Process one user tool at a time
    if state["user_tools_to_process"]:
        current_user_tool = state["user_tools_to_process"][0]
        print(f"Processing user tool: {current_user_tool}")
        
        # Call the LLM tool to search for installation documentation
        install_docs = search_user_tool_documentation.invoke({
            "user_tool_name": current_user_tool,
            "query_type": "install"
        })
        
        # Call the LLM tool to search for update documentation
        update_docs = search_user_tool_documentation.invoke({
            "user_tool_name": current_user_tool,
            "query_type": "update"
        })
        
        # Store the processed documentation
        state["processed_user_tools"][current_user_tool] = {
            "install_steps": install_docs,
            "update_steps": update_docs,
            "version": "latest"  # TODO: Extract version from docs
        }
        
        # Remove processed user tool from queue
        state["user_tools_to_process"] = state["user_tools_to_process"][1:]
    
    return state


# Node: Compile final markdown
def compile_markdown_node(state: AgentState) -> AgentState:
    """
    Compile all processed user tool information into a single markdown file.
    """
    print("Compiling markdown...")
    
    markdown = "# User Tools Installation and Update Guide\n\n"
    
    for user_tool_name, info in state["processed_user_tools"].items():
        markdown += f"## {user_tool_name}\n\n"
        markdown += f"**Version:** {info['version']}\n\n"
        markdown += f"### Installation\n{info['install_steps']}\n\n"
        markdown += f"### Update\n{info['update_steps']}\n\n"
    
    state["final_markdown"] = markdown
    return state


# Conditional edge: decide where to go next
def should_continue(state: AgentState) -> str:
    """
    Determine the next step in the workflow.
    - If there are user tools to process, go to LLM tools node
    - If all user tools processed, compile markdown
    - Otherwise, end
    """
    if state["user_tools_to_process"]:
        return "llm_tools"
    elif state["processed_user_tools"] and not state["final_markdown"]:
        return "compile"
    else:
        return "end"


# Create the graph
def create_graph():
    """Build and return the LangGraph workflow"""
    workflow = StateGraph(AgentState)
    
    # Add nodes
    workflow.add_node("agent", agent_node)
    workflow.add_node("llm_tools", llm_tool_node)
    workflow.add_node("compile_markdown", compile_markdown_node)
    
    # Set entry point
    workflow.set_entry_point("agent")
    
    # Add edges
    workflow.add_conditional_edges(
        "agent",
        should_continue,
        {
            "llm_tools": "llm_tools",
            "compile": "compile_markdown",
            "end": END
        }
    )
    
    # After executing LLM tools, go back to agent to decide next step
    workflow.add_edge("llm_tools", "agent")
    
    # After compiling markdown, we're done
    workflow.add_edge("compile_markdown", END)
    
    return workflow.compile()


def main():
    """Main entry point"""
    print("User Tool Updater - LangGraph Agent\n")
    
    # Create the graph
    graph = create_graph()
    
    # Initialize state with some example user tools
    initial_state = {
        "messages": [HumanMessage(content="Get installation docs for nodejs, kubectl, and syft")],
        "user_tools_to_process": ["nodejs", "kubectl", "syft"],
        "processed_user_tools": {},
        "final_markdown": ""
    }
    
    # Run the graph
    print("Running agent...\n")
    final_state = graph.invoke(initial_state)
    
    # Print results
    print("\n" + "="*60)
    print("Final Markdown Output:")
    print("="*60)
    print(final_state["final_markdown"])


if __name__ == "__main__":
    main()
