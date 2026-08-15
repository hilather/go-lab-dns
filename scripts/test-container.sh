#!/usr/bin/env bash
# Container contract for DEP-001. Requires Docker. Fail closed if the daemon
# is missing so this is a real check, not an unimplemented stub.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
IMAGE="${LABDNS_TEST_IMAGE:-ghcr.io/hilather/labdns:test}"
NAME="labdns-container-test-$$"
CFG="${ROOT}/testdata/container/config.yaml"

if ! command -v docker >/dev/null 2>&1; then
	echo "docker is required for make test-container" >&2
	exit 1
fi
if ! docker info >/dev/null 2>&1; then
	echo "docker daemon is not available for make test-container" >&2
	exit 1
fi

cleanup() {
	docker rm -f "${NAME}" >/dev/null 2>&1 || true
}
trap cleanup EXIT

echo "building ${IMAGE}"
docker build -t "${IMAGE}" "${ROOT}"

inspect_user="$(docker image inspect --format '{{.Config.User}}' "${IMAGE}")"
if [ "${inspect_user}" != "65532:65532" ]; then
	echo "image User=${inspect_user}, want 65532:65532" >&2
	exit 1
fi

licenses="$(docker image inspect --format '{{index .Config.Labels "org.opencontainers.image.licenses"}}' "${IMAGE}")"
if [ "${licenses}" != "Apache-2.0" ]; then
	echo "image license label=${licenses}, want Apache-2.0" >&2
	exit 1
fi

docker run -d --name "${NAME}" \
	--read-only \
	--cap-drop=ALL \
	--security-opt=no-new-privileges:true \
	--tmpfs /tmp:rw,noexec,nosuid,size=16m \
	-v "${CFG}:/etc/labdns/config.yaml:ro" \
	-p 127.0.0.1::5353/udp \
	-p 127.0.0.1::5353/tcp \
	-p 127.0.0.1::8080/tcp \
	"${IMAGE}"

pid="$(docker inspect --format '{{.State.Pid}}' "${NAME}")"
if [ ! -r "/proc/${pid}/status" ]; then
	echo "cannot read /proc/${pid}/status to verify non-root/no-caps" >&2
	exit 1
fi
uid="$(awk '/^Uid:/{print $2}' "/proc/${pid}/status")"
if [ "${uid}" != "65532" ]; then
	echo "runtime Uid=${uid}, want 65532" >&2
	exit 1
fi
capeff="$(awk '/^CapEff:/{print $2}' "/proc/${pid}/status")"
if [ "${capeff}" != "0000000000000000" ]; then
	echo "CapEff=${capeff}, want 0000000000000000 (no capabilities)" >&2
	exit 1
fi

mgmt_port="$(docker port "${NAME}" 8080/tcp | head -n1 | awk -F: '{print $NF}')"
udp_port="$(docker port "${NAME}" 5353/udp | head -n1 | awk -F: '{print $NF}')"
tcp_port="$(docker port "${NAME}" 5353/tcp | head -n1 | awk -F: '{print $NF}')"

ok=0
for _ in $(seq 1 40); do
	if curl -fsS "http://127.0.0.1:${mgmt_port}/v1/health/ready" >/dev/null 2>&1; then
		ok=1
		break
	fi
	sleep 0.25
done
if [ "${ok}" -ne 1 ]; then
	echo "management ready check failed on 127.0.0.1:${mgmt_port}" >&2
	docker logs "${NAME}" >&2 || true
	exit 1
fi

# The image has no shell; docker exec still runs the copied binary.
if ! docker exec "${NAME}" /labdns version >/dev/null; then
	echo "non-root exec of /labdns version failed" >&2
	exit 1
fi

query_ok() {
	local transport="$1"
	local port="$2"
	local out
	out="$(cd "${ROOT}" && go run ./cmd/labdns query --name ns1.lab.example.net. --type A --transport "${transport}" --server "127.0.0.1:${port}")"
	printf '%s\n' "${out}"
	if ! printf '%s\n' "${out}" | grep -q 'rcode=NOERROR'; then
		echo "${transport} query missing rcode=NOERROR" >&2
		exit 1
	fi
	if ! printf '%s\n' "${out}" | grep -q '10.42.0.53'; then
		echo "${transport} query missing 10.42.0.53" >&2
		exit 1
	fi
}
query_ok udp "${udp_port}"
query_ok tcp "${tcp_port}"

echo "container contract ok image=${IMAGE}"
