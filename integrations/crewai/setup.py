from setuptools import setup, find_packages

setup(
    name="crewai-llmkit",
    version="0.1.0",
    description="CrewAI integration for LLMKit — cost caps, caching, and analytics for AI agents",
    long_description=open("README.md").read() if __import__("os").path.exists("README.md") else "",
    long_description_content_type="text/markdown",
    author="LLMKit",
    url="https://github.com/llmkit/crewai-llmkit",
    packages=find_packages(),
    python_requires=">=3.9",
    install_requires=["langchain-core>=0.1.0", "requests>=2.28.0", "pydantic>=2.0.0"],
    keywords=["crewai", "llm", "proxy", "agents", "cost", "cache", "llmkit"],
    license="MIT",
)
