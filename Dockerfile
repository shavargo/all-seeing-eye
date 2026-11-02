# =============================================================================
# Multi-stage Unified All-In-One Dockerfile
# Combines Go Master Server Daemon + Next.js The All-Seeing Eye Web Browser
# Zero-clone deployment: docker run -d -p 27900:27900/udp -p 3000:3000 ...
# =============================================================================

# Step 1: Build the Go Universal Master Server
FROM golang:alpine AS go-builder
WORKDIR /build
COPY go.mod ./
COPY cmd/ ./cmd/
COPY internal/ ./internal/
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /master ./cmd/master/main.go

# Step 2: Build the Next.js Standalone Web Application
FROM node:22-alpine AS web-builder
WORKDIR /app
COPY web/package*.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

# Step 3: Minimal Unified Runtime Image
FROM node:22-alpine AS runtime
WORKDIR /app

ENV NODE_ENV=production \
    PORT=3000 \
    HOSTNAME="0.0.0.0" \
    UDP_ADDR=":27900" \
    KPQ3_ADDR=":27950" \
    TCP_ADDR=":28900" \
    API_ADDR="127.0.0.1:8085" \
    MASTER_API_URL="http://127.0.0.1:8085"

# Install runtime utilities & signal forwarder
RUN apk --no-cache add ca-certificates tzdata dumb-init

# Copy compiled Go master daemon
COPY --from=go-builder /master /usr/local/bin/master

# Copy Next.js standalone server and static assets
COPY --from=web-builder /app/public ./public
COPY --from=web-builder /app/.next/standalone ./
COPY --from=web-builder /app/.next/static ./.next/static

# Copy entrypoint script
COPY docker-entrypoint.sh /usr/local/bin/docker-entrypoint.sh
RUN chmod +x /usr/local/bin/docker-entrypoint.sh

# Ports: Quake2/GameSpy UDP, DarkPlaces UDP, GameSpy TCP, Web Dashboard TCP
EXPOSE 27900/udp 27950/udp 28900/tcp 3000/tcp

ENTRYPOINT ["/usr/bin/dumb-init", "--", "/usr/local/bin/docker-entrypoint.sh"]
