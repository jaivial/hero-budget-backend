package main

import (
	"database/sql"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// validateAppleToken validates the Apple JWT token (simplified version)
// In production, you should fetch Apple's public keys and validate properly
func validateAppleToken(tokenString string) (*AppleClaims, error) {
	log.Printf("🔍 Starting Apple token validation")
	log.Printf("🎫 Token length: %d characters", len(tokenString))
	log.Printf("🎫 Token preview: %s...", tokenString[:min(50, len(tokenString))])

	// Parse token without verification for now (development only)
	// In production, implement proper JWT verification with Apple's public keys
	token, _, err := new(jwt.Parser).ParseUnverified(tokenString, &AppleClaims{})
	if err != nil {
		log.Printf("❌ Failed to parse token: %v", err)
		return nil, fmt.Errorf("failed to parse token: %v", err)
	}
	log.Printf("✅ Token parsed successfully")

	claims, ok := token.Claims.(*AppleClaims)
	if !ok {
		log.Printf("❌ Invalid token claims - type assertion failed")
		return nil, fmt.Errorf("invalid token claims")
	}
	log.Printf("✅ Claims extracted successfully")

	// Log claims details
	log.Printf("🔍 Claims details:")
	log.Printf("   - Subject: '%s'", claims.Subject)
	log.Printf("   - Email: '%s'", claims.Email)
	log.Printf("   - EmailVerified: %v", claims.EmailVerified)
	log.Printf("   - Issuer: '%s'", claims.Issuer)
	log.Printf("   - Audience: '%s'", claims.Audience)
	log.Printf("   - Expiration: %d", claims.Expiration)
	log.Printf("   - IssuedAt: %d", claims.IssuedAt)
	log.Printf("   - Current time: %d", time.Now().Unix())

	// Basic validation
	if claims.Subject == "" {
		log.Printf("❌ Missing subject in token")
		return nil, fmt.Errorf("missing subject in token")
	}
	log.Printf("✅ Subject validation passed")

	if claims.Expiration < time.Now().Unix() {
		log.Printf("❌ Token expired - exp: %d, now: %d", claims.Expiration, time.Now().Unix())
		return nil, fmt.Errorf("token expired")
	}
	log.Printf("✅ Expiration validation passed")

	log.Printf("🎉 Apple token validation completed successfully")
	return claims, nil
}

// Helper function for min
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// extractUserFromRequest extracts user information from Apple Sign-In request
func extractUserFromRequest(req AppleSignInRequest, claims *AppleClaims) User {
	user := User{
		AppleID:       sql.NullString{String: claims.Subject, Valid: claims.Subject != ""},
		Email:         claims.Email,
		VerifiedEmail: claims.EmailVerified,
		// Apple doesn't provide profile images, so initialize as empty
		ProfileImageBlob: sql.NullString{String: "", Valid: false},
	}

	log.Printf("🔍 Extracting user data from Apple Sign-In:")
	log.Printf("   - Email from token: %s", claims.Email)
	log.Printf("   - Subject: %s", claims.Subject)

	// Use provided user data if available (first-time sign-in)
	if req.UserData.Name.FirstName != "" || req.UserData.Name.LastName != "" {
		user.GivenName = sql.NullString{String: req.UserData.Name.FirstName, Valid: req.UserData.Name.FirstName != ""}
		user.FamilyName = sql.NullString{String: req.UserData.Name.LastName, Valid: req.UserData.Name.LastName != ""}
		fullName := strings.TrimSpace(fmt.Sprintf("%s %s", req.UserData.Name.FirstName, req.UserData.Name.LastName))
		user.Name = sql.NullString{String: fullName, Valid: fullName != ""}

		log.Printf("   - FirstName from request: '%s'", req.UserData.Name.FirstName)
		log.Printf("   - LastName from request: '%s'", req.UserData.Name.LastName)
		log.Printf("   - Full name constructed: '%s'", fullName)
	} else {
		log.Printf("   - No name data provided in user data")
		// If no name is provided, try to extract from email or use Apple ID
		emailParts := strings.Split(user.Email, "@")
		if len(emailParts) > 0 && emailParts[0] != "" {
			// Use email prefix as fallback name
			emailName := emailParts[0]
			user.Name = sql.NullString{String: emailName, Valid: true}
			log.Printf("   - Using email prefix as name: '%s'", emailName)
		} else {
			// Use "Usuario Apple" as fallback
			user.Name = sql.NullString{String: "Usuario Apple", Valid: true}
			log.Printf("   - Using default name: 'Usuario Apple'")
		}
	}

	// Use provided email if available (overrides token email)
	if req.UserData.Email != "" {
		user.Email = req.UserData.Email
		log.Printf("   - Email from user data: %s", req.UserData.Email)
	}

	// Set locale
	if req.DeviceLocale != "" {
		user.Locale = sql.NullString{String: req.DeviceLocale, Valid: true}
		log.Printf("   - Device locale: %s", req.DeviceLocale)
	} else {
		user.Locale = sql.NullString{String: "en-US", Valid: true} // Default locale
		log.Printf("   - Using default locale: en-US")
	}

	// Apple doesn't provide profile pictures, so Picture is left as empty/invalid
	user.Picture = sql.NullString{String: "", Valid: false}

	log.Printf("✅ User extraction completed for Apple ID: %s", claims.Subject)
	return user
}
