package data

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"time"
)

const (
	ScopeActivation     = "activation"
	ScopeAuthentication = "authentication"
	ScopePasswordReset  = "password-reset"
)

type TokenModel interface {
	New(ctx context.Context, userID int64, ttl time.Duration, scope string) (*Token, error)
	Insert(ctx context.Context, token *Token) error
	DeleteAllForUser(ctx context.Context, scope string, id int64) error
	DeleteExpiredTokens(ctx context.Context) (int64, error)
}

type Token struct {
	Plaintext string    `json:"token"`
	Hash      []byte    `json:"-"`
	UserID    int64     `json:"-"`
	Expiry    time.Time `json:"expiry"`
	Scope     string    `json:"-"`
}

func generateToken(userID int64, ttl time.Duration, scope string) *Token {
	token := &Token{
		Plaintext: rand.Text(),
		UserID:    userID,
		Expiry:    time.Now().Add(ttl),
		Scope:     scope,
	}

	hash := sha256.Sum256([]byte(token.Plaintext))
	token.Hash = hash[:]

	return token
}

type tokenModel struct {
	db DBTX
}

func NewTokenModel(db DBTX) TokenModel {
	return &tokenModel{db}
}

func (m tokenModel) New(ctx context.Context, userID int64, ttl time.Duration, scope string) (*Token, error) {
	token := generateToken(userID, ttl, scope)

	err := m.Insert(ctx, token)
	return token, err
}

func (m tokenModel) Insert(ctx context.Context, token *Token) error {
	query := `
		INSERT INTO tokens 
			(hash, user_id, expiry, scope)
		VALUES 
			($1, $2, $3, $4)
		ON CONFLICT 
			(user_id, scope)
		WHERE 
			scope IN ('activation', 'password-reset')
		DO UPDATE SET
			hash = EXCLUDED.hash,
			expiry = EXCLUDED.expiry
		`

	args := []any{token.Hash, token.UserID, token.Expiry, token.Scope}

	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	_, err := m.db.ExecContext(ctx, query, args...)

	return err
}

func (m tokenModel) DeleteAllForUser(ctx context.Context, scope string, id int64) error {
	query := `
		DELETE FROM 
			tokens 
		WHERE 
			scope = $1 
			AND user_id = $2`

	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	_, err := m.db.ExecContext(ctx, query, scope, id)
	return err
}

func (m tokenModel) DeleteExpiredTokens(ctx context.Context) (int64, error) {
	query := `
		DELETE FROM 
			tokens
		WHERE 
			expiry < NOW()`

	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	result, err := m.db.ExecContext(ctx, query)
	if err != nil {
		return 0, err
	}

	rowsAffected, _ := result.RowsAffected()

	return rowsAffected, nil
}
