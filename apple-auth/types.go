package main

import (
	"database/sql"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// User represents a user in the database
type User struct {
	ID               int            `json:"id"`
	AppleID          sql.NullString `json:"apple_id,omitempty"`
	GoogleID         sql.NullString `json:"google_id,omitempty"`
	Email            string         `json:"email"`
	Name             sql.NullString `json:"name"`
	GivenName        sql.NullString `json:"given_name"`
	FamilyName       sql.NullString `json:"family_name"`
	Picture          sql.NullString `json:"picture"`
	ProfileImageBlob sql.NullString `json:"profile_image_blob,omitempty"`
	Locale           sql.NullString `json:"locale"`
	VerifiedEmail    bool           `json:"verified_email"`
	CreatedAt        time.Time      `json:"created_at"`
	UpdatedAt        time.Time      `json:"updated_at"`
}

// AppleClaims represents the claims in Apple's JWT token
type AppleClaims struct {
	Issuer         string `json:"iss"`
	Audience       string `json:"aud"`
	Expiration     int64  `json:"exp"`
	IssuedAt       int64  `json:"iat"`
	Subject        string `json:"sub"`
	Email          string `json:"email"`
	EmailVerified  bool   `json:"email_verified"`
	IsPrivateEmail bool   `json:"is_private_email"`
	RealUserStatus int    `json:"real_user_status"`
	TransferSub    string `json:"transfer_sub"`
	jwt.RegisteredClaims
}

// AppleSignInRequest represents the request payload for Apple Sign-In
type AppleSignInRequest struct {
	IdentityToken string `json:"identityToken"`
	AuthCode      string `json:"authorizationCode"`
	DeviceLocale  string `json:"deviceLocale"`
	UserData      struct {
		Name struct {
			FirstName string `json:"firstName"`
			LastName  string `json:"lastName"`
		} `json:"name"`
		Email string `json:"email"`
	} `json:"user,omitempty"`
}

// AppleSignInResponse represents the response for Apple Sign-In
type AppleSignInResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message,omitempty"`
	User    interface{} `json:"user,omitempty"`
}
