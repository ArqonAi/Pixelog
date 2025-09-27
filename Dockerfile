# Streamlined Docker build for Pixelog
FROM golang:1.25-alpine AS backend-builder

RUN apk add --no-cache git ca-certificates

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY backend/ ./backend/
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o pixelog-server ./backend/cmd/server
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o pixelog-cli ./backend/cmd/pixelog

FROM alpine:latest

RUN apk --no-cache add ca-certificates ffmpeg wget
RUN mkdir -p /app/frontend/dist /app/output

WORKDIR /app

# Copy binaries
COPY --from=backend-builder /app/pixelog-server .
COPY --from=backend-builder /app/pixelog-cli .

# Copy prebuilt frontend assets
COPY frontend/dist ./frontend/dist

# Create non-root user
RUN adduser -D -s /bin/sh pixelog
RUN chown -R pixelog:pixelog /app
USER pixelog

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:8080/api/v1/health || exit 1

CMD ["./pixelog-server", "-port=8080"]