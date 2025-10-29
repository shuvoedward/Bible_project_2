// @title           Bible Notes API
// @version         1.0
// @description     A note-taking API for Bible study and annotations
// @termsOfService  http://swagger.io/terms/

// @contact.name   API Support
// @contact.email  shuvoedward@gmail.com

// @host      localhost:4000
// @BasePath  /v1

// @in header
// @name Authorization

package main

import (
	"context"
	"database/sql"
	"flag"
	"log/slog"
	"os"
	"shuvoedward/Bible_project/internal/cache"
	"shuvoedward/Bible_project/internal/data"
	"shuvoedward/Bible_project/internal/mailer"
	"shuvoedward/Bible_project/internal/ratelimit"
	"shuvoedward/Bible_project/internal/s3_service"
	"sync"
	"time"

	_ "github.com/lib/pq"
)

var (
	version = "1.0.0"
)

type config struct {
	port            int
	env             string
	LanguageToolURL string

	db struct {
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

	redisConfig cache.RedisConfig
}

type application struct {
	config           config
	books            map[string]struct{}
	booksSearchIndex map[string][]string
	logger           *slog.Logger
	redis            *cache.RedisClient
	models           data.Models
	mailer           *mailer.Mailer
	wg               sync.WaitGroup
	ipRateLimiter    *ratelimit.RateLimiter
	noteRateLimiter  *ratelimit.RateLimiter // TODO: Change name to note to writeRateLimit
	s3ImageService   *s3_service.S3ImageService
}

func main() {
	var cfg config

	flag.IntVar(&cfg.port, "port", 4000, "API server port")

	flag.StringVar(&cfg.env, "env", "development", "Environment (development|staging|production)")

	flag.StringVar(&cfg.LanguageToolURL, "languageToolURL", "", "LanguageTool URL")

	flag.StringVar(&cfg.db.dsn, "db-dsn", "", "PostgreSQL DSN")

	flag.StringVar(&cfg.smtp.host, "smtp-host", "sandbox.smtp.mailtrap.io", "SMTP host")
	flag.IntVar(&cfg.smtp.port, "smtp-port", 25, "SMTP port")
	flag.StringVar(&cfg.smtp.username, "smtp-username", "c1692736a88ff8", "SMTP username")
	flag.StringVar(&cfg.smtp.password, "smtp-password", "8f8adcaf82b8a4", "SMTP password")
	flag.StringVar(&cfg.smtp.sender, "smtp-sender", "<no-reply@bible.edward.net>", "SMTP sender")

	flag.IntVar(&cfg.ratelimit.ipRateLimit, "ip-rate-limit", 30, "IP rate limit")
	flag.IntVar(&cfg.ratelimit.noteRateLimit, "note-rate-limit", 5, "Note rate limit")

	flag.StringVar(&cfg.redisConfig.Host, "redis-host", "localhost", "Redis Host")
	flag.StringVar(&cfg.redisConfig.Port, "redis-port", "6379", "Redis Port")
	flag.StringVar(&cfg.redisConfig.Password, "redis-password", "", "Redis Password")
	flag.IntVar(&cfg.redisConfig.DB, "redis-db", 0, "Redis DB")
	flag.IntVar(&cfg.redisConfig.PoolSize, "redis-poolsize", 10, "Redis Pool Size")

	flag.Parse()

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	// DB connections

	db, err := openDB(cfg)
	if err != nil {
		logger.Error(err.Error())
		os.Exit(1)
	}
	logger.Info("Successful connection to database")

	redisClient, err := cache.NewRedisClient(cfg.redisConfig, 24*time.Hour)
	if err != nil {
		logger.Error(err.Error())
		os.Exit(1)
	}

	books := make(map[string]struct{}, 66)
	for _, bookTitle := range data.AllBooks {
		books[bookTitle] = struct{}{}
	}

	logger.Info("Successful connection to redis")

	booksSearchIndex := data.BuildBookSearchIndex(data.AllBooks)

	mailer, err := mailer.New(cfg.smtp.host, cfg.smtp.port, cfg.smtp.username, cfg.smtp.password, cfg.smtp.sender)
	if err != nil {
		logger.Error(err.Error())
		os.Exit(1)
	}

	s3Config, err := s3_service.NewS3Config(context.Background())
	if err != nil {
		logger.Error(err.Error())
		os.Exit(1)
	}

	s3ImageService := s3_service.NewS3ImageService(s3Config)

	app := application{
		config:           cfg,
		books:            books,
		booksSearchIndex: booksSearchIndex,
		logger:           logger,
		redis:            redisClient,
		models:           data.NewModels(db),
		mailer:           mailer,
		ipRateLimiter:    ratelimit.NewRateLimiter(cfg.ratelimit.ipRateLimit, time.Second),
		noteRateLimiter:  ratelimit.NewRateLimiter(cfg.ratelimit.noteRateLimit, time.Second),
		s3ImageService:   s3ImageService,
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
