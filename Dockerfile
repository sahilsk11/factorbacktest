# Dockerfile References: https://docs.docker.com/engine/reference/builder/

# Start from the latest golang base image
FROM golang:1.17.2-alpine as build-env

# git is required to fetch go dependencies
RUN apk update && apk add gcc libc-dev && apk add --no-cache ca-certificates git

# Git arguments
ARG GIT_USER
ARG GIT_TOKEN
RUN git config --global url."https://$GIT_USER:$GIT_TOKEN@github.com".insteadOf "https://github.com"

# Private go package access
ENV GOPRIVATE=github.com/perchcredit/*

# Add Maintainer Info
LABEL maintainer="Ayush Jain <ayush@getperch.app>"

# Set the Current Working Directory inside the container
WORKDIR /mobley

# Download all dependencies. Dependencies will be cached if the go.mod and go.sum files are not changed
COPY go.mod .
COPY go.sum .
RUN go mod download

# Copy the source from the current directory to the Working Directory inside the container
COPY . .

# Build go application binaries
RUN CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -ldflags '-linkmode=external' -o ./bin/mobley ./cmd

# Switch to the runtime image
FROM alpine
WORKDIR /mobley

# Install dependencies
RUN apk add --no-cache \
    libc6-compat \
    bash

# As recommended for grpc health probes until the kube native version makes it out of alpha.
# https://github.com/grpc-ecosystem/grpc-health-probe#example-grpc-health-checking-on-kubernetes
RUN GRPC_HEALTH_PROBE_VERSION=v0.4.8 && \
    wget -qO/bin/grpc_health_probe https://github.com/grpc-ecosystem/grpc-health-probe/releases/download/${GRPC_HEALTH_PROBE_VERSION}/grpc_health_probe-linux-amd64 && \
    chmod +x /bin/grpc_health_probe

# Run as appuser instead of root
RUN addgroup -g 1001 -S appuser && adduser -u 1001 -S appuser -G appuser
RUN chown -R appuser:appuser /mobley
USER appuser

# Copy the binaries from the build stage
COPY --from=build-env /mobley/bin/mobley /mobley/bin/mobley
COPY --from=build-env /mobley/migrations /mobley/migrations

