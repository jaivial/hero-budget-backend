package main

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// validateAppleToken validates the Apple JWT token (simplified version)
// In production, you should fetch Apple's public keys and validate properly
func validateAppleToken(tokenString string) (*AppleClaims, error) {
	// Parse token without verification for now (development only)
	// In production, implement proper JWT verification with Apple's public keys
	token, _, err := new(jwt.Parser).ParseUnverified(tokenString, &AppleClaims{})
	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %v", err)
	}

	claims, ok := token.Claims.(*AppleClaims)
	if !ok {
		return nil, fmt.Errorf("invalid token claims")
	}

	// Basic validation
	if claims.Subject == "" {
		return nil, fmt.Errorf("missing subject in token")
	}

	if claims.Expiration < time.Now().Unix() {
		return nil, fmt.Errorf("token expired")
	}

	return claims, nil
}

// extractUserFromRequest extracts user information from Apple Sign-In request
func extractUserFromRequest(req AppleSignInRequest, claims *AppleClaims) User {
	user := User{
		AppleID:       claims.Subject,
		Email:         claims.Email,
		VerifiedEmail: claims.EmailVerified == "true",
	}

	// Use provided user data if available (first-time sign-in)
	if req.UserData.Name.FirstName != "" || req.UserData.Name.LastName != "" {
		user.GivenName = req.UserData.Name.FirstName
		user.FamilyName = req.UserData.Name.LastName
		user.Name = fmt.Sprintf("%s %s", user.GivenName, user.FamilyName)
	}

	// Use provided email if available (overrides token email)
	if req.UserData.Email != "" {
		user.Email = req.UserData.Email
	}

	// Set locale
	if req.DeviceLocale != "" {
		user.Locale = req.DeviceLocale
	} else {
		user.Locale = "en-US" // Default locale
	}

	return user
}
