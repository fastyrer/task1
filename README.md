# Загрузка клиентских данных

Веб-приложение для первого этапа задачи: загрузка CSV/XLS/XLSX, чтение заголовков, preview первых строк и сохранение распарсенных данных в памяти по `fileId`.

## Структура

```text
backend/
  main.go
  handlers/upload.go
  services/file_parser.go
  storage/memory_storage.go
  models/file_data.go
frontend/
  index.html
```

## Запуск

```bash
go mod tidy
go run ./backend
```

После запуска страница доступна по адресу:

```text
http://localhost:8080
```

Если нужен другой порт:

```bash
PORT=8090 go run ./backend
```

## Запуск через Docker

Собрать образ:

```bash
docker build -t task1 .
```

Запустить контейнер:

```bash
docker run --rm -p 8080:8080 task1
```

После запуска страница будет доступна по адресу:

```text
http://localhost:8080
```

## API

```http
POST /api/upload
Content-Type: multipart/form-data
```

Поле файла:

```text
file
```

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
  ]
}
```

## Проверка

```bash
go test ./...
```
