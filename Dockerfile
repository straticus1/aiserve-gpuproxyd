FROM golang:1.22-alpine AS builder

WORKDIR /app

RUN apk add --no-cache git gcc musl-dev sqlite-dev

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=1 GOOS=linux go build -o /app/bin/aiserve-gpuproxyd ./cmd/server
RUN CGO_ENABLED=1 GOOS=linux go build -o /app/bin/aiserve-gpuproxy-client ./cmd/client
RUN CGO_ENABLED=1 GOOS=linux go build -o /app/bin/aiserve-gpuproxy-admin ./cmd/admin

FROM alpine:latest

RUN apk --no-cache add ca-certificates sqlite-libs

WORKDIR /app

COPY --from=builder /app/bin/aiserve-gpuproxyd /app/aiserve-gpuproxyd
COPY --from=builder /app/bin/aiserve-gpuproxy-client /app/aiserve-gpuproxy-client
COPY --from=builder /app/bin/aiserve-gpuproxy-admin /app/aiserve-gpuproxy-admin
COPY --from=builder /app/web /app/web

EXPOSE 8080

ENV SERVER_HOST=0.0.0.0
ENV SERVER_PORT=8080

CMD ["/app/aiserve-gpuproxyd"]
