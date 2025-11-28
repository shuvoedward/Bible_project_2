// @title           Bible Notes API
// @version         1.0
// @description     A note-taking API for Bible study and annotations
// @termsOfService  http://swagger.io/terms/

// @contact.name   API Support
// @contact.email  shuvoedward@gmail.com

// @host      localhost:4000
// @BasePath  /v1
// @Security ApiKeyAuth
// @in header
// @name Authorization
// @description API token authentication. Use format: "Bearer <your-api-token>"
package main

import (
	"context"
	"database/sql"
	"expvar"
	"flag"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"runtime"
	"shuvoedward/Bible_project/internal/cache"
	"shuvoedward/Bible_project/internal/data"
	"shuvoedward/Bible_project/internal/mailer"
	"shuvoedward/Bible_project/internal/ratelimit"
	"shuvoedward/Bible_project/internal/s3_service"
	"shuvoedward/Bible_project/internal/service"
	"strconv"
	"sync"
	"time"

	_ "shuvoedward/Bible_project/swagger"

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
		dsn          string
		maxOpenConns int
		maxIdleConns int
		maxIdleTime  time.Duration
	}

	smtp struct {
		host     string
		port     int
		username string
		password string
		sender   string
	}

	limiter struct {
		authRatelimit int
		ipRateLimit   int
		// ipRateLimitWindow   time.Duration
		noteRateLimit int
		// noteRateLimitWindow time.Duration
	}

	redisConfig cache.RedisConfig

	corsTrustedOrigin string
}

type RateLimit struct {
	IPRateLimiter   *ratelimit.RateLimiter
	NoteRateLimiter *ratelimit.RateLimiter
	AuthRateLimiter *ratelimit.RateLimiter
}
type application struct {
	wg               sync.WaitGroup
	ctx              context.Context
	cancel           context.CancelFunc
	logger           *slog.Logger
	config           config
	books            map[string]struct{} // name of all the Bible books "John"
	booksSearchIndex map[string][]string // Book names "joh": ["John", "1 John", "2 John", "3 John"]
	redis            *cache.RedisClient
	models           data.Models
	mailer           *mailer.Mailer
	RateLimit
	s3ImageService *s3_service.S3ImageService
}

