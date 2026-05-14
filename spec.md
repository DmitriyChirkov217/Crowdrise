# Техническое задание для Codex: краудфандинговая платформа с обязательной этапностью

## 1. Цель разработки

Создать MVP веб-приложения краудфандинговой платформы, в которой пользователь может публиковать проекты, поддерживать проекты деньгами, а автор получает доступ к средствам не сразу, а по этапам после подтверждения выполнения промежуточных обязательств администратором.

Ключевая особенность приложения: каждый проект обязан иметь этапы реализации (`milestones`). Средства учитываются внутри системы по модели:

`собрано → зарезервировано → доступно автору / возвращено спонсору`.

Физический вывод средств автору в MVP не реализовывать. Нужно реализовать учётную модель средств: после подтверждения этапа администратором часть средств переводится из `reserved` в `available`.

## 2. Технологический стекl

### Backend

Использовать:

- Go.
- REST API.
- PostgreSQL.
- JWT-аутентификацию.
- bcrypt для хэширования паролей.
- SQL-миграции.
- Структурированное логирование.
- OpenAPI/Swagger-документацию.
- Docker Compose для локального запуска.

Рекомендуемый набор библиотек:

- `github.com/go-chi/chi/v5` — HTTP-роутинг.
- `github.com/jackc/pgx/v5` — работа с PostgreSQL.
- `github.com/golang-jwt/jwt/v5` — JWT.
- `golang.org/x/crypto/bcrypt` — хэширование паролей.
- `github.com/google/uuid` — UUID.
- `github.com/rs/zerolog` — структурированные логи.
- `github.com/pressly/goose/v3` или `golang-migrate` — миграции.

### Frontend

Использовать:

- React.
- Vite.
- JavaScript.
- React Router.
- Fetch API или Axios.
- Простое управление состоянием через React hooks.

Не нужно делать сложный дизайн. Нужен рабочий интерфейс для демонстрации ключевых сценариев.

### Инфраструктура

Создать:

- `docker-compose.yml` для PostgreSQL и backend.
- `.env.example`.
- `README.md` с командами запуска.
- SQL-миграции.
- seed-данные для ролей, категорий и администратора.

## 3. Архитектурный стиль

Приложение должно быть модульным монолитом.

Backend разделить на слои:

```text
cmd/api                 точка входа приложения
internal/config         конфигурация
internal/http           handlers, routes, middleware
internal/domain         доменные модели и статусы
internal/services       бизнес-логика
internal/repositories   работа с PostgreSQL
internal/integrations   платежный провайдер и уведомления
internal/outbox         обработчик notification_outbox
internal/db/migrations  SQL-миграции
```

Не создавать микросервисы. Все модули находятся в одном backend-приложении, но должны быть разделены по ответственности.

## 4. Роли пользователей

В системе должны быть роли:

| Роль | Назначение |
|---|---|
| `backer` | поддерживает проекты |
| `author` | создаёт и ведёт проекты |
| `admin` | модерирует проекты, проверяет этапы, блокирует пользователей |

Один пользователь может иметь несколько ролей.

При регистрации по умолчанию выдавать пользователю роли `backer` и `author`.

Администратор создаётся через seed-данные.

## 5. Основные бизнес-сущности

Реализовать следующие сущности:

1. Пользователи и роли.
2. Проекты.
3. Категории.
4. Медиафайлы проекта.
5. Этапы проекта.
6. Отчёты по этапам.
7. Проверки этапов администратором.
8. Вознаграждения.
9. Поддержки проекта.
10. Платежи.
11. Возвраты.
12. Учёт средств проекта.
13. Журнал финансовых операций.
14. Обновления проекта.
15. Очередь уведомлений `notification_outbox`.

## 6. Модель данных PostgreSQL

Создать миграции для следующих таблиц.

### 6.1. `users`

```sql
id uuid primary key,
email varchar(255) not null unique,
password_hash varchar(255) not null,
display_name varchar(255) not null,
phone varchar(50),
is_blocked boolean not null default false,
created_at timestamp not null default now(),
updated_at timestamp not null default now()
```

### 6.2. `roles`

