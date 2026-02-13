FROM golang:1.25-alpine AS builder

RUN apk add --no-cache gcc musl-dev

WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY server.go .
RUN CGO_ENABLED=1 go build -o urlaubsplaner -ldflags="-s -w" server.go

FROM alpine:latest

RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=builder /build/urlaubsplaner .
COPY index.html .

EXPOSE 8080
ENV URLAUBSPLANER_DB_PATH=/app/data/urlaubsplaner.db
CMD ["./urlaubsplaner"]
