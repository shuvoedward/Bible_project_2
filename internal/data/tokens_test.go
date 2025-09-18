package data

import (
	"database/sql"
	"errors"
	"reflect"
	"testing"
	"time"
)

func TestTokenModel_New(t *testing.T) {
	m := NewModels(testDB)

	testUser := User{
		Name:      "cornelius",
		Email:     "example@gmail.com",
		Activated: false,
	}
	err := testUser.Password.Set("12345678")
	if err != nil {
		t.Fatalf("failed to set password: %v", err)
	}
	err = m.Users.Insert(&testUser)
	if err != nil {
		t.Fatalf("Failed to insert user: %v", err)
	}
	defer deleteUser(testUser.ID)

	testToken, err := m.Tokens.New(testUser.ID, 3*24*time.Hour, ScopeActivation)
	if err != nil {
		t.Fatalf("New() returned an error: %v", err)
	}
	defer deleteTestToken(testToken.UserID)

	retrievedToken, err := getTestToken(testToken.UserID)
	if err != nil {
		t.Fatalf("getTestToken() returned err: %v", err)
	}

	if !reflect.DeepEqual(testToken.Hash, retrievedToken.Hash) {
		t.Error("tokens does not match")
	}

	if !testToken.Expiry.Equal(retrievedToken.Expiry) {
		t.Errorf("expected expiry to be %v but got %v", testToken.Expiry, retrievedToken.Expiry)
	}

	if testToken.UserID != retrievedToken.UserID {
		t.Errorf("expected userID to be %d but got %d", testToken.UserID, retrievedToken.UserID)
	}

	if testToken.Scope != retrievedToken.Scope {
		t.Errorf("expected userID to be %s but got %s", testToken.Scope, retrievedToken.Scope)
	}
}

func TestTokenModel_DeleteAllForUser(t *testing.T) {

	m := NewModels(testDB)

	testUser := User{
		Name:      "cornelius",
		Email:     "example@gmail.com",
		Activated: false,
	}
	err := testUser.Password.Set("12345678")
	if err != nil {
		t.Fatalf("failed to set password: %v", err)
	}
	err = m.Users.Insert(&testUser)
	if err != nil {
		t.Fatalf("Failed to insert user: %v", err)
	}
	defer deleteUser(testUser.ID)

	testToken, err := m.Tokens.New(testUser.ID, 3*24*time.Hour, ScopeActivation)
	if err != nil {
		t.Fatalf("New() returned an error: %v", err)
	}
	defer deleteTestToken(testToken.UserID)

	err = m.Tokens.DeleteAllForUser(ScopeActivation, testUser.ID)
	if err != nil {
		t.Fatalf("DeleteAllForUser() returned err: %v", err)
	}

	token, err := getTestToken(testToken.UserID)
	if !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("token deletion did not succeed, token: %v", token)
	}
}

func getTestToken(userID int64) (Token, error) {
	query := `
		SELECT t.user_id, t.hash, t.expiry, t.scope
		FROM tokens as t
		WHERE t.user_id = $1`

	var token Token

	err := testDB.QueryRow(query, userID).Scan(
		&token.UserID, &token.Hash, &token.Expiry, &token.Scope,
	)
	return token, err
}

func deleteTestToken(userID int64) error {
	query := `
		DELETE from tokens
		WHERE user_id = $1`

	_, err := testDB.Exec(query, userID)
	return err
}
