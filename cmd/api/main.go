package main

import (
	"database/sql"
	"log/slog"
	"os"

	_ "github.com/lib/pq"
)

type application struct {
	logger *slog.Logger
	db     *sql.DB
}

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	BIBLE_DB_DSN := os.Getenv("BIBLE_DB_DSN")
	db, err := openDB(BIBLE_DB_DSN)
	if err != nil {
		logger.Error(err.Error())
		os.Exit(1)
	}

	app := application{
		logger: logger,
	}

	err = app.serve()
	if err != nil {
		logger.Error(err.Error())
		os.Exit(1)
	}

}

func openDB(dsn string) (*sql.DB, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, err
	}

	err = db.Ping()
	if err != nil {
		db.Close()
		return nil, err
	}

	return db, nil
}
