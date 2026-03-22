# Build stage
FROM docker.io/library/golang:1.22-alpine AS builder

RUN apk add --no-cache git pkgconfig libpcap-dev gcc musl-dev

WORKDIR /build

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY . .

ARG TARGETOS=linux
ARG TARGETARCH=amd64
ENV GOOS=${TARGETOS} GOARCH=${TARGETARCH}
ENV CGO_ENABLED=1
ENV CC=gcc

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=.cache/go-build,id=${TARGETOS}-${TARGETARCH} \
    go build -ldflags="-s -w" -o gateway ./cmd/gateway

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=.cache/go-build,id=${TARGETOS}-${TARGETARCH} \
    go build -ldflags="-s -w" -o sender ./cmd/sender

# Frontend build stage
FROM docker.io/library/node:20-alpine AS frontend-builder

WORKDIR /build/frontend

COPY frontend/package*.json ./
RUN npm ci --prefer-offline

COPY frontend/ ./
RUN npm run build

# Final stage
FROM docker.io/library/alpine:3.19

RUN apk add --no-cache ca-certificates tzdata curl libpcap

RUN addgroup -S app && adduser -S app -G app

COPY --from=builder /build/gateway /bin/gateway
COPY --from=builder /build/sender /bin/sender
COPY --from=frontend-builder /build/frontend/dist /app/frontend

RUN mkdir -p /app/logs && chown -R app:app /app

WORKDIR /app

ENV GATEWAY_WEB_ROOT=/app/frontend

EXPOSE 8080 6060 12101

USER app

ENTRYPOINT ["gateway"]
