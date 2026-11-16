# =============================================================================
# Multi-stage Unified All-In-One Dockerfile
# Combines Go Master Server Daemon + Nginx Web Server & Server Directory
# =============================================================================

# Step 1: Build the Go Universal Master Server
FROM golang:alpine AS go-builder
WORKDIR /build
COPY go.mod ./
COPY cmd/ ./cmd/
COPY internal/ ./internal/
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /master ./cmd/master/main.go

# Step 2: Minimal Unified Runtime Image with Nginx
FROM alpine:3.20 AS runtime

ENV UDP_ADDR=":27900" \
    KPQ3_ADDR=":27950" \
    TCP_ADDR=":28900" \
    UT2004_ADDR=":28902" \
    TCP_V1_ADDR=":28910" \
    API_ADDR="127.0.0.1:8085"

RUN apk --no-cache add ca-certificates tzdata dumb-init nginx

# Copy compiled Go master daemon
COPY --from=go-builder /master /usr/local/bin/master

# Copy Nginx config and static website assets
COPY web/nginx.conf /etc/nginx/http.d/default.conf
COPY web/public /usr/share/nginx/html

# Copy entrypoint script
COPY docker-entrypoint.sh /usr/local/bin/docker-entrypoint.sh
RUN chmod +x /usr/local/bin/docker-entrypoint.sh

# Master TCP and UDP ports + HTTP/Web
EXPOSE 80 3000 28900/tcp 28902/tcp 28910/tcp 27900/udp 27950/udp 20510/udp 20610/udp 20710/udp 20810/udp 28060/udp 29060/udp 20110/udp 27953/udp 29250/udp 27777/udp

ENTRYPOINT ["/usr/bin/dumb-init", "--"]
CMD ["/usr/local/bin/docker-entrypoint.sh"]