func main() {
	var cfg config

	flag.IntVar(&cfg.port, "port", 4000, "API server port")

	flag.StringVar(&cfg.env, "env", "production", "Environment (development|staging|production)")

	flag.StringVar(&cfg.LanguageToolURL, "languageToolURL", os.Getenv("LANGUAGETOOL_URL"), "LanguageTool URL")

	if cfg.env == "production" {
		password := os.Getenv("DB_PASSWORD")
		port := getEnvAsInt("DB_PORT", 5432)
		cfg.db.dsn = fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=%s",
			os.Getenv("DB_USER"),
			url.QueryEscape(password),
			os.Getenv("DB_HOST"), // ← Changed from DB_PASSWORD
			port,
			os.Getenv("DB_NAME"),
			os.Getenv("DB_SSLMODE"),
		)
	} else {
		flag.StringVar(&cfg.db.dsn, "db-dsn", os.Getenv("BIBLE_DB_DSN"), "PostgreSQL DSN")
	}
	flag.IntVar(&cfg.db.maxOpenConns, "db-max-open-conns", 25, "PostgreSQL max open connection")
	flag.IntVar(&cfg.db.maxIdleConns, "db-max-idle-conns", 25, "PostgreSQL max idle connection")
	flag.DurationVar(&cfg.db.maxIdleTime, "db-max-idle-time", 15*time.Minute, "PostgreSQL max connection idle time")

	flag.StringVar(&cfg.smtp.host, "smtp-host", getEnv("SMTP_HOST", "sandbox.smtp.mailtrap.io"), "SMTP host")
	flag.IntVar(&cfg.smtp.port, "smtp-port", getEnvAsInt("SMTP_PORT", 25), "SMTP port")
	flag.StringVar(&cfg.smtp.username, "smtp-username", os.Getenv("SMTP_USERNAME"), "SMTP username")
	flag.StringVar(&cfg.smtp.password, "smtp-password", os.Getenv("SMTP_PASSWORD"), "SMTP password")
	flag.StringVar(&cfg.smtp.sender, "smtp-sender", os.Getenv("SMTP_SENDER"), "SMTP sender")

	flag.IntVar(&cfg.limiter.ipRateLimit, "ip-rate-limit", 200, "IP rate limit minutes")
	flag.IntVar(&cfg.limiter.noteRateLimit, "note-rate-limit", 30, "Note rate limit in minutes")
	flag.IntVar(&cfg.limiter.authRatelimit, "auth-rate-limit", 15, "Auth rate limit in minutes")

	flag.StringVar(&cfg.redisConfig.Host, "redis-host", getEnv("REDIS_HOST", "localhost"), "Redis Host")
	flag.StringVar(&cfg.redisConfig.Port, "redis-port", getEnv("REDIS_PORT", "6379"), "Redis Port")
	flag.StringVar(&cfg.redisConfig.Password, "redis-password", getEnv("REDIS_PASSWORD", ""), "Redis Password")
	flag.IntVar(&cfg.redisConfig.DB, "redis-db", 0, "Redis DB")
	flag.IntVar(&cfg.redisConfig.PoolSize, "redis-poolsize", 10, "Redis Pool Size")

	flag.StringVar(&cfg.corsTrustedOrigin, "cors-trusted-origin", "http://localhost:9000", "Cross Origin Trusted")

	flag.Parse()

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	ctx, cancel := context.WithCancel(context.Background())

	// DB connections

	db, err := openDB(cfg)
	if err != nil {
		logger.Error(err.Error())
		os.Exit(1)
	}
	logger.Info("Successful connection to database")

	redisClient, err := cache.NewRedisClient(cfg.redisConfig, 15*time.Minute)
	if err != nil {
		logger.Error(err.Error())
		os.Exit(1)
	}
	logger.Info("Successful connection to redis")

	books := make(map[string]struct{}, 66)
	for _, bookTitle := range data.AllBooks {
		books[bookTitle] = struct{}{}
	}

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

	expvar.NewString("version").Set(version)

	expvar.Publish("goroutines", expvar.Func(func() any {
		return runtime.NumGoroutine()
	}))

	expvar.Publish("database", expvar.Func(func() any {
		return db.Stats()
	}))

	expvar.Publish("timestamp", expvar.Func(func() any {
		return time.Now().Unix()
	}))

	rateLimit := RateLimit{
		IPRateLimiter:   ratelimit.NewRateLimiter(cfg.limiter.ipRateLimit, time.Minute),
		NoteRateLimiter: ratelimit.NewRateLimiter(cfg.limiter.noteRateLimit, time.Minute),
		AuthRateLimiter: ratelimit.NewRateLimiter(cfg.limiter.authRatelimit, time.Minute),
	}

	models := data.NewModels(db)

	services := service.NewServices(
		models,
		logger,
		s3ImageService,
		redisClient,
		mailer,
		books,
	)

	app := application{
		ctx:              ctx,
		cancel:           cancel,
		config:           cfg,
		books:            books,
		booksSearchIndex: booksSearchIndex,
		logger:           logger,
		redis:            redisClient,
		models:           models,
		mailer:           mailer,
		RateLimit:        rateLimit,
		s3ImageService:   s3ImageService,
	}

	// Create all handlers
	handlers := NewHandlers(&app, services)

	app.background(app.runBackgroundTasks)

	err = app.serve(handlers)
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

	db.SetMaxOpenConns(cfg.db.maxOpenConns)
	db.SetMaxIdleConns(cfg.db.maxIdleConns)
	db.SetConnMaxIdleTime(cfg.db.maxIdleTime)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err = db.PingContext(ctx)
	if err != nil {
		db.Close()
		return nil, err
	}

	return db, nil
}

func (app *application) runBackgroundTasks() {
	ticker := time.NewTicker(6 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			app.logger.Info("running scheduled token cleanup")

			affectedRows, err := app.models.Tokens.DeleteExpiredToken()
			if err != nil {
				app.logger.Error("scheduled token cleanup failed", "error", err)
			} else {
				app.logger.Info("Deleted expired token", "affectedRows", affectedRows)
			}

		case <-app.ctx.Done():
			app.logger.Info("background tasks stopping")
			return
		}
	}
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}

	return fallback
}

func getEnvAsInt(key string, fallback int) int {
	if value := os.Getenv(key); value != "" {
		if intVal, err := strconv.Atoi(value); err == nil {
			return intVal
		}
	}
	return fallback
}
