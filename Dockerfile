FROM golang:1.23 AS builder
WORKDIR /webhook
COPY go.mod go.sum main.go webhook.go ./
RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -installsuffix cgo -o mutating-webhook

FROM alpine:latest
WORKDIR /webhook
COPY --from=builder /webhook/mutating-webhook .
RUN chmod +x /webhook/mutating-webhook

ENTRYPOINT ["./mutating-webhook"]