```sql
id smallserial primary key,
code varchar(50) not null unique,
name varchar(255) not null
```

Seed-данные:

```text
backer
author
admin
```

### 6.3. `user_roles`

```sql
user_id uuid not null references users(id) on delete cascade,
role_id smallint not null references roles(id) on delete cascade,
created_at timestamp not null default now(),
primary key (user_id, role_id)
```

### 6.4. `categories`

```sql
id serial primary key,
name varchar(255) not null unique
```

Seed-данные:

```text
Технологии
Игры
Музыка
Кино
Образование
Социальные проекты
```

### 6.5. `projects`

```sql
id uuid primary key,
author_id uuid not null references users(id),
title varchar(255) not null,
short_description varchar(500) not null,
description text not null,
category_id int references categories(id),
campaign_type varchar(20) not null,
currency varchar(3) not null default 'RUB',
goal_amount numeric(14,2) not null,
start_at timestamp,
end_at timestamp,
status varchar(30) not null,
created_at timestamp not null default now(),
updated_at timestamp not null default now()
```

Ограничения:

```sql
check (campaign_type in ('reward', 'donation')),
check (goal_amount > 0),
check (status in ('draft', 'on_review', 'rejected', 'published', 'completed', 'blocked', 'canceled'))
```

### 6.6. `project_media`

```sql
id uuid primary key,
project_id uuid not null references projects(id) on delete cascade,
media_type varchar(20) not null,
url text not null,
sort_order int not null default 0
```

Ограничение:

```sql
check (media_type in ('image', 'video', 'document'))
```

### 6.7. `milestones`

```sql
id uuid primary key,
project_id uuid not null references projects(id) on delete cascade,
title varchar(255) not null,
description text not null,
due_at timestamp not null,
amount_limit numeric(14,2) not null,
position int not null,
status varchar(30) not null,
created_at timestamp not null default now(),
updated_at timestamp not null default now()
```

Ограничения:

```sql
check (amount_limit > 0),
check (position > 0),
check (status in ('planned', 'in_progress', 'on_review', 'approved', 'rejected', 'overdue')),
unique (project_id, position)
```

### 6.8. `milestone_submissions`

```sql
id uuid primary key,
milestone_id uuid not null references milestones(id) on delete cascade,
author_id uuid not null references users(id),
report_text text not null,
submitted_at timestamp not null default now()
```

### 6.9. `milestone_submission_files`

```sql
id uuid primary key,
submission_id uuid not null references milestone_submissions(id) on delete cascade,
file_url text not null,
file_type varchar(50) not null
```

### 6.10. `milestone_reviews`

```sql
id uuid primary key,
submission_id uuid not null references milestone_submissions(id) on delete cascade,
admin_id uuid not null references users(id),
decision varchar(20) not null,
comment text,
reviewed_at timestamp not null default now()
```

Ограничение:

```sql
check (decision in ('approved', 'rejected'))
```

### 6.11. `rewards`

```sql
id uuid primary key,
project_id uuid not null references projects(id) on delete cascade,
title varchar(255) not null,
description text not null,
min_amount numeric(14,2) not null,
limit_count int,
delivery_estimate date,
created_at timestamp not null default now(),
updated_at timestamp not null default now()
```

Ограничения:

```sql
check (min_amount > 0),
check (limit_count is null or limit_count > 0)
```

### 6.12. `pledges`

```sql
id uuid primary key,
project_id uuid not null references projects(id),
backer_id uuid not null references users(id),
reward_id uuid references rewards(id),
amount numeric(14,2) not null,
status varchar(30) not null,
created_at timestamp not null default now(),
updated_at timestamp not null default now()
```

Ограничения:

```sql
check (amount > 0),
check (status in ('created', 'payment_pending', 'paid', 'canceled', 'refunded'))
```

### 6.13. `payments`

```sql
id uuid primary key,
pledge_id uuid not null references pledges(id),
provider varchar(50) not null,
provider_payment_id varchar(255) not null unique,
status varchar(30) not null,
amount numeric(14,2) not null,
created_at timestamp not null default now(),
updated_at timestamp not null default now()
```

