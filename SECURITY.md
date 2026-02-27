# Security Policy

## Reporting a Vulnerability

If you discover a security vulnerability in Stockyard, please report it responsibly.

**Email:** security@stockyard.dev

Please include:
- Description of the vulnerability
- Steps to reproduce
- Impact assessment
- Suggested fix (if any)

We will acknowledge receipt within 48 hours and aim to provide a fix within 7 days for critical issues.

## Scope

Security issues in the following are in scope:
- The Stockyard binary and all embedded apps
- Authentication and authorization (admin keys, user API keys, BYOK)
- The Trust audit ledger (hash chain integrity)
- Provider key storage and handling
- The proxy middleware chain
- SQLite database access controls

## Not in Scope

- Vulnerabilities in upstream LLM providers (OpenAI, Anthropic, etc.)
- Denial of service attacks against the proxy (rate limiting is a feature, not a bug)
- Issues requiring physical access to the host machine
- Social engineering

## Security Features

Stockyard includes several security-focused features:
- **Trust app:** Hash-chained tamper-evident audit ledger
- **PromptGuard:** Prompt injection detection
- **SecretScan:** API key and PII detection in prompts
- **ToxicFilter:** Content moderation
- **AgentGuard:** Agentic workflow safety controls
- **IPFence:** IP-based access control
- **TenantWall:** Multi-tenant isolation
- **EncryptVault:** At-rest encryption for sensitive data
