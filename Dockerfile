FROM golang:1.20-alpine as builder
RUN apk add --no-cache git gcc musl-dev opus opusfile ffmpeg python3 py3-pip
RUN apk add --no-cache python3-dev build-base
RUN pip3 install spotdl
WORKDIR /app

# Copy go mod and sum files to download dependencies
COPY go.mod go.sum ./

# Download all dependencies
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o disco main.go
FROM alpine:3.19.0
RUN apk --no-cache add ca-certificates opus opusfile ffmpeg

WORKDIR /root/
COPY --from=builder /app/disco .
COPY --from=builder /app/config.yaml .
CMD ["./disco"]
