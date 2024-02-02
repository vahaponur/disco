FROM golang:1.16-alpine as builder
WORKDIR /app
COPY . .
RUN apk add --no-cache git
RUN apk add --no-cache gcc musl-dev opus opusfile ffmpeg
RUN apk add --update python3 py3-pip
RUN pip3 install spotdl
RUN go get -d -v ./...
RUN go install -v ./...
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o bot .
FROM alpine:latest
RUN apk --no-cache add ca-certificates

WORKDIR /root/
COPY --from=builder /app/bot .
COPY --from=builder /app/config.yaml .
EXPOSE 8080
CMD ["./bot"]
