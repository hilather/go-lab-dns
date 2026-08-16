#!/usr/bin/env bash
# Health + offline semantic probes + optional wire queries against a live instance.
set -euo pipefail

# shellcheck source=lib.sh
. "$(cd "$(dirname "$0")" && pwd)/lib.sh"

ENV_NAME="${1:-main-lab}"
SERVER="${2:-127.0.0.1:53}"
HEALTH_URL="${3:-http://127.0.0.1:8080/v1/health/ready}"
DIR="$(env_dir "${ENV_NAME}")"
ROOT="$(deploy_root)"
BIN="$(labdns_bin)"

"${BIN}" healthcheck --url "${HEALTH_URL}"
"${BIN}" verify \
	--config "${DIR}/dns.yaml" \
	--probes "${DIR}/probes.yaml" \
	--policies "${ROOT}/policies" \
	--image-env "${DIR}/image.env" \
	--server "${SERVER}"
echo "live-probe ${ENV_NAME} ok" >&2
