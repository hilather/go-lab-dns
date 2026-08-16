# Shared helpers for the copyable deployment repo. Sourced, not executed.
# Fail closed: callers must `set -euo pipefail` before sourcing.

deploy_root() {
	cd "$(dirname "${BASH_SOURCE[0]}")/.."
	pwd
}

require_cmd() {
	if ! command -v "$1" >/dev/null 2>&1; then
		echo "missing required command: $1" >&2
		exit 1
	fi
}

labdns_bin() {
	if [ -n "${LABDNS:-}" ]; then
		printf '%s' "${LABDNS}"
		return
	fi
	if command -v labdns >/dev/null 2>&1; then
		printf '%s' labdns
		return
	fi
	echo "labdns is not on PATH; set LABDNS to the binary (or go run ./cmd/labdns from the application repo)" >&2
	exit 1
}

env_dir() {
	local env="$1"
	local root
	root="$(deploy_root)"
	local dir="${root}/environments/${env}"
	if [ ! -d "${dir}" ]; then
		echo "unknown environment ${env} (expected ${dir})" >&2
		exit 1
	fi
	printf '%s' "${dir}"
}

load_image_env() {
	local file="$1"
	if [ ! -f "${file}" ]; then
		echo "missing ${file}" >&2
		exit 1
	fi
	# shellcheck disable=SC1090
	set -a
	# shellcheck disable=SC1090
	. "${file}"
	set +a
	if [ -z "${LABDNS_IMAGE:-}" ]; then
		echo "${file}: LABDNS_IMAGE is required" >&2
		exit 1
	fi
}

# One generation: previous successful deploy stays in .last. The next
# deploy copies .last to .previous so rollback can restore it.
rotate_deploy_snapshot() {
	local dir="$1"
	if [ -f "${dir}/.last/dns.yaml" ] && [ -f "${dir}/.last/image.env" ]; then
		mkdir -p "${dir}/.previous"
		cp -f "${dir}/.last/dns.yaml" "${dir}/.last/image.env" "${dir}/.previous/"
	fi
}

record_deploy_snapshot() {
	local dir="$1"
	mkdir -p "${dir}/.last"
	cp -f "${dir}/dns.yaml" "${dir}/image.env" "${dir}/.last/"
}
