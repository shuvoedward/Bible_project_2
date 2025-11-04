# Architecture

## System Overview
![Architecture Diagram](./Bible%20app%20system%20overivew.png)

**Purpose**: This diagram illustrates the high-level architecture of the Bible note-taking application, showing how the client communicates with the API server and how the server interacts with various backend services for data persistence, caching, and file storage.

## Technology Stack

### Backend
- **Language**: Go 1.21+
- **Router**: httprouter (lightweight, fast routing)
- **Database**: PostgreSQL 14+
- **Cache**: Redis 7+
- **Storage**: AWS S3 (for future file attachments)

### Development Tools
- **Migration**: golang-migrate/migrate
- **Testing**: Go standard testing package
- **Logging**: Structured JSON logging

## Component Details

### API Server (Go)
The API server is built with Go for high performance and simple concurrency handling.

**Key Features**:
- RESTful API design
- JSON request/response format
- Graceful shutdown with cleanup
- Request timeout handling (10s default)
- Structured logging with request IDs

**Middleware Chain** (executed in order):
1. **Logging** - Records all requests with timing
2. **Panic Recovery** - Catches panics and returns 500 errors
3. **CORS** - Cross-origin resource sharing configuration
4. **Rate Limiting** - Prevents abuse (2 req/sec per IP)
5. **Authentication** - Validates API tokens

**Connection Pooling**:
- Max open connections: 25
- Max idle connections: 5
- Connection max lifetime: 15 minutes
- Connection max idle time: 5 minutes

### Database Layer

**PostgreSQL Configuration**:
- Version: 14 or higher
- Connection pooling managed by pq driver
- Prepared statements for frequently used queries
- Transaction support for data consistency
- Indexes on frequently queried columns

**Migration Strategy**:
- Use golang-migrate for version control
- Sequential numbered migrations (001_create_users.up.sql)
- Both up and down migrations for rollback capability
- Migrations run automatically on application start in development
- Manual migration required in production

### Caching Strategy

**Redis Configuration**:
- Distributed caching layer
- Reduces database load for frequently accessed data
- Supports horizontal scaling

**Cache Patterns**:

| Data Type | Key Pattern | TTL | Invalidation |
|-----------|-------------|-----|--------------|
| User Sessions | `user:token:{token}` | 24 hours | On logout |
| User Profiles | `user:profile:{user_id}` | 1 hour | On profile update |


**Cache Invalidation Strategies**:
1. **Time-based (TTL)** - Automatic expiration for stale data tolerance
2. **Event-based** - Immediate invalidation on data mutations
3. **Lazy Loading** - Populate cache on cache miss

**Cache-Aside Pattern**:
```
1. Client requests data
2. Check Redis cache
3. If HIT: return cached data
4. If MISS: query PostgreSQL
5. Store result in Redis
6. Return data to client
```

### Storage Layer (S3)

**Purpose**: Future feature for storing file attachments (images, PDFs, audio notes)

**Configuration**:
- Bucket per environment (dev, staging, prod)
- Object key pattern: `{user_id}/{note_id}/{filename}`
- Pre-signed URLs for secure downloads (expires in 15 minutes)
- Server-side encryption enabled

## Data Models

### Core Entities

**Users**
```
- id: bigserial (PK)
- email: varchar(255) (unique, indexed)
- password_hash: bytea
- name: varchar(500)
- is_active: boolean (default: false)
- created_at: timestamp
- version: integer (optimistic locking)
```

**Tokens**
```
- hash: bytea (PK)
- user_id: bigint (FK to users.id)
- expiry: timestamp
- scope: varchar(50) (activation, authentication)
```

**Notes**
```
- id: bigserial (PK)
- user_id: bigint (FK to users.id, indexed)
- title: varchar(500)
- content: text
- bible_reference: jsonb (book, chapter, verse_start, verse_end)
- created_at: timestamp
- updated_at: timestamp
- version: integer (optimistic locking)
```

**Tags**
```
- id: bigserial (PK)
- user_id: bigint (FK to users.id)
- name: varchar(100)
- color: varchar(7) (hex color code)
- created_at: timestamp
```

