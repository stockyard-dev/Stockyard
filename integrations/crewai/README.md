# crewai-stockyard

> CrewAI integration for Stockyard proxy. Save 30-50% on agent token costs.

```bash
pip install crewai-stockyard
```

```python
from crewai import Agent, Task, Crew
from crewai_stockyard import StockyardLLM

llm = StockyardLLM(model="gpt-4o-mini")

researcher = Agent(role="Researcher", goal="Find info", llm=llm)
writer = Agent(role="Writer", goal="Write content", llm=llm)
```

CrewAI multi-agent workflows are the highest token consumers. Stockyard's caching saves 30-50% on repeated tool calls.

## License
MIT
