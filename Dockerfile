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
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# Copy frontend dist from previous stage
COPY --from=frontend-builder /web/dist /app/web/dist
RUN go build -o sgound-server ./cmd/sgound

# Stage 3: Runtime
FROM alpine:latest
WORKDIR /app
RUN apk add --no-cache ca-certificates
COPY --from=backend-builder /app/sgound-server /app/
# Ensure directories exist
RUN mkdir -p /app/data /music

ENV SGOUND_PORT=8088
ENV SGOUND_MUSIC_PATH=/music
ENV SGOUND_DB_PATH=/app/data/sgound.db

EXPOSE 8088
CMD ["./sgound-server"]