Ограничения:

```sql
check (amount > 0),
check (status in ('created', 'pending', 'captured', 'failed', 'canceled', 'refunded'))
```

### 6.14. `payment_webhook_events`

Нужна для идемпотентности вебхуков.

```sql
id uuid primary key,
provider varchar(50) not null,
provider_event_id varchar(255) not null,
payment_id uuid references payments(id),
event_type varchar(100) not null,
payload jsonb not null,
processed_at timestamp not null default now(),
unique (provider, provider_event_id)
```

### 6.15. `refunds`

```sql
id uuid primary key,
payment_id uuid not null references payments(id),
provider_refund_id varchar(255) unique,
status varchar(30) not null,
amount numeric(14,2) not null,
reason varchar(255),
created_at timestamp not null default now(),
updated_at timestamp not null default now()
```

Ограничения:

```sql
check (amount > 0),
check (status in ('created', 'pending', 'succeeded', 'failed'))
```

### 6.16. `project_funds`

```sql
project_id uuid primary key references projects(id) on delete cascade,
total_collected numeric(14,2) not null default 0,
total_refunded numeric(14,2) not null default 0,
total_available numeric(14,2) not null default 0,
total_reserved numeric(14,2) not null default 0,
updated_at timestamp not null default now()
```

Ограничения:

```sql
check (total_collected >= 0),
check (total_refunded >= 0),
check (total_available >= 0),
check (total_reserved >= 0)
```

### 6.17. `fund_ledger`

```sql
id uuid primary key,
project_id uuid not null references projects(id),
operation_type varchar(30) not null,
amount numeric(14,2) not null,
reference_type varchar(50) not null,
reference_id uuid not null,
created_at timestamp not null default now()
```

Ограничения:

```sql
check (amount > 0),
check (operation_type in ('collect', 'reserve', 'release', 'refund')),
check (reference_type in ('payment', 'refund', 'milestone'))
```

### 6.18. `project_updates`

```sql
id uuid primary key,
project_id uuid not null references projects(id) on delete cascade,
author_id uuid not null references users(id),
title varchar(255) not null,
content text not null,
created_at timestamp not null default now()
```

### 6.19. `notification_outbox`

```sql
id uuid primary key,
user_id uuid not null references users(id),
event_type varchar(100) not null,
payload jsonb not null,
status varchar(30) not null,
attempts int not null default 0,
created_at timestamp not null default now(),
updated_at timestamp not null default now()
```

Ограничение:

```sql
check (status in ('pending', 'processing', 'sent', 'failed'))
```

## 7. Индексы

Добавить индексы:

```sql
create index idx_projects_status on projects(status);
create index idx_projects_category_id on projects(category_id);
create index idx_projects_author_id on projects(author_id);
create index idx_projects_campaign_type on projects(campaign_type);
create index idx_milestones_project_id on milestones(project_id);
create index idx_pledges_project_id on pledges(project_id);
create index idx_pledges_backer_id on pledges(backer_id);
create index idx_payments_pledge_id on payments(pledge_id);
create index idx_fund_ledger_project_id on fund_ledger(project_id);
create index idx_notification_outbox_status on notification_outbox(status);
```

## 8. Основные бизнес-правила

### 8.1. Проекты

Проект создаётся в статусе `draft`.

Автор может редактировать проект только в статусах:

```text
draft
rejected
```

Проект можно отправить на модерацию только если:

1. Проект находится в статусе `draft` или `rejected`.
2. У проекта есть хотя бы один этап.
3. Сумма `amount_limit` всех этапов равна `goal_amount`.
4. Если `campaign_type = reward`, у проекта есть хотя бы одно вознаграждение.
5. Если `campaign_type = donation`, вознаграждения не обязательны.

После отправки на модерацию статус проекта становится `on_review`.

Администратор может:

- одобрить проект: статус становится `published`;
- отклонить проект: статус становится `rejected`;
- заблокировать проект: статус становится `blocked`.

### 8.2. Этапы

У каждого проекта должен быть хотя бы один этап.

Этапы создаются автором до отправки проекта на модерацию.

