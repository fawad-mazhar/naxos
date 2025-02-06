# Dockerfile
# syntax=docker/dockerfile:1.4
FROM golang:1.23-bookworm AS builder

ARG upx_version=4.2.4
ARG arch=amd64

RUN <<EOF
apt-get update 
apt-get install -y --no-install-recommends xz-utils 
curl -Ls https://github.com/upx/upx/releases/download/v${upx_version}/upx-${upx_version}-${arch}_linux.tar.xz -o - | tar xvJf - -C /tmp 
cp /tmp/upx-${upx_version}-${arch}_linux/upx /usr/local/bin/
chmod +x /usr/local/bin/upx 
apt-get remove -y xz-utils
rm -rf /var/lib/apt/lists/*
EOF


WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the API server
RUN <<EOF
CGO_ENABLED=0 GOARCH=${arch} GOOS=linux go build -a -ldflags="-s -w" -installsuffix cgo -o /app/api ./cmd/api
CGO_ENABLED=0 GOARCH=${arch} GOOS=linux go build -a -ldflags="-s -w" -installsuffix cgo -o /app/runner ./cmd/runner
upx --ultra-brute -qq /app/api && upx -t /app/api
upx --ultra-brute -qq /app/runner && upx -t /app/runner
EOF


FROM registry.access.redhat.com/ubi9-minimal:latest AS runner

WORKDIR /app
# Copy the binary
COPY --from=builder /app/api /app/api
COPY --from=builder /app/runner /app/runner
COPY --from=builder /app/naxos.yaml /app/naxos.yaml