**Notes_Tags** (junction table)
```
- note_id: bigint (FK to notes.id)
- tag_id: bigint (FK to tags.id)
- PRIMARY KEY (note_id, tag_id)
```

**Bible_Verses**
```
- id: bigserial (PK)
- book: varchar(50) (indexed)
- chapter: integer (indexed)
- verse: integer
- text: text
- version: varchar(50) (e.g., "KJV", "NIV")
- UNIQUE (book, chapter, verse, version)
```

## Authentication Flow
![Authentication flow](./Bible%20app%20Authentication.png)

**Flow Steps**:

1. **Registration** - User submits email, password, name
   - Server hashes password with bcrypt (cost 12)
   - Creates inactive user account
   - Generates random activation token (expires in 3 days)
   - Sends activation email asynchronously
   - Returns 202 Accepted

2. **Activation** - User clicks email link
   - Server verifies token validity and expiration
   - Activates user account
   - Deletes activation token
   - Returns 200 OK

3. **Login** - User submits credentials
   - Server verifies email and password
   - Generates random authentication token (expires in 24 hours)
   - Stores token in database and Redis cache
   - Returns token to client

4. **Authenticated Requests** - Client includes token in Authorization header
   - Server checks Redis cache for token (fast path)
   - If cache miss, queries database (slow path)
   - Validates token expiration
   - Extracts user_id for request context
   - Processes request with user context

## Security Measures

### Password Security
- **Hashing Algorithm**: bcrypt with cost factor 12
- **Salt**: Automatically handled by bcrypt (per-password random salt)
- **Minimum Length**: 8 characters (enforced at API level)
- **Password Strength**: Must contain letters and numbers (optional special chars)

### Token Security
- **Generation**: Cryptographically secure random bytes (32 bytes)
- **Storage**: SHA-256 hash stored in database (original token never stored)
- **Transmission**: HTTPS only in production
- **Expiration**: 
  - Activation tokens: 3 days
  - Authentication tokens: 24 hours
- **Invalidation**: Tokens deleted from DB and cache on logout

### API Security
- **HTTPS Only**: Enforced in production (redirects HTTP to HTTPS)
- **Rate Limiting**: 2 requests/second per IP address
- **CORS**: Configured allowed origins (no wildcard in production)
- **SQL Injection Prevention**: Prepared statements and parameterized queries
- **Input Validation**: All inputs validated before processing
- **Output Encoding**: JSON responses properly encoded
- **Request Size Limits**: Maximum request body size enforced

### Authentication Headers
```
Authorization: Bearer <token>
```

**Token sent with every authenticated request**:
- Extracted from Authorization header
- Validated against database/cache
- Request rejected with 401 if invalid/expired

### Data Access Control
- Users can only access their own notes and tags
- All database queries filtered by user_id
- Authorization check middleware on protected routes
- Resource ownership verification before mutations

## Error Handling

### HTTP Status Codes
- **200 OK** - Successful GET, PUT, DELETE
- **201 Created** - Successful POST
- **202 Accepted** - Async operation initiated
- **400 Bad Request** - Validation errors
- **401 Unauthorized** - Missing or invalid authentication
- **403 Forbidden** - Valid auth but insufficient permissions
- **404 Not Found** - Resource doesn't exist
- **409 Conflict** - Duplicate resource or version mismatch
- **422 Unprocessable Entity** - Validation failures
- **429 Too Many Requests** - Rate limit exceeded
- **500 Internal Server Error** - Unexpected server errors

### Error Response Format
```json
{
  "error": {
    "message": "Human-readable error message",
    "code": "VALIDATION_ERROR",
    "request_id": "req_abc123",
    "details": {
      "field": "email",
      "issue": "already registered"
    }
  }
}
```

### Error Handling Strategy
1. **Validation Errors** - Return 400/422 with specific field errors
2. **Database Errors** - Log detailed error, return generic 500 to client
3. **Panic Recovery** - Catch panics, log stack trace, return 500
4. **Request Timeouts** - Return 408 after timeout threshold
5. **Rate Limit** - Return 429 with Retry-After header

