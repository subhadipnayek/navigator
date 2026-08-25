# syntax=docker/dockerfile:1

# Stage 1: Build binary with Go module & compiler caching
FROM golang:1.26-alpine AS builder

WORKDIR /app

RUN apk add --no-cache git ca-certificates tzdata

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY . .
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /app/server ./cmd/server/main.go

# Stage 2: Minimal runtime container (~15MB)
FROM alpine:3.21

WORKDIR /app

RUN apk --no-cache add ca-certificates tzdata curl && \
    addgroup -S appgroup && adduser -S appuser -G appgroup

COPY --from=builder /app/server /app/server

USER appuser:appgroup

EXPOSE 8080

ENV PORT=8080 \
    GIN_MODE=release

ENTRYPOINT ["/app/server"]