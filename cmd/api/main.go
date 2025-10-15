package main

import (
	"database/sql"
	"flag"
	"log/slog"
	"os"
	"shuvoedward/Bible_project/internal/data"
	"shuvoedward/Bible_project/internal/mailer"
	"shuvoedward/Bible_project/internal/ratelimit"
	"sync"
	"time"

	_ "github.com/lib/pq"
)

type config struct {
	port int
	db   struct {
		dsn string
	}
	smtp struct {
		host     string
		port     int
		username string
		password string
		sender   string
	}
	ratelimit struct {
		ipRateLimit int
		// ipRateLimitWindow   time.Duration
		noteRateLimit int
		// noteRateLimitWindow time.Duration
	}
}

type application struct {
	config          config
	books           map[string]struct{}
	logger          *slog.Logger
	models          data.Models
	mailer          *mailer.Mailer
	wg              sync.WaitGroup
	ipRateLimiter   *ratelimit.RateLimiter
	noteRateLimiter *ratelimit.RateLimiter // TODO: Change name to note to writeRateLimit
}

func main() {
	var cfg config

	flag.IntVar(&cfg.port, "port", 4000, "API server port")

	flag.StringVar(&cfg.db.dsn, "db-dsn", "", "PostgreSQL DSN")

	flag.StringVar(&cfg.smtp.host, "smtp-host", "sandbox.smtp.mailtrap.io", "SMTP host")
	flag.IntVar(&cfg.smtp.port, "smtp-port", 25, "SMTP port")
	flag.StringVar(&cfg.smtp.username, "smtp-username", "c1692736a88ff8", "SMTP username")
	flag.StringVar(&cfg.smtp.password, "smtp-password", "8f8adcaf82b8a4", "SMTP password")
	flag.StringVar(&cfg.smtp.sender, "smtp-sender", "<no-reply@bible.edward.net>", "SMTP sender")

	flag.IntVar(&cfg.ratelimit.ipRateLimit, "ip-rate-limit", 30, "IP rate limit")
	flag.IntVar(&cfg.ratelimit.noteRateLimit, "note-rate-limit", 5, "Note rate limit")

	flag.Parse()

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	db, err := openDB(cfg)
	if err != nil {
		logger.Error(err.Error())
		os.Exit(1)
	}
	logger.Info("Successful connection to database")

	books := make(map[string]struct{}, 66)
	for _, bookTitle := range data.AllBooks {
		books[bookTitle] = struct{}{}
	}

	mailer, err := mailer.New(cfg.smtp.host, cfg.smtp.port, cfg.smtp.username, cfg.smtp.password, cfg.smtp.sender)
	if err != nil {
		logger.Error(err.Error())
		os.Exit(1)
	}

	app := application{
		config:          cfg,
		books:           books,
		logger:          logger,
		models:          data.NewModels(db),
		mailer:          mailer,
		ipRateLimiter:   ratelimit.NewRateLimiter(cfg.ratelimit.ipRateLimit, time.Second),
		noteRateLimiter: ratelimit.NewRateLimiter(cfg.ratelimit.noteRateLimit, time.Second),
	}

	err = app.serve()
	if err != nil {
		logger.Error(err.Error())
		os.Exit(1)
	}
}

func openDB(cfg config) (*sql.DB, error) {
	db, err := sql.Open("postgres", cfg.db.dsn)
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
