# LabDNS container image is implemented in PR-16 (DEP-001).
# This stub exists so the repository layout is complete.
# `make test-container` and the container-test CI job fail closed until then.
#
# Planned image: ghcr.io/hilather/labdns
# Multi-stage, numeric non-root UID, read-only root filesystem, cap_drop ALL.
FROM alpine:3.22
RUN echo "unimplemented until PR-16 (DEP-001)" >&2 && exit 1
