# BG-API - Go Boilerplate

A production-ready Go API boilerplate with Fiber framework.

## Features

1. **Authentication**
   - Login with access token
   - Genesis account (initial superadmin)
   - Refresh token support
   - Role-based access control

2. **Database Support**
   - MongoDB
   - PostgreSQL
   - MySQL
   - SQLite

3. **File Upload**
   - Cloudinary CDN integration
   - File type validation
   - File size limit

4. **Middleware**
   - JWT Authentication guard
   - Role-based authorization
   - CORS support

5. **Utilities**
   - Response standardization
   - Pagination helper
   - Slug generator
   - Cron job support

## Quick Start

### 1. Copy environment file
```powershell
copy .env.example .env
```

### 2. Edit `.env` with your configuration

### 3. Install dependencies
```powershell
go mod tidy
```

### 4. Run the server

**Standard run:**
```powershell
go run cmd/server/main.go
```

**With hot reload:**
```powershell
# Install Air first
go install github.com/air-verse/air@latest

# Run with hot reload
air
```

## Project Structure

```
bg-go/
├── cmd/
│   └── server/
│       └── main.go          # Entry point
├── internal/
│   ├── config/              # Configuration
│   ├── database/            # Database connections
│   ├── handlers/            # HTTP handlers
│   │   ├── auth.go          # Auth handlers
│   │   └── blueprint.go     # Handler template
│   ├── lib/
│   │   ├── cloudinary/      # CDN integration
│   │   ├── cron/            # Cron jobs
│   │   ├── crypt/           # Password hashing
│   │   ├── file/            # File utilities
│   │   ├── jwt/             # JWT handling
│   │   ├── response/        # Response helpers
│   │   └── utils/           # General utilities
│   ├── middleware/
│   │   ├── auth.go          # Auth middleware
│   │   └── cors.go          # CORS middleware
│   ├── models/              # Data models
│   └── routes/
│       └── routes.go        # Route definitions
├── .air.toml                # Air configuration
├── .env.example             # Environment template
├── go.mod                   # Go module
└── README.md
```

## API Endpoints

### Auth
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/auth/genesis` | Create initial superadmin |
| POST | `/api/v1/auth/login` | Login |
| POST | `/api/v1/auth/refresh` | Refresh token |
| GET | `/api/v1/auth/me` | Get current user (Auth) |
| GET | `/api/v1/auth/users` | List users (Admin) |
| POST | `/api/v1/auth/register` | Create user (Admin) |

## Creating New Endpoints

Use the blueprint at `internal/handlers/blueprint.go` as a template:

1. Copy `blueprint.go` to your new handler file
2. Replace `Blueprint` with your entity name
3. Implement the CRUD methods
4. Add routes to `internal/routes/routes.go`

## Response Format

### Success with data
```json
{
  "status": 200,
  "message": "success",
  "data": { ... }
}
```

### Success with pagination
```json
{
  "status": 200,
  "message": "success",
  "data": [ ... ],
  "pagination": {
    "current_page": 1,
    "total_pages": 10,
    "total_items": 100,
    "per_page": 10,
    "has_next": true,
    "has_prev": false
  }
}
```

### Error
```json
{
  "status": 400,
  "message": "Error description"
}
```

## Database Configuration

Set `DB_DRIVER` in `.env` to choose database:
- `mongodb`
- `postgres`
- `mysql`
- `sqlite`

## License

MIT
