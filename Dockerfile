FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o authgraph-agent ./cmd/agent

FROM alpine:3.20
RUN apk --no-cache add ca-certificates
COPY --from=builder /app/authgraph-agent /usr/local/bin/authgraph-agent
EXPOSE 3000
ENTRYPOINT ["authgraph-agent"]
CMD ["serve"]
