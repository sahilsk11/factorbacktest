# Use an official Golang runtime as the base image
FROM golang:1.20

# Set the working directory inside the container
WORKDIR /app

# Copy the local package files to the container's workspace
COPY . . 
COPY secrets-test.json /go/src/app/secrets-test.json
# Install any dependencies if needed (e.g., using go get)
RUN go get -d -v ./...

# Build the Go application for x86_64 architecture
RUN GOARCH=amd64 GOOS=linux go build -o ./bin/ ./cmd/api

# Expose a port if your application listens on a specific port
EXPOSE 3009

# Define the command to run your application when the container starts
CMD ["/app/bin/api"]
