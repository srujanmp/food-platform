docker compose up --build

OR

docker compose -f docker-compose.infra.yml ps
docker compose -f docker-compose.infra.yml up -d
docker compose -f docker-compose.infra.yml down
cd auth-service
go mod tidy               //This downloads all packages from your go.mod and generates go.sum
go run ./internal/cmd/main.go



go work init
go mod init github.com/food-platform/auth-service
go get "module"
cd ..
ls

# API TESTING

# 1. Health check
curl http://localhost:8001/api/v1/auth/health

# 2. Register a user
curl -X POST http://localhost:8001/api/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{"name":"Srujan","email":"srujan@test.com","password":"password123","phone":"+919999999999"}'

# 3. Login (copy the access_token from response)
curl -X POST http://localhost:8001/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"srujan@test.com","password":"password123"}'

# 4. Refresh token (use refresh_token from login response)
curl -X POST http://localhost:8001/api/v1/auth/refresh \
  -H "Content-Type: application/json" \
  -d '{"refresh_token":"<paste_refresh_token_here>"}'

# 5. Logout (use access_token as Bearer, refresh_token in body)
curl -X POST http://localhost:8001/api/v1/auth/logout \
  -H "Authorization: Bearer <paste_access_token_here>" \
  -H "Content-Type: application/json" \
  -d '{"refresh_token":"<paste_refresh_token_here>"}'