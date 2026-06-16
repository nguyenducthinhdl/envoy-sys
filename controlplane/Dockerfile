FROM golang:1.22-alpine AS builder
WORKDIR /build
COPY visualization/ ./visualization/
COPY controlplane/ ./controlplane/
WORKDIR /build/controlplane
RUN go mod download
RUN go build -o controlplane .

FROM alpine:3.20
RUN apk add --no-cache ca-certificates wget
WORKDIR /app
COPY --from=builder /build/controlplane/controlplane /app/controlplane
EXPOSE 18000 8080 8022
ENTRYPOINT ["/app/controlplane"]
