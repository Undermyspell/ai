from typing import Annotated, Sequence
from langchain_core.messages import BaseMessage, HumanMessage
from langchain_core.tools import tool
from langgraph.graph import StateGraph, END
from langgraph.prebuilt import ToolNode
from dotenv import load_dotenv
from langchain_community.tools.tavily_search import TavilySearchResults
from langchain_ollama import ChatOllama
from pydantic import BaseModel, Field
import operator
import json

load_dotenv()


# Pydantic models for type safety
class SearchResult(BaseModel):
    """Search result with URL and title"""
    url: str
    title: str


class UserToolInfo(BaseModel):
    """Processed information for a user tool"""
    install_steps: list[str] = Field(default_factory=list)
    update_steps: list[str] = Field(default_factory=list)
    version: str = "unknown"
    requirements: list[str] = Field(default_factory=list)
    notes: str = ""


class AgentState(BaseModel):
    """State that will be passed between nodes in the graph"""
    messages: Annotated[Sequence[BaseMessage], operator.add]
    user_tools_to_process: list[str] = Field(default_factory=list)
    current_user_tool: str = ""
    search_results: list[SearchResult] = Field(default_factory=list)
    scraped_content: str = ""
    processed_user_tools: dict[str, UserToolInfo] = Field(default_factory=dict)
    final_markdown: str = ""
    
    model_config = {
        "arbitrary_types_allowed": True,  # Allow BaseMessage types
        "validate_assignment": True  # Validate on assignment - catches type errors at runtime
    }

# LLM Tool: Search for user tool documentation
@tool
def search_user_tool_documentation(
    user_tool_name: Annotated[str, "Name of the user tool to search documentation for"]
) -> dict:
    """
    Search for installation or update documentation for a given user tool.
    Returns a list of URLs with titles from the search results.
    Can be extracted to an MCP server in the future.
    """
    query = f"Install {user_tool_name} on linux"
    # Initialize Tavily with content included
    # Exclude video platforms and social media that don't provide useful documentation
    search = TavilySearchResults(
        max_results=3,
        include_raw_content=False,
        search_depth="advanced",
        exclude_domains=[
            "youtube.com",
            "youtu.be",
            "vimeo.com",
            "dailymotion.com",
            "tiktok.com",
            "facebook.com",
            "twitter.com",
            "instagram.com",
            "x.com"
        ]
    )
    try:
        print(f"✓ Travily search for tool: {user_tool_name}")
        results = search.invoke(query)
        search_results = [
            {
                "url": r["url"],
                "title": r.get("title", ""),
            }
            for r in results
        ]
        print(f"✓ Found {len(results)} results with content for {user_tool_name}")
        return search_results
    except Exception as e:
        print(f"✗ Search failed for {user_tool_name}: {e}")
        return search_results

# LLM Tool: Scrape documentation page
@tool
def scrape_documentation_page(
    url: Annotated[str, "URL of the documentation page to scrape"]
) -> str:
    """
    Scrape content from a documentation page.
    Uses BeautifulSoup to extract text content from HTML.
    Can be extracted to an MCP server in the future.
    """
    import requests
    from bs4 import BeautifulSoup
    
    try:
        print(f"✓ Scraping URL: {url}")
        
        # Fetch the page with a timeout
        headers = {
            'User-Agent': 'Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36'
        }
        response = requests.get(url, headers=headers, timeout=10)
        response.raise_for_status()
        
        # Parse HTML with BeautifulSoup
        soup = BeautifulSoup(response.content, 'html.parser')
        
        # Remove script, style, nav, footer, and other non-content elements
        for element in soup(['script', 'style', 'nav', 'footer', 'header', 'aside', 'iframe']):
            element.decompose()
        
        # Try to find main content areas (common patterns in documentation sites)
        main_content = None
        
        # Try different selectors for main content
        content_selectors = [
            'main',
            'article',
            '[role="main"]',
            '.main-content',
            '.content',
            '#content',
            '.documentation',
            '.doc-content'
        ]
        
        for selector in content_selectors:
            main_content = soup.select_one(selector)
            if main_content:
                break
        
        # If no main content found, use body
        if not main_content:
            main_content = soup.body
        
        # Extract text
        if main_content:
            # Get text with some formatting preserved
            text = main_content.get_text(separator='\n', strip=True)
            
            # Clean up excessive whitespace
            lines = [line.strip() for line in text.split('\n') if line.strip()]
            cleaned_text = '\n'.join(lines)
            
            print(f"✓ Scraped {len(cleaned_text)} characters from {url}")
            return cleaned_text
        else:
            print(f"✗ No content found at {url}")
            return ""
            
    except requests.Timeout:
        print(f"✗ Timeout while scraping {url}")
        return ""
    except requests.RequestException as e:
        print(f"✗ Error scraping {url}: {e}")
        return ""
    except Exception as e:
        print(f"✗ Unexpected error scraping {url}: {e}")
        return ""

