from setuptools import setup, find_packages

setup(
    name="langchain-llmkit",
    version="0.1.0",
    description="LangChain integration for LLMKit — LLM cost caps, caching, rate limiting, and analytics",
    long_description=open("README.md").read(),
    long_description_content_type="text/markdown",
    author="LLMKit",
    author_email="hello@llmkit.dev",
    url="https://github.com/llmkit/langchain-llmkit",
    packages=find_packages(),
    python_requires=">=3.9",
    install_requires=[
        "langchain-core>=0.1.0",
        "requests>=2.28.0",
        "pydantic>=2.0.0",
    ],
    classifiers=[
        "Development Status :: 3 - Alpha",
        "Intended Audience :: Developers",
        "License :: OSI Approved :: MIT License",
        "Programming Language :: Python :: 3",
        "Topic :: Software Development :: Libraries",
    ],
    keywords=["langchain", "llm", "proxy", "openai", "cost", "cache", "llmkit"],
    license="MIT",
)
