#!/usr/bin/env bash
# Positive path plus required negative fixtures. No bypass for a failed check.
set -euo pipefail

# shellcheck source=lib.sh
. "$(cd "$(dirname "$0")" && pwd)/lib.sh"

ENV_NAME="${1:-main-lab}"
DIR="$(env_dir "${ENV_NAME}")"
ROOT="$(deploy_root)"
BIN="$(labdns_bin)"
NEG="${ROOT}/testdata/invalid"

echo "test-config ${ENV_NAME}" >&2
"${ROOT}/scripts/validate.sh" "${ENV_NAME}"

fail_closed() {
	local name="$1"
	shift
	echo "expect-fail ${name}" >&2
	if "$@"; then
		echo "NEGATIVE ${name} unexpectedly succeeded" >&2
		exit 1
	fi
}

fail_closed unpinned-image \
	"${BIN}" verify --config "${DIR}/dns.yaml" --probes "${DIR}/probes.yaml" \
	--image "ghcr.io/hilather/labdns:latest"

fail_closed tag-image \
	"${BIN}" verify --config "${DIR}/dns.yaml" --probes "${DIR}/probes.yaml" \
	--image-env "${NEG}/unpinned.image.env"

fail_closed broad-client \
	"${BIN}" verify --config "${NEG}/broad-client.yaml" --probes "${DIR}/probes.yaml" \
	--policies "${ROOT}/policies"

fail_closed bad-upstream \
	"${BIN}" verify --config "${NEG}/bad-upstream.yaml" --probes "${DIR}/probes.yaml" \
	--policies "${ROOT}/policies"

fail_closed unsafe-chaos \
	"${BIN}" verify --config "${NEG}/unsafe-chaos.yaml" --probes "${DIR}/probes.yaml" \
	--policies "${ROOT}/policies"

echo "test-config ok (positives passed, negatives rejected)" >&2
