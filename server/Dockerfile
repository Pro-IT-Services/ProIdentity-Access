# syntax=docker/dockerfile:1

FROM node:22-bookworm-slim AS ui-builder
WORKDIR /src/webui
COPY webui/package*.json ./
RUN npm ci
COPY webui/ ./
RUN npm run build

FROM golang:1.24-bookworm AS go-builder
WORKDIR /src
ARG TARGETOS=linux
ARG TARGETARCH=amd64
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=ui-builder /src/webui/dist /src/internal/api/ui/dist
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -trimpath -ldflags="-s -w" -o /out/proidentity ./cmd/server

FROM debian:bookworm-slim AS runtime
RUN apt-get update \
  && apt-get install -y --no-install-recommends \
    ca-certificates \
    iproute2 \
    iptables \
    kmod \
    netcat-openbsd \
    tzdata \
    wireguard-tools \
  && rm -rf /var/lib/apt/lists/*

WORKDIR /app
COPY --from=go-builder /out/proidentity /app/proidentity
COPY migrations /app/migrations
COPY docker/config.docker.yaml /app/config.yaml
COPY docker/entrypoint.sh /usr/local/bin/proidentity-entrypoint
RUN chmod 0755 /usr/local/bin/proidentity-entrypoint /app/proidentity

EXPOSE 8080/tcp
EXPOSE 51820-51840/udp

ENTRYPOINT ["proidentity-entrypoint"]
CMD ["/app/config.yaml"]

