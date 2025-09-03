include .envrc


run/api:
	go run ./cmd/api -db-dsn=${BIBLE_DB_DSN}


db/psql:
	psql ${BIBLE_DB_DSN}