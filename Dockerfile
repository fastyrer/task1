FROM golang:1.22-alpine AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY backend ./backend
COPY frontend ./frontend

RUN CGO_ENABLED=0 GOOS=linux go build -o /out/server ./backend

FROM alpine:3.20

WORKDIR /app

RUN apk add --no-cache ca-certificates \
	&& addgroup -S app \
	&& adduser -S -G app app

COPY --from=builder /out/server ./server
COPY --from=builder /src/frontend ./frontend

RUN chown -R app:app /app

ENV PORT=8080
EXPOSE 8080

USER app

HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 \
	CMD wget -qO- http://127.0.0.1:8080/api/health >/dev/null || exit 1

CMD ["./server"]
