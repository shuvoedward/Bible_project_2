# Include variables from the .envrc file
include .envrc

# ==================================================================================== #
# HELPERS
# ==================================================================================== #

## help: print this help message
.PHONY: help
help:
	@echo 'Usage:'
	@sed -n 's/^##//p' ${MAKEFILE_LIST} | column -t -s ':' |  sed -e 's/^/ /'


.PHONY: confirm
confirm:
	@echo -n 'Are you sure? [y/N]' && read ans && [ $${ans:-N} = y ]


# ==================================================================================== #
# DEVELOPMENT
# ==================================================================================== #


## run/api: run cmd/api application
.PHONY: run/api
run/api:
	go run ./cmd/api 

## db/psql: connect to the database using psql
.PHONY: db/psql
db/psql:
	psql ${BIBLE_DB_DSN}


## db/migrations/new name=$1: create a new database migration
.PHONY: db/migrations/new
db/migrations/new:
	@echo 'Creating migration files for ${name}...'
	migrate create -seq -ext=.sql -dir=./migrations ${name}

## db/migrations/up: apply all up database migrations
.PHONY: db/migrations/up
db/migrations/up: confirm
	@echo 'Running up migrations...'
	migrate -path ./migrations -database ${BIBLE_DB_DSN} up

# ==================================================================================== #
# QUALITY CONTROL
# ==================================================================================== #


## tidy: tidy module dependencies and format all .go files
.PHONY: tidy
tidy:
	@echo 'Tidying module dependencies...'
	go mod tidy
	@echo 'Formatting .go files...'
	go fmt ./...

## audit: run quality control checks
.PHONY: audit
audit:
	@echo 'Checking module dependencies...'
	go mod tidy -diff
	go mod verify
	@echo 'Vetting code...'
	go vet ./...
	go tool staticcheck ./...
	@echo 'Running tests...'
	go test -race -vet=off ./...


# ==================================================================================== #
# BUILD
# ==================================================================================== #

## build/api: build the cmd/api application
.PHONY: build/api
build/api:
	@echo 'Building cmd/api...'
	go build -ldflags='-s' -o=./bin/api ./cmd/api
	GOOS=linux GOARCH=amd64 go build -ldflags='-s' -o=./bin/linux_amd64/api ./cmd/api


# ==================================================================================== #
# TEST
# ==================================================================================== #
run/test/api:
	go test -count=1 shuvoedward/Bible_project/cmd/api

run/test/db:
	BIBLE_TEST_DB_DSN=${BIBLE_TEST_DB_DSN} go test -count=1 shuvoedward/Bible_project/internal/data

run/test/migrate-up:
	migrate -path=./migrations -database=${BIBLE_TEST_DB_DSN} up	

db_test/psql:
	psql ${BIBLE_TEST_DB_DSN}


