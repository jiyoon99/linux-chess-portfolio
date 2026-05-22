create table if not exists users (
  id text primary key,
  username text not null unique,
  password_hash text not null,
  is_admin boolean not null default false,
  created_at timestamptz not null default now()
);

create table if not exists games (
  id text primary key,
  mode text not null,
  room_code text,
  moves text[] not null,
  final_fen text not null,
  outcome text not null,
  method text not null,
  winner text,
  ai_level text,
  ai_engine text,
  white_user_id text,
  white_username text,
  black_user_id text,
  black_username text,
  started_at timestamptz not null,
  ended_at timestamptz not null default now()
);

alter table games add column if not exists white_user_id text;
alter table games add column if not exists white_username text;
alter table games add column if not exists black_user_id text;
alter table games add column if not exists black_username text;
alter table users add column if not exists is_admin boolean not null default false;
