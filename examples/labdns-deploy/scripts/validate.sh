#!/usr/bin/env bash
# Schema + policy + digest pin. Failures print diagnostics and cannot be skipped.
set -euo pipefail

# shellcheck source=lib.sh
. "$(cd "$(dirname "$0")" && pwd)/lib.sh"

ENV_NAME="${1:-main-lab}"
DIR="$(env_dir "${ENV_NAME}")"
ROOT="$(deploy_root)"
BIN="$(labdns_bin)"

echo "validate ${ENV_NAME} config=${DIR}/dns.yaml" >&2
"${BIN}" validate --config "${DIR}/dns.yaml"
"${BIN}" verify \
	--config "${DIR}/dns.yaml" \
	--probes "${DIR}/probes.yaml" \
	--policies "${ROOT}/policies" \
	--image-env "${DIR}/image.env"
echo "validate ok" >&2
