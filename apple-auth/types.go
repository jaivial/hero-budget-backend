package main

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// User represents a user in the database
type User struct {
	ID            int       `json:"id"`
	AppleID       string    `json:"apple_id,omitempty"`
	GoogleID      string    `json:"google_id,omitempty"`
	Email         string    `json:"email"`
	Name          string    `json:"name"`
	GivenName     string    `json:"given_name"`
	FamilyName    string    `json:"family_name"`
	Picture       string    `json:"picture"`
	Locale        string    `json:"locale"`
	VerifiedEmail bool      `json:"verified_email"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// AppleClaims represents the claims in Apple's JWT token
type AppleClaims struct {
	Issuer         string `json:"iss"`
	Audience       string `json:"aud"`
	Expiration     int64  `json:"exp"`
	IssuedAt       int64  `json:"iat"`
	Subject        string `json:"sub"`
	Email          string `json:"email"`
	EmailVerified  string `json:"email_verified"`
	IsPrivateEmail string `json:"is_private_email"`
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
