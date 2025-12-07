# Multi-stage build for Multi-Avatar Chat Application

# Stage 1: Build frontend
FROM node:20-alpine AS frontend-builder
WORKDIR /app/frontend

# Install dependencies
COPY frontend/package.json frontend/yarn.lock* ./
RUN yarn install --frozen-lockfile || yarn install

# Copy source and build
COPY frontend/ ./
RUN yarn build

# Stage 2: Build backend
FROM golang:1.23-alpine AS backend-builder

# Install build dependencies for CGO (SQLite)
RUN apk add --no-cache gcc musl-dev

WORKDIR /app/backend

# Download dependencies
COPY backend/go.mod backend/go.sum* ./
RUN go mod download

# Copy source and build
COPY backend/ ./
RUN CGO_ENABLED=1 GOOS=linux go build -o /app/server ./cmd/server

# Stage 3: Final runtime image
FROM alpine:3.19

# Install runtime dependencies
RUN apk add --no-cache ca-certificates sqlite-libs

WORKDIR /app

# Copy backend binary
COPY --from=backend-builder /app/server ./server

# Copy frontend static files
COPY --from=frontend-builder /app/frontend/dist ./static

# Create data directory for SQLite
RUN mkdir -p /app/data

# Copy settings directory structure (secrets will be mounted)
RUN mkdir -p /app/settings/secrets

EXPOSE 8080

ENV DB_PATH=/app/data/app.db
ENV STATIC_DIR=/app/static
ENV SETTINGS_DIR=/app/settings
ENV PORT=8080

CMD ["./server"]
