# Stage 1: Build frontend
FROM oven/bun:1 AS frontend-build
ARG PUBLIC_SUPABASE_URL=https://bwzldaesynlruqukbnej.supabase.co
ARG PUBLIC_SUPABASE_ANON_KEY
ENV PUBLIC_SUPABASE_URL=$PUBLIC_SUPABASE_URL
ENV PUBLIC_SUPABASE_ANON_KEY=$PUBLIC_SUPABASE_ANON_KEY
WORKDIR /app/frontend
COPY frontend/package.json frontend/bun.lock ./
RUN bun install --frozen-lockfile
COPY frontend/ ./
RUN bun run build

# Stage 2: Build backend
FROM golang:1.26-bookworm AS backend-build
WORKDIR /app/backend
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
RUN CGO_ENABLED=0 go build -o /server ./cmd/server

# Stage 3: Production image
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=backend-build /server /server
COPY --from=frontend-build /app/frontend/dist /frontend/dist
ENV STATIC_DIR=/frontend/dist
ENV HOST=0.0.0.0
ENV PORT=8080
EXPOSE 8080
ENTRYPOINT ["/server"]