# LLM Tool: Extract structured installation and update steps
@tool
def extract_installation_steps(
    user_tool_name: Annotated[str, "Name of the user tool"],
    scraped_content: Annotated[str, "Raw scraped documentation content"]
) -> dict:
    """
    Extract structured installation and update steps from scraped documentation.
    Uses an LLM to parse and structure both installation and update information.
    Can be extracted to an MCP server in the future.
    """
    # Initialize Ollama LLM
    llm = ChatOllama(
        model="llama3.1:8b",
        temperature=0.1  # Low temperature for more consistent extraction
    )
    
    # Create extraction prompt
    prompt = f"""You are an expert at extracting installation and update instructions from technical documentation.
                Given the following documentation content for {user_tool_name}, extract structured information about both installation AND update instructions.
                Documentation content:
                {scraped_content[:8000]}  

                Please analyze the content and extract:
                1. Step-by-step INSTALLATION instructions (as a list)
                2. Step-by-step UPDATE instructions (as a list) - if not available, provide reasonable update steps
                3. Version information (if mentioned)
                4. System requirements (as a list)
                5. Important notes or warnings

                Respond ONLY with a valid JSON object in this exact format:
                {{
                    "install_steps": ["install step 1", "install step 2", ...],
                    "update_steps": ["update step 1", "update step 2", ...],
                    "version": "version number or 'unknown'",
                    "requirements": ["requirement 1", "requirement 2", ...],
                    "notes": "any important notes or warnings"
                }}

                Do not include any other text outside the JSON object."""

    try:
        print(f"✓ Extracting installation and update information using LLM for {user_tool_name}...")
        
        # Invoke the LLM
        response = llm.invoke(prompt)
        response_text = response.content.strip()
        
        # Try to parse JSON from response
        # Sometimes LLMs wrap JSON in markdown code blocks
        if "```json" in response_text:
            response_text = response_text.split("```json")[1].split("```")[0].strip()
        elif "```" in response_text:
            response_text = response_text.split("```")[1].split("```")[0].strip()
        
        extracted_data = json.loads(response_text)
        
        # Validate structure
        result = {
            "user_tool_name": user_tool_name,
            "install_steps": extracted_data.get("install_steps", []),
            "update_steps": extracted_data.get("update_steps", []),
            "version": extracted_data.get("version", "unknown"),
            "requirements": extracted_data.get("requirements", []),
            "notes": extracted_data.get("notes", "")
        }
        
        print(f"✓ Successfully extracted {len(result['install_steps'])} install steps and {len(result['update_steps'])} update steps for {user_tool_name}")
        return result
        
    except json.JSONDecodeError as e:
        print(f"✗ Failed to parse LLM response as JSON: {e}")
        print(f"Response was: {response_text[:200]}...")
        # Return fallback structure
        return {
            "user_tool_name": user_tool_name,
            "install_steps": ["Failed to extract steps - please check documentation manually"],
            "update_steps": ["Failed to extract steps - please check documentation manually"],
            "version": "unknown",
            "requirements": [],
            "notes": "Extraction failed - manual review needed"
        }
    except Exception as e:
        print(f"✗ Error during LLM extraction: {e}")
        return {
            "user_tool_name": user_tool_name,
            "install_steps": ["Error during extraction"],
            "update_steps": ["Error during extraction"],
            "version": "unknown",
            "requirements": [],
            "notes": f"Error: {str(e)}"
        }


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
def search_node(state: AgentState) -> AgentState:
    """
    Search for documentation URLs for the current user tool.
    Calls the search_user_tool_documentation LLM tool.
    """
    print("Search node - finding documentation URLs...")
    
    if state.user_tools_to_process:
        current_user_tool = state.user_tools_to_process[0]
        state.current_user_tool = current_user_tool
        print(f"Searching for: {current_user_tool}")
        
        # Search for installation and update documentation
        search_results_raw = search_user_tool_documentation.invoke({
            "user_tool_name": current_user_tool
        })
        
        # Convert to Pydantic models
        state.search_results = [SearchResult(**result) for result in search_results_raw]
        print(f"Search results: {[r.model_dump() for r in state.search_results]}")
    
    return state


