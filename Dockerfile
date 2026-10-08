FROM node:24-alpine AS frontend
WORKDIR /src
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci
COPY frontend/ ./
RUN npm run build

FROM golang:1.26-alpine AS backend
WORKDIR /src
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /kvui ./cmd/kvui

FROM gcr.io/distroless/static:nonroot
COPY --from=backend /kvui /app/kvui
COPY --from=frontend /src/dist /app/static
USER 65532:65532
ENTRYPOINT ["/app/kvui"]
