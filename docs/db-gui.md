# Database GUI

pgAdmin is available for local database inspection.

## Open pgAdmin

```text
http://localhost:5050
```

Login:

```text
Email: admin@example.com
Password: chess
```

The PostgreSQL server is pre-registered as `Chess PostgreSQL`.

If pgAdmin asks for the database password:

```text
Password: chess
```

## View Tables

1. Open `Servers`
2. Open `Chess PostgreSQL`
3. Open `Databases > chess > Schemas > public > Tables`
4. Right-click `users` or `games`
5. Select `View/Edit Data > All Rows`

## Security

pgAdmin is bound to `127.0.0.1:5050`, so it is available only from this machine. Do not expose pgAdmin through the public tunnel.