# Node: Scrape documentation from URLs
def scrape_node(state: AgentState) -> AgentState:
    """
    Scrape content from all documentation URLs found in search.
    Calls the scrape_documentation_page LLM tool for each URL and merges results.
    The merged content will be fed to the LLM for extraction.
    """
    print("Scrape node - extracting content from documentation pages...")
    
    if state.search_results:
        all_scraped_content = []
        
        # Scrape all URLs from search results
        for i, result in enumerate(state.search_results, 1):
            url = result.url
            title = result.title or "Unknown"
            
            if url:
                print(f"Scraping URL {i}/{len(state.search_results)}: {url}")
                
                scraped_content = scrape_documentation_page.invoke({
                    "url": url
                })
                
                if scraped_content:
                    # Add metadata to help LLM identify source
                    content_with_metadata = f"""
                                ===== SOURCE {i}: {title} =====
                                URL: {url}

                                {scraped_content}

                                ===== END OF SOURCE {i} =====
                                """
                    all_scraped_content.append(content_with_metadata)
        
        # Merge all scraped content
        if all_scraped_content:
            merged_content = "\n\n".join(all_scraped_content)
            state.scraped_content = merged_content
            print(f"✓ Successfully scraped and merged {len(all_scraped_content)} pages")
        else:
            print("✗ No content could be scraped from any URL")
            state.scraped_content = ""
    else:
        print("No search results available to scrape")
        state.scraped_content = ""
    
    return state


# Node: Extract structured information using LLM
def extract_node(state: AgentState) -> AgentState:
    """
    Extract structured installation/update steps from scraped content.
    Calls the extract_installation_steps LLM tool which uses Ollama LLM to parse content.
    """
    print("Extract node - parsing documentation with LLM...")
    
    if state.scraped_content and state.current_user_tool:
        current_user_tool = state.current_user_tool
        print(f"Extracting structured data for: {current_user_tool}")
        
        # Extract both installation and update steps in one LLM call
        extracted_info = extract_installation_steps.invoke({
            "user_tool_name": current_user_tool,
            "scraped_content": state.scraped_content
        })
        
        # Store the processed documentation as Pydantic model
        state.processed_user_tools[current_user_tool] = UserToolInfo(
            install_steps=extracted_info.get("install_steps", []),
            update_steps=extracted_info.get("update_steps", []),
            version=extracted_info.get("version", "unknown"),
            requirements=extracted_info.get("requirements", []),
            notes=extracted_info.get("notes", "")
        )
        
        # Remove processed user tool from queue
        state.user_tools_to_process = state.user_tools_to_process[1:]
        # Clear temporary state
        state.current_user_tool = ""
        state.search_results = 5
        state.scraped_content = ""
    else:
        print("✗ No scraped content available for extraction")
    
    return state


