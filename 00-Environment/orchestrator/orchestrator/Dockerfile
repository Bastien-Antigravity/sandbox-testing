FROM golang:1.25.4-alpine3.22

# Install Docker CLI to interact with the host Docker engine via socket
RUN apk add --no-cache docker-cli bash git

# Set up working directory
WORKDIR /scenarios

# Copy scenarios for local development (will be overridden by volume in docker-compose)
COPY . /scenarios

# Default command: run all Go tests
CMD ["go", "test", "-v", "./go/..."]
