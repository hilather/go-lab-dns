#!/usr/bin/env bash
# Recreate the environment from Git desired state. Runtime drift is discarded.
set -euo pipefail

# shellcheck source=lib.sh
. "$(cd "$(dirname "$0")" && pwd)/lib.sh"

ENV_NAME="${1:-main-lab}"
MODE="${2:-compose}"
DIR="$(env_dir "${ENV_NAME}")"
ROOT="$(deploy_root)"

"${ROOT}/scripts/validate.sh" "${ENV_NAME}"
load_image_env "${DIR}/image.env"
rotate_deploy_snapshot "${DIR}"

case "${MODE}" in
compose)
	require_cmd docker
	if [ ! -f "${LABDNS_TOKEN_FILE:-${ROOT}/secrets/labdns-token}" ]; then
		echo "missing bearer token file (set LABDNS_TOKEN_FILE or create secrets/labdns-token)" >&2
		exit 1
	fi
	export LABDNS_IMAGE
	(cd "${DIR}" && docker compose --env-file image.env -f compose.yaml up -d --force-recreate)
	;;
k8s|kubernetes)
	require_cmd kubectl
	if [ ! -d "${DIR}/k8s" ]; then
		echo "${ENV_NAME} has no k8s/ directory" >&2
		exit 1
	fi
	kubectl apply -k "${DIR}/k8s"
	# Recreate semantics: a new ReplicaSet discards process-local drift.
	kubectl rollout status "deployment/labdns" -n "labdns-${ENV_NAME}" --timeout=120s
	;;
*)
	echo "usage: deploy.sh <env> [compose|k8s]" >&2
	exit 2
	;;
esac

record_deploy_snapshot "${DIR}"
echo "deploy ${ENV_NAME} ${MODE} ok (container recreation resets runtime drift)" >&2
