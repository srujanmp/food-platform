go work use ./user-service

# user-service · Port 8002

Manages user profiles and delivery addresses. Reads JWT tokens issued by auth-service.

## Structure

```
user-service/
├── internal/
│   ├── cmd/main.go          ← entry point, wiring, graceful shutdown
│   ├── config/config.go     ← env vars, DB + Redis connect
│   ├── models/models.go     ← GORM structs + request/response DTOs
│   ├── repository/repo.go   ← DB access via interfaces
│   ├── service/service.go   ← business logic + ownership checks
│   ├── handlers/handlers.go ← Gin handlers
│   └── middleware/
│       └── middleware.go    ← Auth (JWT), RBAC, RateLimit, Logger
├── migrations/              ← versioned SQL (golang-migrate)
├── Dockerfile
├── go.mod
└── .env.example
```

## API

| Method | Endpoint | Auth | Description |
|--------|----------|------|-------------|
| GET | /api/v1/users/:id | Bearer | Get profile |
| PUT | /api/v1/users/:id | Bearer+Owner | Update name/avatar |
| DELETE | /api/v1/users/:id | Bearer+Owner | Soft-delete account |
| GET | /api/v1/users/:id/addresses | Bearer+Owner | List addresses |
| POST | /api/v1/users/:id/addresses | Bearer+Owner | Add address |
| PUT | /api/v1/users/addresses/:addressId | Bearer+Owner | Update address |
| DELETE | /api/v1/users/addresses/:addressId | Bearer+Owner | Delete address |
| GET | /api/v1/users/health | None | Health check |

## Run locally

```bash
# Copy env
cp .env.example .env

# Start infra (from repo root)
docker compose -f docker-compose.infra.yml up -d

# Run service
go run ./internal/cmd/main.go
```

## Add to go.work (from repo root)

```bash
go work use ./user-service
```

## Production migrations (instead of AutoMigrate)

```bash
migrate -path ./migrations \
  -database "postgres://postgres:postgres@localhost:5433/user_db?sslmode=disable" up
```

## Notes

- `auth_id` in `profiles` is a logical reference to `auth_db.users.id` — **no FK constraint** (cross-DB).
- Ownership checks are done in the service layer: callers can only modify their own data; ADMINs bypass this.
- `EnsureProfile` can be called by auth-service post-registration via internal API (optional future integration).