FROM golang:1.22-alpine AS backend-builder
WORKDIR /app
COPY backend/go.mod ./
COPY backend/main.go ./
RUN go build -o backend .

FROM envoyproxy/envoy:v1.31-latest
USER root
RUN apt-get update && apt-get install -y --no-install-recommends curl && rm -rf /var/lib/apt/lists/*

COPY --from=backend-builder /app/backend /backend
COPY sidecar/bootstrap.yaml /etc/envoy/bootstrap.yaml
COPY scripts/entrypoint-sidecar.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

EXPOSE 8081
ENTRYPOINT ["/entrypoint.sh"]