Статусы этапа:

```text
planned
in_progress
on_review
approved
rejected
overdue
```

Автор может отправить отчёт по этапу, если:

1. Он является автором проекта.
2. Проект опубликован.
3. Этап находится в статусе `planned`, `in_progress` или `rejected`.

После отправки отчёта статус этапа становится `on_review`.

Администратор может проверить отчёт.

Если решение `approved`:

1. Создать запись в `milestone_reviews`.
2. Обновить статус этапа на `approved`.
3. Выполнить учётную операцию `release`.
4. Уменьшить `project_funds.total_reserved` на сумму этапа.
5. Увеличить `project_funds.total_available` на сумму этапа.
6. Создать запись в `fund_ledger` с `operation_type = 'release'`.

Если решение `rejected`:

1. Создать запись в `milestone_reviews`.
2. Обновить статус этапа на `rejected`.
3. Средства не переводить в `available`.

### 8.3. Поддержка проекта

Пользователь может поддержать только проект в статусе `published`.

При поддержке проекта создаётся запись в `pledges`.

Для `donation`-проекта:

```text
reward_id должен быть null
```

Для `reward`-проекта:

```text
reward_id может быть null, если пользователь хочет поддержать без вознаграждения
если reward_id указан, сумма должна быть >= rewards.min_amount
```

После создания поддержки создаётся платёж в mock-платёжном провайдере.

Начальные статусы:

```text
pledges.status = payment_pending
payments.status = pending
```

### 8.4. Платежи

Реальный платёжный провайдер не нужен. Реализовать mock-провайдера.

Mock-провайдер должен:

1. Создавать `provider_payment_id`.
2. Возвращать `payment_url`, который может быть фиктивной ссылкой.
3. Позволять имитировать вебхук об успешном или неуспешном платеже.

При успешном вебхуке `captured`:

1. Проверить идемпотентность через `payment_webhook_events`.
2. В транзакции обновить `payments.status = captured`.
3. Обновить `pledges.status = paid`.
4. Увеличить `project_funds.total_collected` на сумму платежа.
5. Увеличить `project_funds.total_reserved` на сумму платежа.
6. Создать записи в `fund_ledger`:
   - `collect`;
   - `reserve`.
7. Создать уведомление в `notification_outbox`.

Повторный вебхук с тем же `provider_event_id` не должен менять данные повторно.

При вебхуке `failed` или `canceled`:

1. Обновить статус платежа.
2. Обновить статус поддержки на `canceled`.
3. Не менять `project_funds`.

### 8.5. Возвраты

Возврат можно создать только для оплаченной поддержки.

В MVP разрешить возврат, если:

1. Платёж имеет статус `captured`.
2. Поддержка имеет статус `paid`.
3. В проекте есть достаточно `total_reserved`, чтобы выполнить возврат.
4. Деньги ещё не были переведены в `available` через подтверждённый этап.

При успешном возврате:

1. Создать запись в `refunds`.
2. Обновить `payments.status = refunded`.
3. Обновить `pledges.status = refunded`.
4. Уменьшить `project_funds.total_reserved`.
5. Увеличить `project_funds.total_refunded`.
6. Создать запись в `fund_ledger` с `operation_type = 'refund'`.
7. Создать уведомление в `notification_outbox`.

### 8.6. Уведомления

Реальную отправку email можно не реализовывать.

Нужно реализовать outbox-паттерн:

1. При значимых событиях создавать запись в `notification_outbox`.
2. Фоновый worker выбирает записи со статусом `pending`.
3. Worker переводит запись в `processing`.
4. Worker логирует отправку уведомления.
5. Worker переводит запись в `sent`.

События для уведомлений:

```text
project_published
project_rejected
pledge_paid
payment_failed
milestone_submitted
milestone_approved
milestone_rejected
project_update_created
refund_succeeded
```

Для выборки использовать безопасный подход:

```sql
FOR UPDATE SKIP LOCKED
```

## 9. REST API

Все ответы должны быть в JSON.

Ошибки возвращать в формате:

```json
{
  "error": {
    "code": "VALIDATION_ERROR",
    "message": "Описание ошибки"
  }
}
```

