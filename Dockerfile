# Первый этап собирает Go-бинарник и не попадает в финальный образ.
FROM golang:1.26.5-alpine3.24 AS builder

WORKDIR /src

# Зависимости копируются отдельно, чтобы Docker кэшировал go mod download.
COPY go.mod go.sum ./
RUN go mod download

# Копируются только исходники, необходимые для сборки серверного бинарника.
COPY cmd ./cmd
COPY internal ./internal
COPY docs/docs.go ./docs/docs.go

# CGO отключён для статического Linux-бинарника; -s -w уменьшают его размер.
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/server ./cmd/server

# Второй этап содержит только runtime-файлы, поэтому финальный образ меньше.
FROM alpine:3.24.1

WORKDIR /app

# ca-certificates нужны для TLS, а отдельный user не даёт приложению работать от root.
RUN apk add --no-cache ca-certificates \
	&& addgroup -S app \
	&& adduser -S -G app app

COPY --from=builder /out/server ./server

RUN chown app:app /app/server

# x/tools, используемый Swagger runtime, инициализирует индекс в user cache.
# В read-only контейнере cache направляется в уже разрешённый tmpfs /tmp.
ENV PORT=8080 \
	XDG_CACHE_HOME=/tmp/.cache
EXPOSE 8080

USER app

# Docker проверяет не только HTTP-сервер, но и доступность PostgreSQL через /api/health.
HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 \
	CMD wget -qO- http://127.0.0.1:8080/api/health >/dev/null || exit 1

CMD ["./server"]
