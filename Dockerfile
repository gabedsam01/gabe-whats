# syntax=docker/dockerfile:1

FROM golang:1.26-bookworm AS builder

RUN apt-get update \
    && apt-get install -y --no-install-recommends gcc libc6-dev \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY apps ./apps
COPY shared ./shared
COPY docs/openapi.yaml ./docs/openapi.yaml

RUN CGO_ENABLED=1 go build -trimpath -o /out/gabe-whats-api ./apps/api/cmd/server \
    && CGO_ENABLED=0 go build -trimpath -o /out/gabe-whats-mcp ./apps/mcp/cmd/mcp-server

FROM debian:bookworm-slim

RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates tzdata wget \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

COPY --from=builder /out/gabe-whats-api /usr/local/bin/gabe-whats-api
COPY --from=builder /out/gabe-whats-mcp /usr/local/bin/gabe-whats-mcp
COPY docs/openapi.yaml /app/docs/openapi.yaml

RUN mkdir -p /app/data/media

EXPOSE 8080 8090

CMD ["gabe-whats-api"]
