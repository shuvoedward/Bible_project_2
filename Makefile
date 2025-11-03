include .envrc


run/api:
	go run ./cmd/api -db-dsn=${BIBLE_DB_DSN} -languageToolURL=${LANGUAGETOOL_URL}

run/migrations:
	migrate -path=./migrations -database=${BIBLE_DB_DSN} up	

run/test/api:
	go test -count=1 shuvoedward/Bible_project/cmd/api

run/test/db:
	BIBLE_TEST_DB_DSN=${BIBLE_TEST_DB_DSN} go test -count=1 shuvoedward/Bible_project/internal/data

run/test/migrate-up:
	migrate -path=./migrations -database=${BIBLE_TEST_DB_DSN} up	

db_test/psql:
	psql ${BIBLE_TEST_DB_DSN}

db/psql:
	psql ${BIBLE_DB_DSN}


