from setuptools import setup, find_packages

setup(
    name="stockyard-openai",
    version="1.0.0",
    description="Stockyard OpenAI SDK wrapper",
    packages=find_packages(),
    install_requires=["openai>=1.0.0"],
    python_requires=">=3.8",
)
