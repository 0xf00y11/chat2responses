FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /chat2responses ./cmd/chat2responses

FROM scratch
COPY --from=builder /chat2responses /chat2responses
EXPOSE 8000
CMD ["/chat2responses", "serve"]
