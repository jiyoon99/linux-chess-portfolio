package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		fatalf("DATABASE_URL is required")
	}

	// 컨테이너 안에서는 SQL 파일이 없을 수 있어 기본 migration을 바이너리에 함께 넣어 둔다.
	path := os.Getenv("MIGRATION_FILE")
	sql := []byte(defaultMigration)
	if path != "" {
		var err error
		sql, err = os.ReadFile(path)
		if err != nil {
			fatalf("read migration file: %v", err)
		}
	} else if _, err := os.Stat(filepath.Join("infra", "sql", "001_games.sql")); err == nil {
		path = filepath.Join("infra", "sql", "001_games.sql")
		sql, err = os.ReadFile(path)
		if err != nil {
			fatalf("read migration file: %v", err)
		}
	} else {
		path = "embedded default migration"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// migration은 배포 절차에서 명시적으로 실행하는 운영 명령이다.
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		fatalf("connect database: %v", err)
	}
	defer pool.Close()

	if _, err := pool.Exec(ctx, string(sql)); err != nil {
		fatalf("apply migration: %v", err)
	}

	fmt.Printf("applied migration %s\n", path)
}

func fatalf(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}

const defaultMigration = `
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
`
