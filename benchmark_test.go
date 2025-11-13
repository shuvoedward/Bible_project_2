package main

import (
	"database/sql"
	"os"
	"shuvoedward/Bible_project/internal/cache"
	"shuvoedward/Bible_project/internal/data"
	"testing"
	"time"
)

func BenchmarkTokenValidationWithoutCache(b *testing.B) {
	db, err := sql.Open("postgres", os.Getenv("BIBLE_DB_DSN"))
	if err != nil {
		b.Fatalf("failed to connect to db %v", err)
	}
	defer db.Close()

	model := data.NewModels(db)

	token := "RN2NKWBUQVCUHJZAFGYSTXBM7H"
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, err := model.Users.GetForToken(token, data.ScopeAuthentication)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkTokenValidationWithCache(b *testing.B) {
	db, err := sql.Open("postgres", os.Getenv("BIBLE_DB_DSN"))
	if err != nil {
		b.Fatalf("failed to connect to db %v", err)
	}
	defer db.Close()

	model := data.NewModels(db)

	cfg := cache.RedisConfig{
		Host:     "localhost",
		Port:     "6379",
		Password: "",
		DB:       1, // 1 for tests
		PoolSize: 10,
	}

	redis, err := cache.NewRedisClient(cfg, 24*time.Hour)
	if err != nil {
		b.Fatalf("failed to connect to redis: %v", err)
	}

	token := "RN2NKWBUQVCUHJZAFGYSTXBM7H"

	user, _ := model.Users.GetForToken(token, data.ScopeAuthentication)
	redis.SetToken(token, user.ID, user.Activated)

	b.ResetTimer() // Start timing here

	// runs b.N times - adjusted by testing framework(atleast 1 second by default)
	for i := 0; i < b.N; i++ {
		cached, _ := redis.GetForToken(token)
		if cached == "" {
			_, err = model.Users.GetForToken(token, data.ScopeAuthentication)
			if err != nil {
				b.Fatal(err)
			}
		}
	}

}
