# AIStack CLI — Multi-stage Docker build
# Stage 1: Build
FROM golang:1.22-alpine AS builder

ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_DATE

WORKDIR /build

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata

# Cache dependencies
COPY cli/go.mod cli/go.sum ./
RUN go mod download

# Build binary
COPY cli/ ./
RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags="-s -w \
      -X 'github.com/workhubonline-soft/aistack/cmd.version=${VERSION}' \
      -X 'github.com/workhubonline-soft/aistack/cmd.commit=${COMMIT}' \
      -X 'github.com/workhubonline-soft/aistack/cmd.buildDate=${BUILD_DATE}'" \
    -o /aistack \
    .

# Stage 2: Minimal runtime
FROM alpine:3.19

# Required for TLS and timezone
RUN apk add --no-cache ca-certificates tzdata docker-cli

# Copy binary
COPY --from=builder /aistack /usr/local/bin/aistack

# Copy catalog and configs
COPY models/ /opt/aistack/models/
COPY compose/ /opt/aistack/compose/
COPY configs/ /opt/aistack/configs/

# Create required directories
RUN mkdir -p /var/lib/aistack /var/log/aistack

WORKDIR /opt/aistack

ENTRYPOINT ["aistack"]
CMD ["--help"]

LABEL org.opencontainers.image.title="AIStack CLI"
LABEL org.opencontainers.image.description="Self-hosted AI stack installer and manager"
LABEL org.opencontainers.image.source="https://github.com/workhubonline-soft/aistack"
LABEL org.opencontainers.image.licenses="MIT"
