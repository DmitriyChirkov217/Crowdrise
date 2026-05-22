# Crowdrise MVP

MVP краудфандинговой платформы с обязательными этапами: деньги после оплаты попадают в `reserved`, а автор получает учётный доступ к ним через `available` только после подтверждения этапа администратором.

## Стек

- Backend: Go, chi, pgx, JWT, bcrypt, zerolog, PostgreSQL.
- Frontend: React, Vite, React Router, Fetch API.
- Infra: Docker Compose, SQL migrations, OpenAPI.

## Быстрый запуск

```bash
cp .env.example .env
docker compose up -d
```

Backend будет доступен на `http://localhost:8080`, frontend запускается отдельно:

```bash
cd frontend
npm install
npm run dev
```

Если нужно запустить backend без контейнера:

```bash
cd backend
go run ./cmd/api
```

## Миграции

При первом старте PostgreSQL через Docker миграция `backend/internal/db/migrations/001_init.sql` применяется автоматически как init script.

Через goose:

```bash
cd backend
go install github.com/pressly/goose/v3/cmd/goose@latest
goose -dir internal/db/migrations postgres "$DATABASE_URL" up
goose -dir internal/db/migrations postgres "$DATABASE_URL" down
```

## Тесты

```bash
cd backend
go test ./...
```

## Демо-аккаунты

- `admin@example.com` / `admin12345` с ролью `admin`.
- `author@example.com` / `admin12345` с ролями `author`, `backer`.
- `backer@example.com` / `admin12345` с ролями `author`, `backer`.

## Основной сценарий

1. Зарегистрируйтесь или войдите как автор.
2. Создайте reward-проект в `/projects/new`.
3. Добавьте этапы в `/projects/{id}/milestones` так, чтобы сумма этапов равнялась цели.
4. Добавьте вознаграждение в `/projects/{id}/rewards`.
5. Отправьте проект на модерацию через API `POST /api/v1/projects/{id}/submit`.
6. Войдите как администратор и одобрите проект в `/admin/projects`.
7. Поддержите опубликованный проект с другого аккаунта.
8. Вызовите mock-capture платежа через API `POST /api/v1/payments/{payment_id}/mock-capture` от имени администратора.
9. Автор отправляет отчёт по этапу, администратор подтверждает его в `/admin/milestones`.
10. В `project_funds` сумма переходит из `total_reserved` в `total_available`, а `fund_ledger` содержит `collect`, `reserve`, `release`.

## Документация API

OpenAPI-спецификация лежит в [openapi.yaml](./openapi.yaml).

## Структура

```text
backend/
  cmd/api
  internal/config
  internal/domain
  internal/http
  internal/repositories
  internal/services
  internal/outbox
  internal/db/migrations
frontend/
docker-compose.yml
.env.example
openapi.yaml
README.md
```

## Broadcast voice rooms

Broadcast rooms are stored in `broadcast`, and author file URLs are stored in `broadcast_chat_files`.
Voice uses WebRTC mesh: the backend does not carry audio, it only relays signaling through WebSocket.

```text
GET  /api/v1/projects/{project_id}/broadcasts
POST /api/v1/projects/{project_id}/broadcasts
PUT  /api/v1/broadcasts/{broadcast_id}/status
GET  /api/v1/broadcasts/{broadcast_id}/files
POST /api/v1/broadcasts/{broadcast_id}/files
GET  /api/v1/broadcasts/{broadcast_id}/ws?token={jwt}
```

Users must be authenticated to join voice. Participants are not persisted in the database. Only the project author can add broadcast files.
