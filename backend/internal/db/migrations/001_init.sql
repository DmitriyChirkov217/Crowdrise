-- +goose Up
create extension if not exists pgcrypto;

create table users (
    id uuid primary key,
    email varchar(255) not null unique,
    password_hash varchar(255) not null,
    display_name varchar(255) not null,
    phone varchar(50),
    is_blocked boolean not null default false,
    created_at timestamp not null default now(),
    updated_at timestamp not null default now()
);

create table roles (
    id smallserial primary key,
    code varchar(50) not null unique,
    name varchar(255) not null
);

create table user_roles (
    user_id uuid not null references users(id) on delete cascade,
    role_id smallint not null references roles(id) on delete cascade,
    created_at timestamp not null default now(),
    primary key (user_id, role_id)
);

create table categories (
    id serial primary key,
    name varchar(255) not null unique
);

create table projects (
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
    updated_at timestamp not null default now(),
    check (campaign_type in ('reward', 'donation')),
    check (goal_amount > 0),
    check (status in ('draft', 'on_review', 'rejected', 'published', 'completed', 'blocked', 'canceled'))
);

create table project_media (
    id uuid primary key,
    project_id uuid not null references projects(id) on delete cascade,
    media_type varchar(20) not null,
    url text not null,
    sort_order int not null default 0,
    check (media_type in ('image', 'video', 'document'))
);

create table milestones (
    id uuid primary key,
    project_id uuid not null references projects(id) on delete cascade,
    title varchar(255) not null,
    description text not null,
    due_at timestamp not null,
    amount_limit numeric(14,2) not null,
    position int not null,
    status varchar(30) not null,
    created_at timestamp not null default now(),
    updated_at timestamp not null default now(),
    check (amount_limit > 0),
    check (position > 0),
    check (status in ('planned', 'in_progress', 'on_review', 'approved', 'rejected', 'overdue')),
    unique (project_id, position)
);

create table milestone_submissions (
    id uuid primary key,
    milestone_id uuid not null references milestones(id) on delete cascade,
    author_id uuid not null references users(id),
    report_text text not null,
    submitted_at timestamp not null default now()
);

create table milestone_submission_files (
    id uuid primary key,
    submission_id uuid not null references milestone_submissions(id) on delete cascade,
    file_url text not null,
    file_type varchar(50) not null
);

create table milestone_reviews (
    id uuid primary key,
    submission_id uuid not null references milestone_submissions(id) on delete cascade,
    admin_id uuid not null references users(id),
    decision varchar(20) not null,
    comment text,
    reviewed_at timestamp not null default now(),
    check (decision in ('approved', 'rejected'))
);

create table rewards (
    id uuid primary key,
    project_id uuid not null references projects(id) on delete cascade,
    title varchar(255) not null,
    description text not null,
    min_amount numeric(14,2) not null,
    limit_count int,
    delivery_estimate date,
    created_at timestamp not null default now(),
    updated_at timestamp not null default now(),
    check (min_amount > 0),
    check (limit_count is null or limit_count > 0)
);

create table pledges (
    id uuid primary key,
    project_id uuid not null references projects(id),
    backer_id uuid not null references users(id),
    reward_id uuid references rewards(id),
    amount numeric(14,2) not null,
    status varchar(30) not null,
    created_at timestamp not null default now(),
    updated_at timestamp not null default now(),
    check (amount > 0),
    check (status in ('created', 'payment_pending', 'paid', 'canceled', 'refunded'))
);

create table payments (
    id uuid primary key,
    pledge_id uuid not null references pledges(id),
    provider varchar(50) not null,
    provider_payment_id varchar(255) not null unique,
    status varchar(30) not null,
    amount numeric(14,2) not null,
    created_at timestamp not null default now(),
    updated_at timestamp not null default now(),
    check (amount > 0),
    check (status in ('created', 'pending', 'captured', 'failed', 'canceled', 'refunded'))
);

create table payment_webhook_events (
    id uuid primary key,
    provider varchar(50) not null,
    provider_event_id varchar(255) not null,
    payment_id uuid references payments(id),
    event_type varchar(100) not null,
    payload jsonb not null,
    processed_at timestamp not null default now(),
    unique (provider, provider_event_id)
);

create table refunds (
    id uuid primary key,
    payment_id uuid not null references payments(id),
    provider_refund_id varchar(255) unique,
    status varchar(30) not null,
    amount numeric(14,2) not null,
    reason varchar(255),
    created_at timestamp not null default now(),
    updated_at timestamp not null default now(),
    check (amount > 0),
    check (status in ('created', 'pending', 'succeeded', 'failed'))
);

create table project_funds (
    project_id uuid primary key references projects(id) on delete cascade,
    total_collected numeric(14,2) not null default 0,
    total_refunded numeric(14,2) not null default 0,
    total_available numeric(14,2) not null default 0,
    total_reserved numeric(14,2) not null default 0,
    updated_at timestamp not null default now(),
    check (total_collected >= 0),
    check (total_refunded >= 0),
    check (total_available >= 0),
    check (total_reserved >= 0)
);

create table fund_ledger (
    id uuid primary key,
    project_id uuid not null references projects(id),
    operation_type varchar(30) not null,
    amount numeric(14,2) not null,
    reference_type varchar(50) not null,
    reference_id uuid not null,
    created_at timestamp not null default now(),
    check (amount > 0),
    check (operation_type in ('collect', 'reserve', 'release', 'refund')),
    check (reference_type in ('payment', 'refund', 'milestone'))
);

create table project_updates (
    id uuid primary key,
    project_id uuid not null references projects(id) on delete cascade,
    author_id uuid not null references users(id),
    title varchar(255) not null,
    content text not null,
    created_at timestamp not null default now()
);