### Logging
- **Structured Logging**: JSON format for easy parsing
- **Log Levels**: DEBUG, INFO, WARN, ERROR
- **Request Context**: Every log includes request_id
- **Sensitive Data**: Passwords and tokens never logged
- **Error Details**: Full stack traces for 500 errors in logs

### Request ID Tracking
- Unique UUID generated for each request
- Included in response headers: `X-Request-ID`
- Included in all log entries
- Returned in error responses for debugging

## Performance Considerations

### Database Optimization
- **Indexes**: On user_id, email, created_at, bible_reference
- **Connection Pooling**: Reuse connections efficiently
- **Prepared Statements**: Pre-compiled queries for performance
- **Query Timeouts**: Prevent long-running queries (5s timeout)
- **EXPLAIN ANALYZE**: Used during development to optimize slow queries

### Caching Benefits
- **Reduced DB Load**: 70-80% of reads served from cache
- **Faster Response Times**: Sub-millisecond cache reads
- **Scalability**: Handle more concurrent users

### API Response Times (Target)
- **Cached Reads**: < 50ms
- **Uncached Reads**: < 200ms
- **Writes**: < 300ms
- **Authentication**: < 100ms

## Scalability Considerations

### Horizontal Scaling
- **Stateless API**: No session state in server memory
- **Load Balancer**: Multiple API server instances behind load balancer
- **Database**: Master-replica setup for read scaling
- **Redis**: Redis cluster for cache scaling

### Vertical Scaling
- **API Server**: Can handle 1000+ req/sec per instance
- **PostgreSQL**: Can scale to millions of notes
- **Redis**: Can handle 100k+ operations/sec

## Deployment Architecture (Future)

```
                    ┌──────────────┐
                    │ Load Balancer│
                    └──────┬───────┘
                           │
         ┌─────────────────┼─────────────────┐
         │                 │                 │
    ┌────▼────┐       ┌────▼────┐      ┌────▼────┐
    │  API    │       │  API    │      │  API    │
    │ Server 1│       │ Server 2│      │ Server 3│
    └────┬────┘       └────┬────┘      └────┬────┘
         │                 │                 │
         └─────────────────┼─────────────────┘
                           │
              ┌────────────┼────────────┐
              │            │            │
         ┌────▼────┐  ┌────▼────┐  ┌───▼────┐
         │PostgreSQL│  │  Redis  │  │   S3   │
         │ Primary  │  │ Cluster │  │ Bucket │
         └────┬────┘  └─────────┘  └────────┘
              │
         ┌────▼────┐
         │PostgreSQL│
         │ Replica │
         └─────────┘
```

## Development Workflow

### Local Development
1. Start PostgreSQL and Redis with Docker Compose
2. Run migrations: `make migrate-up`
3. Start API server: `make run`
4. API available at: `http://localhost:4000`

### Testing
- Unit tests for business logic
- Integration tests for database operations
- API endpoint tests with httptest
- Test coverage target: 80%+

### Code Organization
```
/cmd
  /api          - Application entry point
/internal
  /data         - Database models and queries
  /validator    - Input validation
/migrations     - Database migrations
/docs           - Documentation
```

## Monitoring and Observability (Future)

### Metrics
- Request rate and latency
- Error rate by endpoint
- Database query performance
- Cache hit/miss ratio
- Active connections

### Health Checks
- `GET /v1/healthcheck` - Returns system status
- Database connectivity check
- Redis connectivity check
- Disk space check

### Alerting
- High error rate (> 5%)
- Slow response times (p95 > 500ms)
- Database connection pool exhaustion
- Cache unavailability

## Future Enhancements

2. **Real-time Sync** - WebSocket support for live updates
4. **Export Features** - Export notes to PDF/Markdown
5. **Collaborative Notes** - Share notes with other users
7. **Offline Support** - Progressive Web App with service workers
8. **Audio Notes** - Voice recording and transcription
9. **Bible Translations** - Support for multiple Bible versions
10. **Social Features** - Follow users, public notes, comments