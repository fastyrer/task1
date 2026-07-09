# Загрузка клиентских данных

Веб-приложение для загрузки CSV/XLS/XLSX, чтения заголовков, preview первых строк, валидации данных и генерации уведомлений по шаблону.

В production-режиме данные сохраняются в PostgreSQL. Для локальных unit-тестов и разработки без `DATABASE_URL` доступен in-memory storage.

## Структура

```text
backend/
  main.go
  handlers/
    health.go
    upload.go
    search.go
    notification.go
  services/
    file_parser.go
    template.go
  storage/
    store.go
    memory_storage.go
    postgres_storage.go
  models/
    file_data.go
frontend/
  index.html
compose.yml
Dockerfile
```

## Запуск через Docker Compose

Создать локальный `.env`:

```bash
cp .env.example .env
```

Перед production-запуском поменяйте `POSTGRES_PASSWORD` в `.env`.

```bash
docker compose up --build
```

После запуска:

```text
http://localhost:8080
```

Compose поднимает:

- `app` — Go-приложение;
- `db` — PostgreSQL 16 с volume `postgres_data`;
- healthcheck БД и приложения.

Остановить:

```bash
docker compose down
```

Остановить и удалить данные PostgreSQL:

```bash
docker compose down -v
```

## Отдельная сборка Docker

```bash
docker build -t task1 .
docker run --rm -p 8080:8080 \
  -e STORAGE_DRIVER=postgres \
  -e DATABASE_URL='postgres://task1:task1_password@host.docker.internal:5432/task1?sslmode=disable' \
  task1
```

Если `DATABASE_URL` не задан, приложение использует memory storage. Это удобно для быстрой локальной проверки, но данные будут потеряны после рестарта контейнера.

## Конфигурация

| Переменная | Значение по умолчанию | Описание |
| --- | --- | --- |
| `HOST_PORT` | `8080` | HTTP-порт на хосте для Docker Compose |
| `PORT` | `8080` | HTTP-порт внутри контейнера |
| `STORAGE_DRIVER` | `postgres` при наличии `DATABASE_URL`, иначе `memory` | Тип хранилища: `postgres` или `memory` |
| `DATABASE_URL` | собирается в `compose.yml` из `POSTGRES_*` | PostgreSQL connection string |
| `MAX_UPLOAD_SIZE_MB` | `20` | Максимальный размер загружаемого файла |
| `MAX_UPLOAD_SIZE_BYTES` | пусто | Точный лимит размера; имеет приоритет над `MAX_UPLOAD_SIZE_MB` |
| `SEARCH_RESULT_LIMIT` | `1000` | Максимум строк, возвращаемых одним поисковым запросом |
| `DB_MAX_CONNS` | `10` | Максимум соединений в пуле PostgreSQL |
| `DB_MIN_CONNS` | `0` | Минимум соединений в пуле PostgreSQL |
| `DB_CONNECT_TIMEOUT_SECONDS` | `10` | Таймаут подключения к PostgreSQL |
| `DB_QUERY_TIMEOUT_SECONDS` | `5` | Таймаут запросов к PostgreSQL |
| `DB_HEALTH_CHECK_SECONDS` | `30` | Период фоновой проверки соединений |
| `DB_MAX_CONN_LIFETIME_SECONDS` | `3600` | Максимальное время жизни соединения |

## API

### Health

```http
GET /api/health
```

Успешный ответ:

```json
{
  "status": "ok",
  "storage": "postgres"
}
```

### Загрузка файла

```http
POST /api/upload
Content-Type: multipart/form-data
```

Поля формы:

- `file` — CSV/XLS/XLSX-файл;
- `sheet` — опциональное имя или номер листа Excel.

Успешный ответ:

```json
{
  "fileId": "abc123",
  "headers": ["Телефон", "Имя", "Скидка"],
  "previewRows": [
    {
      "Телефон": "+79990001122",
      "Имя": "Анна",
      "Скидка": "15"
    }
  ],
  "stats": {
    "rowCount": 1,
    "columnCount": 3,
    "validRowCount": 1,
    "invalidRowCount": 0,
    "emptyRowCount": 0,
    "skippedRowCount": 0,
    "warningCount": 0
  }
}
```

### Поиск по загруженному файлу

```http
POST /api/search
Content-Type: application/json
```

```json
{
  "fileId": "abc123",
  "query": "+79",
  "limit": 1000
}
```

Ответ:

```json
{
  "query": "+79",
  "headers": ["Телефон", "Имя", "Скидка"],
  "rows": [
    {
      "row": 1,
      "values": {
        "Телефон": "+79990001122",
        "Имя": "Анна",
        "Скидка": "15"
      },
      "matches": [
        {
          "column": "Телефон",
          "value": "+79990001122"
        }
      ]
    }
  ],
  "totalMatches": 1,
  "returned": 1,
  "limit": 1000,
  "truncated": false
}
```

Поиск регистронезависимый и проходит по всем колонкам всех сохраненных строк файла. Если найденных строк больше `SEARCH_RESULT_LIMIT`, API возвращает первые строки и `truncated: true`.

### Preview уведомлений

```http
POST /api/preview
Content-Type: application/json
```

```json
{
  "fileId": "abc123",
  "phoneColumn": "Телефон",
  "template": "Привет, {{Имя}}! Ваша скидка {{Скидка}}%"
}
```

### Экспорт уведомлений

```http
POST /api/export
Content-Type: application/json
```

Тело запроса такое же, как у `/api/preview`. Ответ — CSV-файл `notifications.csv` в UTF-8.

## Проверка

Если Go установлен локально:

```bash
go test ./...
```

Если Go локально не установлен:

```bash
docker run --rm -v "$PWD":/src -w /src golang:1.22-alpine go test ./...
```