Для защищённых маршрутов использовать заголовок:

```http
Authorization: Bearer <token>
```

### 9.1. Auth

#### `POST /api/v1/auth/register`

Регистрация пользователя.

Request:

```json
{
  "email": "user@example.com",
  "password": "password123",
  "display_name": "Иван Иванов"
}
```

Response:

```json
{
  "access_token": "jwt",
  "user": {
    "id": "uuid",
    "email": "user@example.com",
    "display_name": "Иван Иванов",
    "roles": ["backer", "author"]
  }
}
```

#### `POST /api/v1/auth/login`

Вход пользователя.

Request:

```json
{
  "email": "user@example.com",
  "password": "password123"
}
```

Response такой же, как у регистрации.

#### `GET /api/v1/users/me`

Возвращает текущего пользователя.

### 9.2. Categories

#### `GET /api/v1/categories`

Возвращает список категорий.

### 9.3. Projects

#### `GET /api/v1/projects`

Каталог проектов.

Query params:

```text
q
category_id
status
campaign_type
min_goal
max_goal
page
page_size
```

По умолчанию для публичного каталога показывать только `published`.

Response:

```json
{
  "items": [
    {
      "id": "uuid",
      "title": "Название проекта",
      "short_description": "Краткое описание",
      "campaign_type": "reward",
      "goal_amount": 100000,
      "currency": "RUB",
      "status": "published",
      "funds": {
        "total_collected": 15000,
        "total_available": 0,
        "total_reserved": 15000,
        "total_refunded": 0
      }
    }
  ],
  "page": 1,
  "page_size": 20,
  "total": 1
}
```

#### `GET /api/v1/projects/{project_id}`

Детальная карточка проекта.

Вернуть:

1. Основные поля проекта.
2. Категорию.
3. Медиа.
4. Этапы.
5. Вознаграждения.
6. Учёт средств.
7. Обновления проекта.

#### `POST /api/v1/projects`

Создание проекта.

Auth: `author`.

Request:

```json
{
  "title": "Настольная игра",
  "short_description": "Краткое описание",
  "description": "Полное описание проекта",
  "category_id": 2,
  "campaign_type": "reward",
  "currency": "RUB",
  "goal_amount": 100000,
  "start_at": "2026-06-01T00:00:00Z",
  "end_at": "2026-08-01T00:00:00Z"
}
```

Response: созданный проект в статусе `draft`.

При создании проекта сразу создать строку в `project_funds`.

#### `PUT /api/v1/projects/{project_id}`

Редактирование проекта.

Auth: автор проекта.

Разрешено только для статусов `draft` и `rejected`.

#### `POST /api/v1/projects/{project_id}/media`

Добавление медиа.

Auth: автор проекта.

Request:

```json
{
  "media_type": "image",
  "url": "https://example.com/image.jpg",
  "sort_order": 1
}
```

#### `POST /api/v1/projects/{project_id}/submit`

Отправка проекта на модерацию.

Auth: автор проекта.

Проверки:

1. Есть хотя бы один этап.
2. Сумма этапов равна `goal_amount`.
3. Для `reward` есть хотя бы одно вознаграждение.

Response:

```json
{
  "id": "uuid",
  "status": "on_review"
}
```

#### `POST /api/v1/projects/{project_id}/updates`

Публикация обновления проекта.

Auth: автор проекта.

Request:

```json
{
  "title": "Новости проекта",
  "content": "Текст обновления"
}
```

После создания обновления добавить уведомления для пользователей, которые поддержали проект.

### 9.4. Milestones

#### `POST /api/v1/projects/{project_id}/milestones`

Создание этапа.

Auth: автор проекта.

Request:

```json
{
  "title": "Разработка прототипа",
  "description": "Создание первой версии продукта",
  "due_at": "2026-07-01T00:00:00Z",
  "amount_limit": 40000,
  "position": 1
}
```

Статус по умолчанию: `planned`.

#### `PUT /api/v1/projects/{project_id}/milestones/{milestone_id}`

Редактирование этапа.

Auth: автор проекта.