# Node: Compile final markdown
def compile_markdown_node(state: AgentState) -> AgentState:
    """
    Compile all processed user tool information into a single markdown file.
    Saves the markdown to ./results directory.
    """
    print("Compile markdown node - generating final documentation...")
    
    markdown = "# User Tools Installation and Update Guide\n\n"
    markdown += f"Generated on: {__import__('datetime').datetime.now().strftime('%Y-%m-%d %H:%M:%S')}\n\n"
    markdown += "---\n\n"
    
    # Generate markdown for each processed user tool
    for user_tool_name, info in state.processed_user_tools.items():
        markdown += f"## {user_tool_name}\n\n"
        
        # Version
        markdown += f"**Version:** {info.version}\n\n"
        
        # Requirements
        if info.requirements:
            markdown += "**Requirements:**\n"
            for req in info.requirements:
                markdown += f"- {req}\n"
            markdown += "\n"
        
        # Installation steps
        markdown += "### Installation\n\n"
        if info.install_steps:
            for i, step in enumerate(info.install_steps, 1):
                markdown += f"{i}. {step}\n"
            markdown += "\n"
        else:
            markdown += "*No installation steps available*\n\n"
        
        # Update steps
        markdown += "### Update\n\n"
        if info.update_steps:
            for i, step in enumerate(info.update_steps, 1):
                markdown += f"{i}. {step}\n"
            markdown += "\n"
        else:
            markdown += "*No update steps available*\n\n"
        
        # Notes
        if info.notes:
            markdown += f"**Notes:** {info.notes}\n\n"
        
        markdown += "---\n\n"
    
    state.final_markdown = markdown
    
    # Save to file in results directory
    import os
    from pathlib import Path
    
    results_dir = Path("./results")
    results_dir.mkdir(exist_ok=True)
    
    # Generate filename with timestamp
    timestamp = __import__('datetime').datetime.now().strftime('%Y%m%d_%H%M%S')
    filename = f"user_tools_guide_{timestamp}.md"
    filepath = results_dir / filename
    
    try:
        with open(filepath, 'w', encoding='utf-8') as f:
            f.write(markdown)
        print(f"✓ Markdown saved to: {filepath}")
    except Exception as e:
        print(f"✗ Failed to save markdown file: {e}")
    
    return state


# Conditional edge: decide where to go next
def should_continue(state: AgentState) -> str:
    """
    Determine the next step in the workflow.
    - If there are user tools to process, start with search
    - If all user tools processed, compile markdown
    - Otherwise, end
    """
    if state.user_tools_to_process:
        return "search"
    elif state.processed_user_tools and not state.final_markdown:
        return "compile"
    else:
        return "end"


# Create the graph
def create_graph():
    """Build and return the LangGraph workflow"""
    workflow = StateGraph(AgentState)
    
    # Add nodes
    workflow.add_node("agent", agent_node)
    workflow.add_node("search", search_node)
    workflow.add_node("scrape", scrape_node)
    workflow.add_node("extract", extract_node)
    workflow.add_node("compile_markdown", compile_markdown_node)
    
    # Set entry point
    workflow.set_entry_point("agent")
    
    # Add edges
    workflow.add_conditional_edges(
        "agent",
        should_continue,
        {
            "search": "search",
            "compile": "compile_markdown",
            "end": END
        }
    )
    
    # Linear flow: search -> scrape -> extract -> back to agent
    workflow.add_edge("search", "scrape")
    workflow.add_edge("scrape", "extract")
    workflow.add_edge("extract", "agent")
    
    # After compiling markdown, we're done
    workflow.add_edge("compile_markdown", END)
    
    return workflow.compile()


def main():
    """Main entry point"""
    print("User Tool Updater - LangGraph Agent\n")
    
    # Create the graph
    graph = create_graph()
    
    # Initialize state with Pydantic model
    initial_state = AgentState(
        messages=[HumanMessage(content="Get installation docs for nodejs")],
        user_tools_to_process=["nodejs", "k9s"]
    )
    
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
