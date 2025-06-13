package main

import (
	"log"
	"time"
)

// getUserByAppleID retrieves a user by their Apple ID
func getUserByAppleID(appleID string) (*User, error) {
	var user User
	err := db.QueryRow(`
		SELECT id, apple_id, google_id, email, name, given_name, family_name, 
		picture, locale, verified_email, created_at, updated_at 
		FROM users WHERE apple_id = ?`, appleID).Scan(
		&user.ID,
		&user.AppleID,
		&user.GoogleID,
		&user.Email,
		&user.Name,
		&user.GivenName,
		&user.FamilyName,
		&user.Picture,
		&user.Locale,
		&user.VerifiedEmail,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	return &user, err
}

// getUserByEmail retrieves a user by their email address
func getUserByEmail(email string) (*User, error) {
	var user User
	err := db.QueryRow(`
		SELECT id, apple_id, google_id, email, name, given_name, family_name, 
		picture, locale, verified_email, created_at, updated_at 
		FROM users WHERE email = ?`, email).Scan(
		&user.ID,
		&user.AppleID,
		&user.GoogleID,
		&user.Email,
		&user.Name,
		&user.GivenName,
		&user.FamilyName,
		&user.Picture,
		&user.Locale,
		&user.VerifiedEmail,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	return &user, err
}

// createAppleUser creates a new user with Apple Sign-In data
func createAppleUser(user User) (*User, error) {
	result, err := db.Exec(`
		INSERT INTO users (
			apple_id, email, name, given_name, family_name, 
			locale, verified_email
		) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		user.AppleID, user.Email, user.Name, user.GivenName,
		user.FamilyName, user.Locale, user.VerifiedEmail,
	)
	if err != nil {
		return nil, err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}

	user.ID = int(id)
	user.CreatedAt = time.Now()
	user.UpdatedAt = time.Now()
	return &user, nil
}

// linkAppleIDToUser links an Apple ID to an existing user account
func linkAppleIDToUser(userID int, appleID string) error {
	_, err := db.Exec("UPDATE users SET apple_id = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?", appleID, userID)
	return err
}

// updateUserLastLogin updates the last login timestamp for a user
func updateUserLastLogin(userID int) {
	_, err := db.Exec("UPDATE users SET updated_at = CURRENT_TIMESTAMP WHERE id = ?", userID)
	if err != nil {
		log.Printf("Failed to update last login time: %v", err)
	}
}