Разрешено только пока проект находится в статусе `draft` или `rejected`.

#### `POST /api/v1/milestones/{milestone_id}/submit`

Отправка отчёта по этапу.

Auth: автор проекта.

Request:

```json
{
  "report_text": "Этап выполнен. Подготовлен прототип.",
  "files": [
    {
      "file_url": "https://example.com/report.pdf",
      "file_type": "document"
    }
  ]
}
```

Response:

```json
{
  "submission_id": "uuid",
  "milestone_status": "on_review"
}
```

### 9.5. Rewards

#### `POST /api/v1/projects/{project_id}/rewards`

Создание вознаграждения.

Auth: автор проекта.

Request:

```json
{
  "title": "Базовое вознаграждение",
  "description": "Доступ к ранней версии продукта",
  "min_amount": 1500,
  "limit_count": 100,
  "delivery_estimate": "2026-09-01"
}
```

Разрешено только для `campaign_type = reward`.

### 9.6. Pledges and Payments

#### `POST /api/v1/projects/{project_id}/pledges`

Создание поддержки и платежа.

Auth: `backer`.

Request:

```json
{
  "amount": 1500,
  "reward_id": "uuid-or-null"
}
```

Response:

```json
{
  "pledge_id": "uuid",
  "payment_id": "uuid",
  "payment_url": "http://localhost:8080/mock-payments/{payment_id}",
  "status": "payment_pending"
}
```

#### `POST /api/v1/payments/{payment_id}/mock-capture`

Тестовый endpoint для имитации успешной оплаты.

Auth: можно разрешить только `admin` или использовать только в dev-режиме.

Endpoint должен создать внутренний mock webhook event и обработать его через тот же сервис, который обрабатывает реальные вебхуки.

#### `POST /api/v1/payments/{payment_id}/mock-fail`

Тестовый endpoint для имитации неуспешной оплаты.

#### `POST /api/v1/integrations/payments/webhook`

Вебхук платёжного провайдера.

Request:

```json
{
  "provider": "mock",
  "provider_event_id": "evt_123",
  "provider_payment_id": "pay_123",
  "event_type": "payment.captured",
  "amount": 1500
}
```

Поддержать события:

```text
payment.captured
payment.failed
payment.canceled
```

### 9.7. Refunds

#### `POST /api/v1/pledges/{pledge_id}/refund`

Инициация возврата.

Auth: `backer` для своей поддержки или `admin`.

Request:

```json
{
  "reason": "Хочу вернуть средства"
}
```

Response:

```json
{
  "refund_id": "uuid",
  "status": "succeeded"
}
```

### 9.8. Admin

#### `GET /api/v1/admin/projects`

Список проектов для администратора.

Auth: `admin`.

Query params:

```text
status
page
page_size
```

#### `POST /api/v1/admin/projects/{project_id}/decision`

Решение по модерации проекта.

Auth: `admin`.

Request:

```json
{
  "decision": "approved",
  "comment": "Проект соответствует требованиям"
}
```

`decision` может быть:

```text
approved
rejected
blocked
```

Поведение:

- `approved` → `projects.status = published`;
- `rejected` → `projects.status = rejected`;
- `blocked` → `projects.status = blocked`.

#### `POST /api/v1/admin/milestones/{milestone_id}/review`

Проверка этапа.

Auth: `admin`.

Request:

```json
{
  "submission_id": "uuid",
  "decision": "approved",
  "comment": "Этап подтверждён"
}
```

При `approved` выполнить операцию `release`.

#### `POST /api/v1/admin/users/{user_id}/block`

Блокировка пользователя.

Auth: `admin`.

#### `POST /api/v1/admin/users/{user_id}/unblock`

Разблокировка пользователя.

Auth: `admin`.

## 10. Транзакции

Обязательно использовать транзакции PostgreSQL в сценариях:

1. Регистрация пользователя и назначение ролей.
2. Создание проекта и строки `project_funds`.
3. Отправка проекта на модерацию.
4. Создание поддержки и платежа.
5. Обработка вебхука платежа.
6. Возврат средств.
7. Подтверждение этапа.
8. Создание обновления проекта и уведомлений.