create table notification_outbox (
    id uuid primary key,
    user_id uuid not null references users(id),
    event_type varchar(100) not null,
    payload jsonb not null,
    status varchar(30) not null,
    attempts int not null default 0,
    created_at timestamp not null default now(),
    updated_at timestamp not null default now(),
    check (status in ('pending', 'processing', 'sent', 'failed'))
);

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

insert into roles (code, name) values
    ('backer', 'Backer'),
    ('author', 'Author'),
    ('admin', 'Admin');

insert into categories (name) values
    ('Технологии'),
    ('Игры'),
    ('Музыка'),
    ('Кино'),
    ('Образование'),
    ('Социальные проекты');

insert into users (id, email, password_hash, display_name) values
    ('00000000-0000-0000-0000-000000000001', 'admin@example.com', '$2a$10$3P.Y1sHasju3ekM2pdf2Yu/TE1nNExKf2RwOlio99OgH/fv/L5RDa', 'Admin');

insert into user_roles (user_id, role_id)
select '00000000-0000-0000-0000-000000000001', id from roles where code = 'admin';

insert into users (id, email, password_hash, display_name) values
    ('00000000-0000-0000-0000-000000000002', 'author@example.com', '$2a$10$3P.Y1sHasju3ekM2pdf2Yu/TE1nNExKf2RwOlio99OgH/fv/L5RDa', 'Demo Author'),
    ('00000000-0000-0000-0000-000000000003', 'backer@example.com', '$2a$10$3P.Y1sHasju3ekM2pdf2Yu/TE1nNExKf2RwOlio99OgH/fv/L5RDa', 'Demo Backer');

insert into user_roles (user_id, role_id)
select u.id, r.id
from users u
join roles r on r.code in ('author', 'backer')
where u.id in ('00000000-0000-0000-0000-000000000002', '00000000-0000-0000-0000-000000000003');

insert into projects (id, author_id, title, short_description, description, category_id, campaign_type, currency, goal_amount, status) values
    ('10000000-0000-0000-0000-000000000001', '00000000-0000-0000-0000-000000000002', 'Модульный конструктор роботов', 'Образовательный набор для школьников', 'Reward-based проект с наборами деталей и ранним доступом к учебным материалам.', 1, 'reward', 'RUB', 120000, 'published'),
    ('10000000-0000-0000-0000-000000000002', '00000000-0000-0000-0000-000000000002', 'Музыкальная студия для подростков', 'Бесплатная студия звукозаписи в районе', 'Donation-проект без вознаграждений, направленный на социальную поддержку.', 6, 'donation', 'RUB', 80000, 'published'),
    ('10000000-0000-0000-0000-000000000003', '00000000-0000-0000-0000-000000000002', 'Настольная игра про Урал', 'Семейная стратегия с локальным фольклором', 'Проект ожидает модерации администратора.', 2, 'reward', 'RUB', 60000, 'on_review');

insert into project_funds (project_id, total_collected, total_reserved) values
    ('10000000-0000-0000-0000-000000000001', 25000, 25000),
    ('10000000-0000-0000-0000-000000000002', 10000, 10000),
    ('10000000-0000-0000-0000-000000000003', 0, 0);

insert into milestones (id, project_id, title, description, due_at, amount_limit, position, status) values
    ('20000000-0000-0000-0000-000000000001', '10000000-0000-0000-0000-000000000001', 'Прототип', 'Собрать рабочий прототип', now() + interval '30 days', 50000, 1, 'planned'),
    ('20000000-0000-0000-0000-000000000002', '10000000-0000-0000-0000-000000000001', 'Партия деталей', 'Заказать первую партию', now() + interval '60 days', 70000, 2, 'planned'),
    ('20000000-0000-0000-0000-000000000003', '10000000-0000-0000-0000-000000000002', 'Ремонт помещения', 'Подготовить комнату студии', now() + interval '45 days', 30000, 1, 'planned'),
    ('20000000-0000-0000-0000-000000000004', '10000000-0000-0000-0000-000000000002', 'Оборудование', 'Купить микрофоны и ноутбук', now() + interval '80 days', 50000, 2, 'planned'),
    ('20000000-0000-0000-0000-000000000005', '10000000-0000-0000-0000-000000000003', 'Иллюстрации', 'Подготовить арт и карточки', now() + interval '30 days', 25000, 1, 'planned'),
    ('20000000-0000-0000-0000-000000000006', '10000000-0000-0000-0000-000000000003', 'Печать тестовой партии', 'Напечатать 50 коробок', now() + interval '70 days', 35000, 2, 'planned');

insert into rewards (id, project_id, title, description, min_amount, limit_count, delivery_estimate) values
    ('30000000-0000-0000-0000-000000000001', '10000000-0000-0000-0000-000000000001', 'Ранний набор', 'Первый набор конструктора', 3000, 100, current_date + 120),
    ('30000000-0000-0000-0000-000000000002', '10000000-0000-0000-0000-000000000003', 'Коробка игры', 'Экземпляр настольной игры', 2500, 200, current_date + 100);

-- +goose Down
drop table if exists notification_outbox;
drop table if exists project_updates;
drop table if exists fund_ledger;
drop table if exists project_funds;
drop table if exists refunds;
drop table if exists payment_webhook_events;
drop table if exists payments;
drop table if exists pledges;
drop table if exists rewards;
drop table if exists milestone_reviews;
drop table if exists milestone_submission_files;
drop table if exists milestone_submissions;
drop table if exists milestones;
drop table if exists project_media;
drop table if exists projects;
drop table if exists categories;
drop table if exists user_roles;
drop table if exists roles;
drop table if exists users;
