FROM golang:1.21-bullseye as builder

WORKDIR /app

# Retrieve application dependencies.
# This allows the container build to reuse cached dependencies.
# Expecting to copy go.mod and if present go.sum.
COPY go.* ./
RUN go mod download

# Build tools
RUN go install github.com/go-task/task/v3/cmd/task@latest
RUN go install github.com/swaggo/swag/cmd/swag@latest

# Copy local code to the container image.
COPY . ./

# Build the binary.
RUN task build-api
# RUN go build -v -o server

# Use the official Debian slim image for a lean production container.
# https://hub.docker.com/_/debian
# https://docs.docker.com/develop/develop-images/multistage-build/#use-multi-stage-builds
FROM debian:buster-slim
RUN set -x && apt-get update && DEBIAN_FRONTEND=noninteractive apt-get install -y \
  ca-certificates && \
  rm -rf /var/lib/apt/lists/*

# Copy the binary to the production image from the builder stage.
COPY --from=builder /app/bin/api /app/server

# Run the web service on container startup.
CMD ["/app/server"]
