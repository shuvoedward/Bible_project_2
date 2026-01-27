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
	imageProcessor "shuvoedward/Bible_project/internal/imageCompress"
	"shuvoedward/Bible_project/internal/mailer"
	"shuvoedward/Bible_project/internal/ratelimit"
	"shuvoedward/Bible_project/internal/s3Service"
	"shuvoedward/Bible_project/internal/scheduler"
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

	s3 struct {
		bucketName      string
		region          string
		secretAccessKey string
		accessKeyID     string
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

type application struct {
	config      config
	logger      *slog.Logger
	services    *service.Service
	rateLimiter *ratelimit.RateLimiters
	wg          *sync.WaitGroup
}

func main() {
	// 1. Load configuration
	var cfg config
	loadConfig(&cfg)

	// 2: Initialize logger
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	// 3: Initialize infrastructure
	db, err := openDB(cfg)
	if err != nil {
		logger.Error(err.Error())
		os.Exit(1)
	}
	defer db.Close()

	redisClient, err := cache.NewRedisClient(cfg.redisConfig, 15*time.Minute)
	if err != nil {
		logger.Error(err.Error())
		os.Exit(1)
	}
	defer redisClient.Close()

	s3Client, err := openS3(&cfg)
	if err != nil {
		logger.Error(err.Error())
		os.Exit(1)
	}

	mailer, err := openMailer(&cfg)
	if err != nil {
		logger.Error(err.Error())
		os.Exit(1)
	}

	scheduler := scheduler.NewScheduler(10)
	scheduler.Start()

	// 4. Initialize data layer models
	model := data.NewModels(db)

	// 5. Initialize domain data
	books := makeBooksMap()
	booksSearchIndex := data.BuildBookSearchIndex(data.AllBooks)

	// 6. Intialize image processor
	imgProcessor := imageProcessor.NewImageProcessor(1920, 1920, 85)

	// 7. Initialize services (business logic layer)
	services := service.NewServices(
		model,
		logger,
		s3Client,
		redisClient,
		mailer,
		books,
		booksSearchIndex,
		imgProcessor,
		scheduler,
	)

	// 8. Initialize rate limiters
	rateLimiters := ratelimit.NewRateLimiters(
		cfg.limiter.ipRateLimit,
		cfg.limiter.noteRateLimit,
		cfg.limiter.authRatelimit,
		time.Minute,
	)

	// 9. Set up expvar metrics
	setupMetrics(version, db)

	// 10. Initialize background tasks with panic recovery
	backgroundTasks := newBackgroundTasks(model.Tokens, logger)
	go backgroundTasks.start()

	// 11. Create application container
	app := application{
		config:      cfg,
		logger:      logger,
		services:    services,
		rateLimiter: rateLimiters,
		wg:          &sync.WaitGroup{},
	}

	// 12. Create all handlers
	handlers := NewHandlers(&app, services)

	// 13. Start server
	err = app.serve(handlers)
	if err != nil {
		logger.Error("failed to start server", "error", err)
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

func openS3(cfg *config) (*s3Service.S3ImageService, error) {
	s3Cfg := s3Service.S3Config{
		Region:          cfg.s3.region, // "us-east-1"
		BucketName:      cfg.s3.bucketName,
		AccessKeyID:     cfg.s3.accessKeyID,     // "" for EC2 instance profile
		SecretAccessKey: cfg.s3.secretAccessKey, // "" for EC2 instance profile
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	awsConfig, err := s3Service.LoadAWSConfig(ctx, s3Cfg)
	if err != nil {
		return nil, err
	}

	s3ImageService := s3Service.NewS3ImageService(
		awsConfig,
		s3Cfg.BucketName,
		s3Cfg.Region,
	)

	return s3ImageService, nil
}

func openMailer(cfg *config) (*mailer.Mailer, error) {
	return mailer.NewMailer(
		cfg.smtp.host,
		cfg.smtp.port,
		cfg.smtp.username,
		cfg.smtp.password,
		cfg.smtp.sender,
	)
}

func loadConfig(cfg *config) {
	// Server
	flag.IntVar(&cfg.port, "port", 4000, "API server port")
	flag.StringVar(&cfg.env, "env", "development", "Environment (development|staging|production)")

	// Database
	flag.IntVar(&cfg.db.maxOpenConns, "db-max-open-conns", 25, "PostgreSQL max open connection")
	flag.IntVar(&cfg.db.maxIdleConns, "db-max-idle-conns", 25, "PostgreSQL max idle connection")
	flag.DurationVar(&cfg.db.maxIdleTime, "db-max-idle-time", 15*time.Minute, "PostgreSQL max connection idle time")

	// Redis
	flag.StringVar(&cfg.redisConfig.Host, "redis-host", getEnv("REDIS_HOST", "localhost"), "Redis Host")
	flag.StringVar(&cfg.redisConfig.Port, "redis-port", getEnv("REDIS_PORT", "6379"), "Redis Port")
	flag.StringVar(&cfg.redisConfig.Password, "redis-password", getEnv("REDIS_PASSWORD", ""), "Redis Password")
	flag.IntVar(&cfg.redisConfig.DB, "redis-db", 0, "Redis DB")
	flag.IntVar(&cfg.redisConfig.PoolSize, "redis-poolsize", 10, "Redis Pool Size")

	// SMTP
	flag.StringVar(&cfg.smtp.host, "smtp-host", getEnv("SMTP_HOST", "sandbox.smtp.mailtrap.io"), "SMTP host")
	flag.IntVar(&cfg.smtp.port, "smtp-port", getEnvAsInt("SMTP_PORT", 25), "SMTP port")
	flag.StringVar(&cfg.smtp.username, "smtp-username", os.Getenv("SMTP_USERNAME"), "SMTP username")
	flag.StringVar(&cfg.smtp.password, "smtp-password", os.Getenv("SMTP_PASSWORD"), "SMTP password")
	flag.StringVar(&cfg.smtp.sender, "smtp-sender", os.Getenv("SMTP_SENDER"), "SMTP sender")

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

	flag.IntVar(&cfg.limiter.ipRateLimit, "ip-rate-limit", 200, "IP rate limit minutes")
	flag.IntVar(&cfg.limiter.noteRateLimit, "note-rate-limit", 30, "Note rate limit in minutes")
	flag.IntVar(&cfg.limiter.authRatelimit, "auth-rate-limit", 15, "Auth rate limit in minutes")

	flag.StringVar(&cfg.corsTrustedOrigin, "cors-trusted-origin", "http://localhost:9000", "Cross Origin Trusted")

	flag.Parse()
}

func makeBooksMap() map[string]struct{} {
	books := make(map[string]struct{}, 66)
	for _, bookTitle := range data.AllBooks {
		books[bookTitle] = struct{}{}
	}
	return books
}
func setupMetrics(version string, db *sql.DB) {
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
