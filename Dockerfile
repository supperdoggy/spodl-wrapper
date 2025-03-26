# Use the official Golang image as a build stage
FROM golang:1.23-alpine AS builder

# Install git and other dependencies
RUN apk add --no-cache git

WORKDIR /app

# Add GitHub token to authenticate private repositories during go mod download
ARG GITHUB_TOKEN
RUN git config --global url."https://${GITHUB_TOKEN}:@github.com/".insteadOf "https://github.com/"

# Copy go.mod and go.sum files to the workspace
COPY go.mod go.sum ./

# Download all Go modules
RUN go mod download

# Copy the rest of the application's source code
COPY . .

# Build the Go app
RUN go build -o main .

# Use a minimal base image for the final stage
FROM alpine:3.18

WORKDIR /app

# Install git in the final stage as well
RUN apk add --no-cache git

# Add GitHub token to authenticate private repositories
ARG GITHUB_TOKEN
RUN git config --global url."https://${GITHUB_TOKEN}:@github.com/".insteadOf "https://github.com/"

# Copy the pre-built binary from the builder stage
COPY --from=builder /app/main .

# Expose port (optional, if your app listens on a specific port)
EXPOSE 8080

# Run the executable
CMD ["./main"]
