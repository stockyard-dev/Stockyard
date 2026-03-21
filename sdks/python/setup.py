from setuptools import setup, find_packages

setup(
    name="stockyard-openai",
    version="0.1.0",
    description="Drop-in OpenAI SDK wrapper that routes through Stockyard",
    long_description=open("README.md").read(),
    long_description_content_type="text/markdown",
    author="Stockyard",
    url="https://stockyard.dev",
    packages=find_packages(),
    install_requires=["openai>=1.0.0"],
    python_requires=">=3.8",
    classifiers=[
        "Programming Language :: Python :: 3",
        "License :: OSI Approved :: MIT License",
    ],
)
