#!/usr/bin/env bash
# Restore the last snapshotted desired state / image pin and redeploy.
# Git revert of the deployment repo is the durable rollback; this is the
# operator fast path using deploy.sh's .previous/ copies.
set -euo pipefail

# shellcheck source=lib.sh
. "$(cd "$(dirname "$0")" && pwd)/lib.sh"

ENV_NAME="${1:-main-lab}"
MODE="${2:-compose}"
DIR="$(env_dir "${ENV_NAME}")"
ROOT="$(deploy_root)"
PREV="${DIR}/.previous"

if [ ! -f "${PREV}/dns.yaml" ] || [ ! -f "${PREV}/image.env" ]; then
	echo "no snapshot at ${PREV}; revert the Git commit and run deploy.sh" >&2
	exit 1
fi

cp -f "${PREV}/dns.yaml" "${DIR}/dns.yaml"
cp -f "${PREV}/image.env" "${DIR}/image.env"
echo "restored ${ENV_NAME} from ${PREV}" >&2
"${ROOT}/scripts/deploy.sh" "${ENV_NAME}" "${MODE}"
