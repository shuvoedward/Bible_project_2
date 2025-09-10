package data

import (
	"database/sql"
	"log"
	"os"
	"testing"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/lib/pq"
)

var testDB *sql.DB

func TestMain(m *testing.M) {
	dsn := os.Getenv("BIBLE_TEST_DB_DSN")
	if dsn == "" {
		log.Println("BIBLE_TEST_DB_DSN not set, skipping integration tests")
		return
	}

	var err error
	testDB, err = sql.Open("postgres", dsn)
	if err != nil {
		log.Fatalf("failed to connect to test database: %s", err)
	}

	var _ = &file.File{}

	runMigrations(testDB)

	code := m.Run()

	testDB.Close()

	os.Exit(code)
}

func runMigrations(db *sql.DB) {
	driver, err := postgres.WithInstance(db, &postgres.Config{})
	if err != nil {
		log.Fatalf("could not create postgres driver: %s", err)
	}

	m, err := migrate.NewWithDatabaseInstance("file://../../migrations", "postgres", driver)
	if err != nil {
		log.Fatalf("could not create migrate instance: %s", err)
	}

	if err = m.Up(); err != nil && err != migrate.ErrNoChange {
		log.Fatalf("could not run up migrations: %s", err)
	}
	log.Println("migrations ran successfully")
}
