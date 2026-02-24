# autogen-stockyard

> AutoGen integration for Stockyard proxy.

```bash
pip install autogen-stockyard
```

```python
import autogen
from autogen_stockyard import stockyard_config

config_list = stockyard_config(model="gpt-4o-mini")

assistant = autogen.AssistantAgent("assistant", llm_config={"config_list": config_list})
user = autogen.UserProxyAgent("user", human_input_mode="NEVER")
user.initiate_chat(assistant, message="Hello!")
```

## License
MIT
