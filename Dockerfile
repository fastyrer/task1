FROM golang:1.22-alpine AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY backend ./backend
COPY frontend ./frontend

RUN CGO_ENABLED=0 GOOS=linux go build -o /out/server ./backend

FROM alpine:3.20

WORKDIR /app

COPY --from=builder /out/server ./server
COPY --from=builder /src/frontend ./frontend

ENV PORT=8080
EXPOSE 8080

CMD ["./server"]
