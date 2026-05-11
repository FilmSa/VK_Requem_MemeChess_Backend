FROM golang:1.25-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o /out/meme-chess ./cmd/app

FROM alpine:3.22

WORKDIR /app

RUN apk add --no-cache ca-certificates

COPY --from=builder /out/meme-chess /app/meme-chess
COPY migrations /app/migrations
COPY emoji /app/emoji

EXPOSE 8080

CMD ["/app/meme-chess"]