Особенно важно: обработка успешного платежа и подтверждение этапа не должны оставлять систему в частично обновлённом состоянии.

## 11. Идемпотентность

Реализовать идемпотентность для вебхуков платежей.

Алгоритм:

1. Получить `provider` и `provider_event_id`.
2. В транзакции попытаться вставить запись в `payment_webhook_events`.
3. Если запись уже существует, вернуть `200 OK` без повторной обработки.
4. Найти платёж по `provider_payment_id`.
5. Заблокировать строку платежа через `SELECT ... FOR UPDATE`.
6. Если платёж уже в финальном статусе, не начислять средства повторно.
7. Выполнить обработку события.
8. Зафиксировать транзакцию.

Финальные статусы платежа:

```text
captured
failed
canceled
refunded
```

## 12. Frontend

Сделать минимальный React-интерфейс.

### 12.1. Страницы

Реализовать страницы:

```text
/login
/register
/projects
/projects/:id
/projects/new
/projects/:id/edit
/projects/:id/milestones
/projects/:id/rewards
/projects/:id/updates
/admin/projects
/admin/projects/:id
/admin/milestones
/me
```

### 12.2. Возможности интерфейса

Пользователь должен иметь возможность:

1. Зарегистрироваться.
2. Войти.
3. Просмотреть каталог проектов.
4. Открыть карточку проекта.
5. Поддержать проект.
6. Посмотреть свои данные.

Автор должен иметь возможность:

1. Создать проект.
2. Добавить этапы.
3. Добавить вознаграждения.
4. Отправить проект на модерацию.
5. Опубликовать обновление.
6. Отправить отчёт по этапу.

Администратор должен иметь возможность:

1. Посмотреть проекты на модерации.
2. Одобрить или отклонить проект.
3. Проверить отчёт по этапу.
4. Заблокировать пользователя.

### 12.3. UI-требования

Интерфейс должен быть простым, но понятным.

Для проекта отображать:

1. Название.
2. Краткое описание.
3. Полное описание.
4. Тип кампании.
5. Целевую сумму.
6. Собранную сумму.
7. Зарезервированные средства.
8. Доступные средства.
9. Возвращённые средства.
10. Этапы и их статусы.
11. Вознаграждения.
12. Обновления.

## 13. Валидация

Реализовать серверную валидацию.

Минимальные правила:

```text
email — обязательный, валидный формат
password — минимум 8 символов
title — обязательный
description — обязательный
goal_amount > 0
amount > 0
milestone.amount_limit > 0
due_at — обязательная дата
campaign_type — только reward или donation
currency — трехбуквенный код
```

Не полагаться только на frontend-валидацию.

## 14. Seed-данные

При локальном запуске должны быть доступны:

### 14.1. Администратор

```text
email: admin@example.com
password: admin12345
role: admin
```

Пароль хранить только в виде bcrypt-хэша.

### 14.2. Категории

Создать несколько категорий:

```text
Технологии
Игры
Музыка
Кино
Образование
Социальные проекты
```

### 14.3. Демонстрационные проекты

Создать 2–3 демонстрационных проекта:

1. Reward-based проект с вознаграждениями.
2. Donation-based проект без вознаграждений.
3. Проект в статусе `on_review`.

Для каждого проекта создать этапы так, чтобы сумма этапов равнялась целевой сумме.

## 15. Логирование

Добавить структурированные логи для:

1. Входящих HTTP-запросов.
2. Ошибок API.
3. Регистрации и логина.
4. Создания проекта.
5. Отправки проекта на модерацию.
6. Решения администратора.
7. Создания поддержки.
8. Обработки платежного вебхука.
9. Возвратов.
10. Подтверждения этапов.
11. Работы outbox worker.

Каждому запросу назначать `request_id`.

## 16. OpenAPI

Создать файл:

```text
openapi.yaml
```

Документировать все публичные endpoints.

Минимально указать:

1. Method.
2. Path.
3. Auth requirement.
4. Request body.
5. Response body.
6. Основные ошибки.

## 17. Тестирование

Добавить backend-тесты для ключевой бизнес-логики.

