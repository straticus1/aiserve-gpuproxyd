FROM golang:1.24-alpine AS builder

# Use GOTOOLCHAIN=auto to allow Go to download the required version
ENV GOTOOLCHAIN=auto

WORKDIR /app

RUN apk add --no-cache git gcc musl-dev sqlite-dev

COPY go.mod go.sum ./

RUN go mod download

COPY . .

RUN CGO_ENABLED=1 GOOS=linux go build -o /app/bin/aiserve-gpuproxyd ./cmd/server
RUN CGO_ENABLED=1 GOOS=linux go build -o /app/bin/aiserve-gpuproxy-client ./cmd/client
RUN CGO_ENABLED=1 GOOS=linux go build -o /app/bin/aiserve-gpuproxy-admin ./cmd/admin

# Validate binaries were built successfully
RUN test -f /app/bin/aiserve-gpuproxyd || (echo "ERROR: aiserve-gpuproxyd binary not found" && exit 1)
RUN test -x /app/bin/aiserve-gpuproxyd || (echo "ERROR: aiserve-gpuproxyd binary not executable" && exit 1)
RUN test -f /app/bin/aiserve-gpuproxy-client || (echo "ERROR: aiserve-gpuproxy-client binary not found" && exit 1)
RUN test -x /app/bin/aiserve-gpuproxy-client || (echo "ERROR: aiserve-gpuproxy-client binary not executable" && exit 1)
RUN test -f /app/bin/aiserve-gpuproxy-admin || (echo "ERROR: aiserve-gpuproxy-admin binary not found" && exit 1)
RUN test -x /app/bin/aiserve-gpuproxy-admin || (echo "ERROR: aiserve-gpuproxy-admin binary not executable" && exit 1)

FROM alpine:latest

RUN apk --no-cache add ca-certificates sqlite-libs

WORKDIR /app

COPY --from=builder /app/bin/aiserve-gpuproxyd /app/aiserve-gpuproxyd
COPY --from=builder /app/bin/aiserve-gpuproxy-client /app/aiserve-gpuproxy-client
COPY --from=builder /app/bin/aiserve-gpuproxy-admin /app/aiserve-gpuproxy-admin
COPY --from=builder /app/web /app/web

EXPOSE 8080
EXPOSE 9090

# Use :: for IPv6 dual-stack (listens on both IPv4 and IPv6)
ENV SERVER_HOST=::
ENV SERVER_PORT=8080
ENV GRPC_PORT=9090

CMD ["/app/aiserve-gpuproxyd"]
