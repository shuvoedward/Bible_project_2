# Bible Notes API

## Overview

Bible Notes API is a RESTful API designed to support Bible study, personal annotations, verse highlighting, and note management. It provides programmatic access to Bible text, passage lookup, highlights, and user notes, making it suitable for integration with web and mobile Bible study applications.

**Note:** This is a learning project where I intentionally built infrastructure components (job scheduler, caching layer) from scratch to understand the underlying concepts rather than using off-the-shelf libraries.


## Why This Project?

Most Bible study apps overwhelm users with pre-packaged resources—commentaries, 
reading plans, devotionals—that don't fit everyone's study style. **I wanted 
to build something different**: a focused note-taking platform where users 
create and organize their own study materials.

### The Problem
- Existing apps: Feature-rich but cluttered, one-size-fits-all resources
- Students and scholars: Need flexible, personalized study tools
- No easy way to combine highlights, notes, cross-references in one place

### My Solution
A minimalist Bible study API that prioritizes **user-generated content**:
- Personal notes tied to specific verses
- Custom highlights with color coding
- Cross-reference linking between passages
- Future: Collaborative study with friends, custom tags, prayer tracking

This is the app I wish I had for my own Bible study—clean, focused, and 
built around how **I** want to study, not how someone else thinks I should

## What I Learned

This project pushed me into real-world backend development:

- **PostgreSQL**: Schema design, full-text search optimization, GIN indexes
- **Redis**: Caching strategies, token storage, TTL management
- **API Design**: RESTful principles, authentication flows, versioning
- **Production Skills**: Rate limiting, email integration (SMTP), cloud storage (S3)
- **Concurrency Patterns**: Built a custom job scheduler with worker pools, min-heap priority queue, and exponential backoff retry
- **High-Performance Caching**: Implemented Otter cache with singleflight pattern for request deduplication


## Features

- Retrieve the text of any Bible chapter, specific verse ranges, or search for verses.
- Take notes on Bible passages with support for different note types (general, scripture-based, and cross-references).
- Highlight Bible verses with support for multiple colors.
- Autocomplete Bible book and passage names for user convenience.
- Search capabilities for both scripture text and personal notes.
- User registration, authentication (token-based), activation, and password management via email.
- Rate limiting for API endpoints to prevent abuse.
- Upload images to attach to notes.
- Built-in health check and metrics endpoints.
- Integration with Redis for caching and with external services such as SMTP for email and AWS S3 for image storage.
- OpenAPI documentation (Swagger) is available at `/swagger`.

## Performance Highlights
- **4.5x faster** authentication with Redis caching
- Token validation: 110μs → 25μs
- Full-text search with PostgreSQL tsvector
- Rate limiting with Redis atomic operations
- **51,492 req/sec** for Bible verse retrieval with Otter cache + singleflight (0.2ms avg latency)

## Custom-Built Infrastructure

Built from scratch for learning purposes (no external job queue libraries):

- **Job Scheduler**: Worker pool with goroutines and channels for async email delivery
- **Delayed Retry Queue**: Min-heap priority queue (`container/heap`) for exponential backoff (2→4→8 min)
- **Custom Error Types**: `MailerError` with `Retriable` flag for intelligent retry decisions
- **Singleflight + Otter Cache**: Request deduplication prevents cache stampede on high-traffic endpoints

## Documentation
- [Design](DESIGN.md)
- [Architecture](docs/ARCHITECTURE.md)
- [API](docs/API.md) or [Swagger UI](http://localhost:4000/swagger) for full documentation.


## Getting Started

### Prerequisites

- Go 1.21 or above
- PostgreSQL database
- Redis instance (for caching and rate limiting)
- SMTP credentials (for email features)
- AWS S3 credentials (for image storage)

### Installation

1. Clone the repository:

   ```sh
   git clone https://github.com/shuvoedward/Bible_project_2.git
   cd Bible_project_2
   ```

2. Configure required environment variables in a `.envrc` file (see Makefile for expected variable names).

3. Run database migrations:

   ```sh
   make run/migrations
   ```

4. Start the API server:

   ```sh
   make run/api
   ```

### Running Tests

- API Tests:  
  ```sh
  make run/test/api
  ```
- Database model Tests:  
  ```sh
  make run/test/db
  ```

## Project Structure

- `cmd/api/` — Main application code and API endpoint handlers
- `internal/data/` — Data models, queries, and business logic
- `internal/scheduler/` — Custom job scheduler with worker pool and delayed retry queue
- `internal/mailer/` — Email service with custom error types
- `migrations/` — Database migration files
- `docs/` — Swagger/OpenAPI auto-generated docs

## Configuration

Important configuration flags and environment variables:

- API server port: `-port`
- Database DSN: `-db-dsn`
- SMTP configuration: `-smtp-host`, `-smtp-port`, `-smtp-username`, `-smtp-password`, `-smtp-sender`
- Redis configuration: `-redis-host`, `-redis-port`, `-redis-db`, `-redis-poolsize`
- CORS: `-cors-trusted-origin`
- Rate limits: `-auth-rate-limit`, `-ip-rate-limit`, `-note-rate-limit`

See the main function and Makefile for all available configuration options.


## Known Limitations

- Single Bible translation support (plan to add more)
- Images limited to 10MB size
- Rate limits may need adjustment based on usage patterns


## Email Configuration

Currently configured with Mailtrap for demonstration purposes. 
To use real email delivery, update SMTP environment variables 
to your preferred provider (SendGrid, AWS SES, Mailgun, etc).


## License

This project is currently intended for personal use. Please contact the author for further licensing information if you wish to use or contribute to this project.

## Contact

API Support: shuvoedward@gmail.com
