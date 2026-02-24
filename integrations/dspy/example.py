import dspy

# DSPy makes thousands of LLM calls during optimization.
# Stockyard caching + cost caps are essential.
lm = dspy.LM(
    model="openai/gpt-4o-mini",
    api_base="http://localhost:4000/v1",
    api_key="any-string",
)
dspy.configure(lm=lm)
