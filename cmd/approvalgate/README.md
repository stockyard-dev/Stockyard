# ApprovalGate

**Human approval for prompt changes.**

ApprovalGate adds an approval workflow to PromptPad. Prompt changes go into pending state until an approver accepts or rejects.

## Quickstart

```bash
export OPENAI_API_KEY=sk-...
npx @stockyard/approvalgate

# Your app:   http://localhost:5850/v1/chat/completions
# Dashboard:  http://localhost:5850/ui
```

## What You Get

- Pending state for prompt changes
- Approver notification
- Approve/reject via dashboard
- Full audit trail
- Role-based approvers
- Integration with PromptPad

## Config

```yaml
# approvalgate.yaml
port: 5850
providers:
  openai:
    api_key: ${OPENAI_API_KEY}
approval:
  enabled: true
  approvers:
    - admin@example.com
  notify_webhook: ""
  auto_approve_after: 0  # 0 = never auto-approve
```

## Docker

```bash
docker run -p 5850:5850 -e OPENAI_API_KEY=sk-... stockyard/approvalgate
```

## Part of Stockyard

ApprovalGate is part of [Stockyard](https://stockyard.dev) — an open-source LLM proxy and control plane. MIT licensed.
