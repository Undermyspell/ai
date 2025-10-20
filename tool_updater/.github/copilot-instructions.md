# Project Overview

An agentic application which lets the user provide a list of tools he usually installs on his linux machine. Each tool out there does have its own documentation in how to install and update it. The agentic application will read the documentation of each tool and generate instructions on how to install and update each tool. The instructions should be summarized in a single markdown file which the user can use to install and update his tools in the future. The agent should be able to get the necessary documentation from the web in how to install and update each tool. 
A second use case is that the user can request that the agent searches for updates for his tools and updates the markdown file with the new instructions on how to update each tool including highlighting the changes made to the instructions and new versions which are available as well as breaking changes.

## Libraries and Frameworks
- LangGraph and LangChain for building the agentic application.
- Web scraping libraries such as BeautifulSoup, Scrapy or tavily_search to extract installation and update instructions from tools.
- Markdown generation libraries such as Markdown or Mistune to create the summarized markdown file.
- Methods, functions, classes etc. should be type safe preferably using Pydantic.
- Gradio or Streamlit for building a simple user interface to interact with the agentic application.

## Miscellaneous
- The code should have comments for major blocks what they do and why the code is needed. But don't be too verbose in the comments.
- Do not do more than the user requests in prompts. You can provide hints what should or could be done in the output.
- We should call the tools which the user installs on his linux machine "user tools" not just "tools" as the term "tools" is reserved for llm tools in the context of langchain/langgraph.