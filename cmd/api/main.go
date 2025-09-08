package main

import (
	"database/sql"
	"log/slog"
	"os"
	"shuvoedward/Bible_project/internal/data"

	_ "github.com/lib/pq"
)

type application struct {
	books  map[string]struct{}
	logger *slog.Logger
	models data.Models
}

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	BIBLE_DB_DSN := os.Getenv("BIBLE_DB_DSN")
	db, err := openDB(BIBLE_DB_DSN)
	if err != nil {
		logger.Error(err.Error())
		os.Exit(1)
	}
	logger.Info("Successful connection to database")

	books := make(map[string]struct{}, 66)
	for _, bookTitle := range data.AllBooks {
		books[bookTitle] = struct{}{}
	}

	app := application{
		books:  books,
		logger: logger,
		models: data.NewModels(db),
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