Минимальный набор тестов:

1. Регистрация создаёт пользователя и роли.
2. Нельзя отправить проект на модерацию без этапов.
3. Нельзя отправить проект на модерацию, если сумма этапов не равна `goal_amount`.
4. Reward-проект требует хотя бы одно вознаграждение перед модерацией.
5. Donation-проект не требует вознаграждения.
6. Успешный вебхук `payment.captured` начисляет средства один раз.
7. Повторный вебхук не дублирует начисление.
8. Подтверждение этапа переводит сумму из `reserved` в `available`.
9. Нельзя подтвердить этап, если `reserved` меньше суммы этапа.
10. Возврат уменьшает `reserved` и увеличивает `refunded`.

## 18. Команды запуска

В `README.md` описать команды:

```bash
cp .env.example .env
docker compose up -d
go run ./cmd/api
cd frontend
npm install
npm run dev
```

Также добавить команды:

```bash
go test ./...
goose up
goose down
```

Или аналогичные команды, если используется другой инструмент миграций.

## 19. Критерии готовности MVP

MVP считается готовым, если можно выполнить полный сценарий:

1. Пользователь регистрируется.
2. Пользователь создаёт reward-based проект.
3. Автор добавляет этапы.
4. Автор добавляет вознаграждение.
5. Автор отправляет проект на модерацию.
6. Администратор одобряет проект.
7. Другой пользователь поддерживает проект.
8. Mock-платёж успешно подтверждается.
9. В `project_funds` увеличиваются `total_collected` и `total_reserved`.
10. Автор отправляет отчёт по этапу.
11. Администратор подтверждает этап.
12. В `project_funds` уменьшается `total_reserved` и увеличивается `total_available`.
13. В `fund_ledger` видны операции `collect`, `reserve`, `release`.
14. В `notification_outbox` появляются уведомления.
15. Frontend позволяет пройти основные действия без ручного обращения к базе данных.

## 20. Что не нужно реализовывать в MVP

Не реализовывать:

1. Реальный платёжный провайдер.
2. Реальный вывод средств автору.
3. Реальную отправку email.
4. Загрузку файлов на сервер.
5. Сложную аналитику.
6. KYC через внешний сервис.
7. Микросервисную архитектуру.
8. WebSocket.
9. Мобильное приложение.

Файлы и медиа можно хранить как URL-строки.

## 21. Приоритет разработки

Разрабатывать в таком порядке:

1. Базовая структура проекта.
2. Docker Compose и PostgreSQL.
3. Миграции и seed-данные.
4. Auth и RBAC.
5. CRUD проектов.
6. Этапы.
7. Вознаграждения.
8. Модерация проектов.
9. Поддержки и mock-платежи.
10. Идемпотентная обработка вебхуков.
11. Учёт средств и `fund_ledger`.
12. Отчёты по этапам и проверка администратором.
13. Outbox-уведомления.
14. Frontend.
15. OpenAPI.
16. Тесты.
17. README.

## 22. Требования к качеству кода

Код должен быть понятным и разделённым по слоям.

Не размещать бизнес-логику в HTTP-handlers. Handler должен:

1. Принять request.
2. Провалидировать входные данные.
3. Получить текущего пользователя.
4. Вызвать service.
5. Вернуть response.

Вся бизнес-логика должна находиться в `internal/services`.

Работа с SQL должна находиться в `internal/repositories`.

Статусы и доменные константы вынести в `internal/domain`.

Ошибки бизнес-логики оформлять явно, например:

```go
ErrForbidden
ErrNotFound
ErrValidation
ErrInvalidStatus
ErrInsufficientReservedFunds
ErrDuplicateWebhookEvent
```

## 23. Ожидаемый результат

В результате должен получиться репозиторий с рабочим MVP:

```text
backend/
frontend/
docker-compose.yml
README.md
openapi.yaml
.env.example
```

Приложение должно запускаться локально и демонстрировать основную идею: краудфандинговая платформа с обязательной этапностью, поддержкой reward/donation проектов, mock-платежами, учётной моделью средств и административным подтверждением этапов.
