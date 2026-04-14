# Stage 1: Build the frontend
FROM node:22-alpine3.19 AS frontend-builder
WORKDIR /web
COPY web/package*.json ./
RUN npm install
COPY web/ ./
RUN npm run build

# Stage 2: Build the backend
FROM golang:1.22-alpine3.19 AS backend-builder
WORKDIR /app
# Optimize Go dependency caching
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# Copy frontend dist from previous stage
COPY --from=frontend-builder /web/dist /app/web/dist
RUN go build -o sgound-server ./cmd/sgound

# Stage 3: Runtime
FROM alpine:3.19
WORKDIR /app
RUN apk add --no-cache ca-certificates tini ffmpeg

# Create non-root user
RUN addgroup -S sgound && adduser -S sgound -G sgound -u 1000

COPY --from=backend-builder /app/sgound-server /app/
# Ensure directories exist with correct permissions
RUN mkdir -p /app/data /music && chown -R sgound:sgound /app/data /music

ENV SGOUND_PORT=8088
ENV SGOUND_MUSIC_PATH=/music
ENV SGOUND_DB_PATH=/app/data/sgound.db

USER sgound
EXPOSE 8088
ENTRYPOINT ["/sbin/tini", "--"]
CMD ["./sgound-server"]
