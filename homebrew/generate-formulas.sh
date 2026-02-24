#!/bin/bash
# Generate Homebrew formulas for all Stockyard products.
# Usage: ./generate-formulas.sh v0.1.0
#
# Run after GoReleaser creates a release. It downloads the checksums file
# and generates Ruby formula files for the homebrew-tap repo.

set -euo pipefail

VERSION="${1:?Usage: $0 <version, e.g. v0.1.0>}"
VERSION_BARE="${VERSION#v}"
REPO="stockyard/stockyard"
BASE_URL="https://github.com/${REPO}/releases/download/${VERSION}"
OUT_DIR="${2:-Formula}"

mkdir -p "$OUT_DIR"

# Product metadata: name|class_name|description|port
PRODUCTS=(
  # Original 7
  "costcap|Costcap|Never get a surprise LLM bill again|4100"
  "llmcache|Llmcache|Cut your LLM costs by 30%+ with one line|4200"
  "jsonguard|Jsonguard|Guarantee valid JSON from any LLM|4300"
  "routefall|Routefall|Automatic LLM failover that just works|4400"
  "rateshield|Rateshield|Protect your LLM endpoints from abuse|4500"
  "promptreplay|Promptreplay|Record and replay every LLM call|4600"
  # Phase 1 Expansion
  "keypool|Keypool|Pool your API keys, multiply your limits|4700"
  "promptguard|Promptguard|PII never hits the LLM|4800"
  "modelswitch|Modelswitch|Right model, right prompt, right price|4900"
  "evalgate|Evalgate|Only ship quality LLM responses|4110"
  "usagepulse|Usagepulse|Know exactly where every token goes|4410"
  # Phase 2 Expansion
  "promptpad|Promptpad|Version control for your prompts|4801"
  "tokentrim|Tokentrim|Never hit a context limit again|4901"
  "batchqueue|Batchqueue|Background jobs for LLM calls|5000"
  "multicall|Multicall|Ask multiple models, pick the best answer|5100"
  "streamsnap|Streamsnap|Capture and replay every LLM stream|5200"
  "llmtap|Llmtap|Full-stack LLM analytics in one binary|5300"
  "contextpack|Contextpack|RAG without the vector database|5400"
  "retrypilot|Retrypilot|Intelligent retries that actually work|5500"
  # Phase 3 P1 Expansion
  "toxicfilter|Toxicfilter|Content moderation middleware for LLM outputs|5600"
  "compliancelog|Compliancelog|Immutable audit trail for every LLM interaction|5610"
  "secretscan|Secretscan|Catch API keys and secrets leaking in LLM traffic|5620"
  "tracelink|Tracelink|Distributed tracing for multi-step LLM chains|5630"
  "alertpulse|Alertpulse|PagerDuty for your LLM stack|5640"
  "chatmem|Chatmem|Persistent conversation memory without eating context window|5650"
  "mockllm|Mockllm|Deterministic LLM responses for testing and CI/CD|5660"
  "tenantwall|Tenantwall|Per-tenant isolation for multi-tenant LLM apps|5670"
  "idlekill|Idlekill|Kill runaway LLM requests burning money doing nothing|5680"
  "ipfence|Ipfence|IP allowlisting and geofencing for your LLM endpoints|5690"
  "embedcache|Embedcache|Never compute the same embedding twice|5700"
  "anthrofit|Anthrofit|Use Claude with OpenAI SDKs zero code changes|5710"
  # Full Suite
  "stockyard|Llmkit|The complete LLM proxy toolkit|4000"
)

# Download checksums
echo "Downloading checksums..."
CHECKSUMS=$(curl -fsSL "${BASE_URL}/checksums.txt")

get_sha() {
  local filename="$1"
  echo "$CHECKSUMS" | grep "$filename" | awk '{print $1}'
}

for product_info in "${PRODUCTS[@]}"; do
  IFS='|' read -r name class desc port <<< "$product_info"

  sha_darwin_amd64=$(get_sha "${name}_darwin_amd64.tar.gz")
  sha_darwin_arm64=$(get_sha "${name}_darwin_arm64.tar.gz")
  sha_linux_amd64=$(get_sha "${name}_linux_amd64.tar.gz")
  sha_linux_arm64=$(get_sha "${name}_linux_arm64.tar.gz")

  cat > "${OUT_DIR}/${name}.rb" <<RUBY
class ${class} < Formula
  desc "${desc}"
  homepage "https://github.com/${REPO}"
  version "${VERSION_BARE}"
  license "MIT"

  on_macos do
    if Hardware::CPU.arm?
      url "${BASE_URL}/${name}_darwin_arm64.tar.gz"
      sha256 "${sha_darwin_arm64}"
    else
      url "${BASE_URL}/${name}_darwin_amd64.tar.gz"
      sha256 "${sha_darwin_amd64}"
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url "${BASE_URL}/${name}_linux_arm64.tar.gz"
      sha256 "${sha_linux_arm64}"
    else
      url "${BASE_URL}/${name}_linux_amd64.tar.gz"
      sha256 "${sha_linux_amd64}"
    end
  end

  def install
    bin.install "${name}"
  end

  test do
    assert_match "ok", shell_output("#{bin}/${name} --health 2>&1", 1)
  end
end
RUBY

  echo "✓ Generated ${OUT_DIR}/${name}.rb"
done

echo ""
echo "Done! Copy ${OUT_DIR}/ to the homebrew-tap repo and push."
