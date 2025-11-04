# API Documentation

## Quick Start

**Base URL**: `http://localhost:4000/v1`

**Full Documentation**: [Swagger UI](http://localhost:4000/swagger)

## Authentication

All protected endpoints require a Bearer token:
```bash
Authorization: Bearer YOUR_TOKEN_HERE
```

## Quick Examples

### Register and Login
```bash
# Register
curl -X POST http://localhost:4000/v1/users \
  -H "Content-Type: application/json" \
  -d '{"name":"John","email":"john@example.com","password":"pass123"}'

# Login
curl -X POST http://localhost:4000/v1/tokens/authentication \
  -H "Content-Type: application/json" \
  -d '{"email":"john@example.com","password":"pass123"}'
```

### Create a Note
```bash
curl -X POST http://localhost:4000/v1/notes \
  -H "Authorization: Bearer TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"title":"Faith","content":"...","tags":["prayer"]}'
```

## Rate Limits
- 2 requests/second per IP
- Tokens expire after 24 hours

For complete endpoint documentation, see [Swagger UI](http://localhost:4000/swagger).