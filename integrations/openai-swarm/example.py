from openai import OpenAI
from swarm import Swarm

# Point Swarm at Stockyard
client = OpenAI(base_url="http://localhost:4000/v1", api_key="any")
swarm = Swarm(client=client)

# All agent handoffs and tool calls route through Stockyard
# AgentGuard recommended for session-level cost caps
