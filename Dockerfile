FROM golang:1.22-alpine AS builder

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o flamarr .

# ──────────────────────────────────────────────────────
FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata && \
    addgroup -S flamarr && adduser -S -G flamarr flamarr

WORKDIR /app
COPY --from=builder /build/flamarr .

RUN mkdir -p /data && chown flamarr:flamarr /data
VOLUME ["/data"]

USER flamarr

ENV PORT=5005 \
    DATA_DIR=/data

EXPOSE 5005

ENTRYPOINT ["/app/flamarr"]
