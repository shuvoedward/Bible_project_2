# Bible Notes API

## Overview

Bible Notes API is a RESTful API designed to support Bible study, personal annotations, verse highlighting, and note management. It provides programmatic access to Bible text, passage lookup, highlights, and user notes, making it suitable for integration with web and mobile Bible study applications.

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

## API Overview

- All endpoints are rooted at `/v1/`.
- Authentication is via API tokens using the `Authorization: Bearer <token>` header.
- Example endpoints:
  - `GET /v1/bible/:book/:chapter` – Retrieve a Bible chapter or specified range.
  - `GET /v1/autocomplete/bible` – Autocomplete feature for Bible books and passages.
  - `POST /v1/notes` – Create a new note attached to a verse or generally.
  - `POST /v1/users` – Register a new user.
  - `POST /v1/tokens/authentication` – Authenticate and obtain a token.

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

## License

This project is currently intended for personal use. Please contact the author for further licensing information if you wish to use or contribute to this project.

## Contact

API Support: shuvoedward@gmail.com
