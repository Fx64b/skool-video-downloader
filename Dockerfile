FROM golang:1.24-alpine AS builder

# Install build dependencies
RUN apk add --no-cache gcc musl-dev

WORKDIR /build
COPY . .
RUN go mod download
RUN go build -o skool-downloader .

FROM alpine:latest

# Install runtime dependencies
RUN apk add --no-cache ffmpeg python3 curl chromium \
    && curl -L https://github.com/yt-dlp/yt-dlp/releases/latest/download/yt-dlp -o /usr/local/bin/yt-dlp \
    && chmod +x /usr/local/bin/yt-dlp

# Create non-root user and setup directories
RUN addgroup -S appgroup && adduser -S appuser -G appgroup
WORKDIR /data
RUN chown -R appuser:appgroup /data

# Copy executable from builder
COPY --from=builder /build/skool-downloader /usr/local/bin/

# Switch to non-root user
USER appuser

ENTRYPOINT ["skool-downloader"]