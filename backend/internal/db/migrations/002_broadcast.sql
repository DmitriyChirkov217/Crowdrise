-- +goose Up
create table broadcast (
    id uuid primary key,
    project_id uuid not null references projects(id) on delete cascade,
    status varchar(30) not null,
    check (status in ('scheduled', 'live', 'ended'))
);

create table broadcast_chat_files (
    id uuid primary key,
    broadcast_id uuid not null references broadcast(id) on delete cascade,
    file_url text not null
);

create index idx_broadcast_project_id on broadcast(project_id);
create index idx_broadcast_chat_files_broadcast_id on broadcast_chat_files(broadcast_id);

-- +goose Down
drop table if exists broadcast_chat_files;
drop table if exists broadcast;
