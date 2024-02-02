FROM golang:1.16-alpine as builder
RUN apk add --no-cache git gcc musl-dev opus opusfile ffmpeg python3 py3-pip
RUN apk add --no-cache python3-dev build-base
RUN pip3 install spotdl
WORKDIR /app
COPY . .
RUN go get -d -v ./...
RUN go install -v ./...
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o bot .
FROM alpine:latest
RUN apk --no-cache add ca-certificates opus opusfile ffmpeg

WORKDIR /root/
COPY --from=builder /app/bot .
COPY --from=builder /app/config.yaml .
CMD ["./bot"]
