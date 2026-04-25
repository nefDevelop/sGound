# Stage 1: Build the frontend
FROM node:22-alpine AS frontend-builder
WORKDIR /web
COPY web/package*.json ./
RUN npm install
COPY web/ ./
RUN npm run build

# Stage 2: Build the backend
FROM golang:alpine AS backend-builder
WORKDIR /app

# Step 1: Cache Go modules
COPY go.mod go.sum ./
RUN go mod download

# Step 2: Copy only necessary backend files
COPY cmd/ ./cmd/
COPY internal/ ./internal/
COPY web/embed.go ./web/embed.go

# Step 3: Copy frontend dist from previous stage
COPY --from=frontend-builder /web/dist /app/web/dist

# Step 4: Build binary (statically linked and stripped)
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o sgound ./cmd/sgound

# Stage 3: Runtime
FROM alpine:3.23
WORKDIR /app
RUN apk add --no-cache ca-certificates tini ffmpeg

# Create non-root user
RUN addgroup -S sgound && adduser -S sgound -G sgound -u 1000

COPY --from=backend-builder /app/sgound /app/
# Ensure directories exist with correct permissions
RUN mkdir -p /app/data /music && chown -R sgound:sgound /app/data /music

ENV SGOUND_PORT=8088
ENV SGOUND_MUSIC_PATH=/music
ENV SGOUND_DB_PATH=/app/data/sgound.db

USER sgound
EXPOSE 8088
ENTRYPOINT ["/sbin/tini", "--"]
CMD ["./sgound"]
