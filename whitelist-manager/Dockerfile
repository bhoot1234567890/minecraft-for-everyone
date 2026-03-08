FROM golang:1.21-alpine AS builder

WORKDIR /app

COPY go.mod ./
COPY go.sum* ./
RUN go mod download

COPY . .
RUN go mod tidy && CGO_ENABLED=0 go build -o whitelist-manager .

FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /root/

COPY --from=builder /app/whitelist-manager .
COPY --from=builder /app/static ./static

EXPOSE 3000

CMD ["./whitelist-manager"]
