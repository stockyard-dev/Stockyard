from setuptools import setup

setup(
    name="stockyard",
    version="0.1.0",
    description="LLM infrastructure proxy — 125 tools in one binary",
    author="Stockyard",
    url="https://stockyard.dev",
    py_modules=["stockyard"],
    entry_points={"console_scripts": ["stockyard=stockyard:main"]},
    python_requires=">=3.8",
)
