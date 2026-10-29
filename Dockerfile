# Stage 1: Build static Go binaries
FROM golang:1.23-alpine AS builder

WORKDIR /build

# Copy Go module definition
COPY go.mod ./
RUN go mod download && go mod verify

# Copy backend source code
COPY cmd/ ./cmd/
COPY pkg/ ./pkg/

# Build static stripped binaries for production
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /build/bin/flowgate ./cmd/flowgate
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /build/bin/upstream ./cmd/upstream
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /build/bin/loadgen ./cmd/loadgen

# Stage 2: Distroless-like minimal runtime image
FROM alpine:3.20

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app

COPY --from=builder /build/bin/flowgate /app/flowgate
COPY --from=builder /build/bin/upstream /app/upstream
COPY --from=builder /build/bin/loadgen /app/loadgen

EXPOSE 8080 9090

ENTRYPOINT ["/app/flowgate"]
CMD ["-port", "8080", "-upstream", "http://127.0.0.1:9090"]
