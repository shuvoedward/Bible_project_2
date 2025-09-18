package data

import (
	"errors"
	"testing"
)

func TestUserModel_Insert(t *testing.T) {
	testUser := User{
		Name:      "cornelius",
		Email:     "example@gmail.com",
		Activated: false,
	}
	err := testUser.Password.Set("12345678")
	if err != nil {
		t.Fatalf("failed to set password: %v", err)
	}

	m := NewModels(testDB)
	err = m.Users.Insert(&testUser)
	if err != nil {
		t.Fatalf("Failed to insert user: %v", err)
	}
	defer deleteUser(testUser.ID)

	retrievedUser, err := m.Users.GetByEmail(testUser.Email)
	if err != nil {
		t.Fatalf("GetByEmail() returned an error: %v", err)
	}

	if testUser.Name != retrievedUser.Name {
		t.Errorf("expected name to be %s, but got %s", testUser.Name, retrievedUser.Name)
	}
}

func TestUserModel_GetByEmail(t *testing.T) {

	testUser := User{
		Name:      "cornelius",
		Email:     "example@gmail.com",
		Activated: false,
	}
	err := testUser.Password.Set("12345678")
	if err != nil {
		t.Fatalf("failed to set password: %v", err)
	}

	m := NewModels(testDB)
	err = m.Users.Insert(&testUser)
	if err != nil {
		t.Fatalf("Failed to insert user: %v", err)
	}
	defer deleteUser(testUser.ID)

	retrievedUser, err := m.Users.GetByEmail(testUser.Email)
	if err != nil {
		t.Fatalf("GetByEmail() returned an error: %v", err)
	}

	if testUser.Name != retrievedUser.Name {
		t.Errorf("expected name to be %s, but got %s", testUser.Name, retrievedUser.Name)
	}

	_, err = m.Users.GetByEmail("nonexistent@example.com")
	if !errors.Is(err, ErrRecordNotFound) {
		t.Errorf("Expected sql.ErrNoRows for nonexistent email, but got %v", err)
	}

}

func deleteUser(id int64) {
	query := `
		DELETE FROM users
		WHERE id = $1`
	testDB.Exec(query, id)
}
