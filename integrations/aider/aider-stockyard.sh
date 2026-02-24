#!/bin/bash
# Run Aider through Stockyard proxy
export OPENAI_API_BASE=http://localhost:4000/v1
export OPENAI_API_KEY=${OPENAI_API_KEY:-any-string}
exec aider "$@"

