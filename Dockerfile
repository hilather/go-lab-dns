# LabDNS production image: ghcr.io/hilather/labdns
#
# Multi-stage, static binary, numeric non-root UID, no shell.
# Run with a read-only root filesystem, cap_drop ALL, and no-new-privileges.
# Host port 53 maps to container 5353. Management is :8080.

# Node 22.14.0 alpine, digest pinned (multi-arch index).
FROM node:22.14.0-alpine@sha256:9bef0ef1e268f60627da9ba7d7605e8831d5b56ad07487d24d1aa386336d1944 AS web
WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
RUN npm run build && test -f dist/index.html

FROM golang:1.26.6-alpine AS build
WORKDIR /src

RUN apk add --no-cache ca-certificates tzdata

COPY go.mod go.sum ./
RUN go mod download

COPY . .
# Overlay Vite output on the committed embed stub so go:embed sees production assets.
COPY --from=web /src/web/dist ./internal/web/dist
RUN test -f internal/web/dist/index.html
RUN test -d internal/web/dist/assets && ls internal/web/dist/assets | grep -q .

ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_TIME=unknown
ARG TARGETARCH

RUN CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH:-amd64} go build -trimpath \
	-ldflags="-s -w \
	-X github.com/hilather/go-lab-dns/internal/buildinfo.version=${VERSION} \
	-X github.com/hilather/go-lab-dns/internal/buildinfo.commit=${COMMIT} \
	-X github.com/hilather/go-lab-dns/internal/buildinfo.buildTime=${BUILD_TIME}" \
	-o /out/labdns ./cmd/labdns \
	&& printf 'labdns:x:65532:65532:labdns:/:/sbin/nologin\n' > /out/passwd \
	&& printf 'labdns:x:65532:\n' > /out/group \
	&& cp /etc/ssl/certs/ca-certificates.crt /out/ca-certificates.crt \
	&& cp LICENSE /out/LICENSE

FROM scratch

LABEL org.opencontainers.image.title="labdns" \
	org.opencontainers.image.description="Laboratory DNS override, wildcard, forwarding, and chaos service" \
	org.opencontainers.image.source="https://github.com/hilather/go-lab-dns" \
	org.opencontainers.image.url="https://github.com/hilather/go-lab-dns" \
	org.opencontainers.image.licenses="Apache-2.0" \
	org.opencontainers.image.vendor="hilather" \
	org.opencontainers.image.documentation="https://github.com/hilather/go-lab-dns/blob/main/docs/11-deployment.md"

COPY --from=build /out/passwd /etc/passwd
COPY --from=build /out/group /etc/group
COPY --from=build /out/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=build /out/labdns /labdns
COPY --from=build /out/LICENSE /LICENSE

USER 65532:65532
EXPOSE 5353/udp 5353/tcp 8080/tcp
WORKDIR /

HEALTHCHECK --interval=10s --timeout=3s --start-period=3s --retries=3 \
	CMD ["/labdns", "healthcheck", "--url=http://127.0.0.1:8080/v1/health/ready"]

ENTRYPOINT ["/labdns"]
CMD ["serve", "--config=/etc/labdns/config.yaml"]
