FROM golang:1.20-alpine as builder

# Install system dependencies
RUN apk add --no-cache git gcc musl-dev opus opusfile ffmpeg python3 py3-pip
RUN apk add --no-cache python3-dev build-base

# Setup a Python virtual environment and install spotdl
RUN python3 -m venv /venv
ENV PATH="/venv/bin:$PATH"
RUN pip3 install --upgrade pip
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

# Copy the virtual environment from the builder stage
COPY --from=builder /venv /venv

WORKDIR /root/
COPY --from=builder /app/disco .
COPY --from=builder /app/config.yaml .
CMD ["./disco"]
