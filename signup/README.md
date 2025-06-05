# Enhanced Signup Service for Hero Budget App

This service handles manual user registration for the Hero Budget Flutter app with advanced features.

## Features

- Email existence check
- User registration with profile picture support
- Image processing (compression, resizing, WebP conversion)
- Email verification
- Configuration via JSON file
- Shared database with the Google Auth service

## Setup and Run

1. Make sure you have Go installed (v1.19 or newer)
2. Install dependencies:
   ```
   cd backend
   go mod tidy
   ```
3. Configure the service by editing `config.json` with your SMTP settings:
   ```json
   {
     "smtp": {
       "host": "smtp.example.com",
       "port": 587,
       "username": "your-email@example.com",
       "password": "your-password",
       "from_email": "your-email@example.com"
     },
     "app": {
       "base_url": "http://localhost:3000",
       "verification_page": "/verify-email"
     }
   }
   ```
4. Run the service:
   ```
   cd signup
   go run main.go
   ```

The service will start on port 8082 by default.

## Email Verification

When a user signs up, the service:
1. Stores the user with `verified_email` set to `false`
2. Generates a unique verification code
3. Sends a verification email with a link
4. Verifies the email when the user clicks the link

## Image Processing

The service handles profile images by:
1. Decoding the Base64 image from the signup request
2. Resizing the image if it's too large (max 800x800)
3. Converting it to WebP format for better compression
4. Compressing the image to keep it under 100KB
5. Storing it in the database for future use

## API Endpoints

### Check Email Existence

```
POST /signup/check-email
```

Request body:
```json
{
  "email": "user@example.com"
}
```

Response:
```json
{
  "exists": true|false
}
```

### Register New User

```
POST /signup/register
```

Request body:
```json
{
  "email": "user@example.com",
  "password": "user_password",
  "name": "Full Name",
  "given_name": "First Name",
  "family_name": "Last Name",
  "picture_base64": "base64_encoded_image", // Optional
  "locale": "en-US",
  "verified_email": false
}
```

Note: The `verified_email` field is always set to `false` regardless of the value sent in the request.

### Verify Email

```
GET /signup/verify-email?code=verification_code
```

or

```
POST /signup/verify-email
```

Request body:
```json
{
  "code": "verification_code"
}
```

Response:
```json
{
  "success": true,
  "message": "Email verification successful",
  "user_id": 123,
  "email": "user@example.com"
}
```

## Database Schema

The service shares the database with the Google Auth service, but adds these columns:
- `password`: For storing the user's password
- `profile_image_blob`: For storing the compressed WebP image as base64
- `verification_code`: For email verification

## Integration with Flutter App

In the Flutter app, the endpoints are configured as follows:

1. In the email check function:
   ```dart
   Uri.parse('http://192.168.0.22:8082/signup/check-email')
   ```

2. In the signup function:
   ```dart
   Uri.parse('http://192.168.0.22:8082/signup/register')
   ```

3. For email verification (to be implemented):
   ```dart
   Uri.parse('http://192.168.0.22:8082/signup/verify-email')
   ```

**Note:** You need to create a verification page in your Flutter app that accepts a verification code parameter from the URL or implement a deep link handler. Update the `base_url` and `verification_page` in `config.json` to match your app's structure.

**Important:** The IP address `192.168.0.22` should be replaced with your actual development machine's IP address when testing on real devices. Using `localhost` will only work when testing on simulators/emulators. 