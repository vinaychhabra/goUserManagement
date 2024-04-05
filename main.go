package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"path/filepath"
	"time"

	_ "github.com/lib/pq"
	"golang.org/x/crypto/bcrypt"

	"github.com/dgrijalva/jwt-go"
	"golang.org/x/time/rate"
)

// Define database connection details
const (
	host     = "localhost"
	port     = 5432
	user     = "root"
	password = "pass123456"
	dbname   = "users"
)

// Define database tables
const (
	usersTable       = "users"
	invitationsTable = "invitations"
	adminsTable      = "admins"
)

// Invitation struct represents an invitation code //B
type Invitation struct {
	ID       int       `json:"id"`
	Code     string    `json:"code"`
	Used     bool      `json:"used"`
	IssuedAt time.Time `json:"issued_at"`
}

var jwtKey = []byte("your_secret_key")

// User struct represents a user in the system
type User struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// Admin struct represents an admin user
type Admin struct {
	ID       int    `json:"id"`
	Username string `json:"username"`
	Password string `json:"-"`
}

// SetupDatabase creates a connection to the PostgreSQL database
func SetupDatabase() *sql.DB {
	connStr := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		host, port, user, password, dbname)
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		log.Fatal(err)
	}
	return db
}

var limiter = rate.NewLimiter(rate.Limit(1), 5) // Allow 1 request per 5 seconds

// RegisterHandler handles user registration
func RegisterHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Parse request body to extract user details
		var requestBody struct {
			Username       string `json:"username"`
			Password       string `json:"password"`
			InvitationCode string `json:"invitation_code"`
		}
		if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
			http.Error(w, "Failed to decode request body", http.StatusBadRequest)
			return
		}

		// Extract user details from the request body
		username := requestBody.Username
		password := requestBody.Password
		invitationCode := requestBody.InvitationCode

		// Validate invitation code
		if invitationCode == "" {
			http.Error(w, "Invitation code is required", http.StatusBadRequest)
			return
		}
		valid, err := validateInvitationCode(db, invitationCode)
		if err != nil {
			http.Error(w, "Failed to validate invitation code", http.StatusInternalServerError)
			return
		}
		if !valid {
			http.Error(w, "Invalid invitation code", http.StatusUnauthorized)
			return
		}

		// Check if the username already exists
		exists, err := isUsernameExists(db, username)
		if err != nil {
			http.Error(w, "Failed to check username existence", http.StatusInternalServerError)
			return
		}
		if exists {
			http.Error(w, "Username already exists", http.StatusConflict)
			return
		}

		// Hash user password
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if err != nil {
			http.Error(w, "Failed to hash password", http.StatusInternalServerError)
			return
		}

		// Save user details to the database
		_, err = db.Exec("INSERT INTO users (username, password_hash) VALUES ($1, $2)", username, string(hashedPassword))
		if err != nil {
			http.Error(w, "Failed to save user", http.StatusInternalServerError)
			return
		}

		// Mark invitation code as used
		err = markInvitationCodeAsUsed(db, invitationCode)
		if err != nil {
			log.Println("Failed to mark invitation code as used:", err)
			// This is a non-critical error, so continue with the registration
		}

		fmt.Fprintf(w, "User registered successfully")
	}
}

// isUsernameExists checks if the username already exists in the database   //C
func isUsernameExists(db *sql.DB, username string) (bool, error) {
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM users WHERE username = $1", username).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// validateInvitationCode checks if the provided invitation code is valid and unused //C
func validateInvitationCode(db *sql.DB, code string) (bool, error) {
	var used bool
	err := db.QueryRow("SELECT used FROM invitations WHERE code = $1", code).Scan(&used)
	if err != nil {
		if err == sql.ErrNoRows {
			return false, nil // Invitation code not found
		}
		return false, err // Other error occurred
	}
	return !used, nil // Invitation code is valid if it is not used
}

// markInvitationCodeAsUsed marks the invitation code as used in the database
func markInvitationCodeAsUsed(db *sql.DB, code string) error {
	_, err := db.Exec("UPDATE invitations SET used = true WHERE code = $1", code)
	return err
}

func GenerateJWT(user User) (string, error) {
	claims := jwt.MapClaims{
		"username": user.Username,
		"exp":      time.Now().Add(time.Minute * 15).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jwtKey)
}
func AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Retrieve session token from the request cookies
		cookie, err := r.Cookie("session_token")
		if err != nil || cookie.Value == "" {
			http.HandleFunc("/", StaticFileHandler)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		// Validate session token
		if !validateSessionToken(cookie.Value) {
			http.HandleFunc("/", StaticFileHandler)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		// If token is valid, proceed to the next handler
		next.ServeHTTP(w, r)
	})
}
func validateSessionToken(tokenString string) bool {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		// Check if the signing method is HMAC
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		// Return the secret key used for signing
		return jwtKey, nil
	})
	if err != nil || !token.Valid {
		return false
	}
	return true
}

// LoginHandler handles user login functionality
func LoginHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Check if the request method is POST
		if r.Method != "POST" {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		ctx := r.Context()
		if err := limiter.Wait(ctx); err != nil {
			http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
			return
		}

		// Parse the request body to extract user credentials
		var user User
		if err := json.NewDecoder(r.Body).Decode(&user); err != nil {
			http.Error(w, "Failed to decode request body", http.StatusBadRequest)
			return
		}
		log.Println("Received user:", user)

		// Retrieve the stored password hash for the given username from the database
		var storedPasswordHash string
		err := db.QueryRow("SELECT password_hash FROM users WHERE username = $1", user.Username).Scan(&storedPasswordHash)
		if err != nil {
			if err == sql.ErrNoRows {
				http.Error(w, "User not found", http.StatusUnauthorized)
				return
			}
			http.Error(w, "Failed to retrieve user details", http.StatusInternalServerError)
			log.Println("Error retrieving user details:", err)
			return
		}

		// Log the stored password hash for debugging purposes
		log.Println("Stored password hash for user", user.Username, ":", storedPasswordHash)

		// Compare the stored hashed password with the provided password
		err = bcrypt.CompareHashAndPassword([]byte(storedPasswordHash), []byte(user.Password))
		if err != nil {
			http.Error(w, "Invalid username or password", http.StatusUnauthorized)
			log.Println("Invalid username or password for user", user.Username, ":", err)
			return
		}

		// Generate JWT token
		tokenString, err := GenerateJWT(user)
		if err != nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		// Set JWT token as cookie
		expiration := time.Now().Add(24 * time.Hour)
		cookie := http.Cookie{Name: "session_token", Value: tokenString, Expires: expiration}
		http.SetCookie(w, &cookie)

		// Respond with a success message (or a JSON response)
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "Logged in successfully. Welcome, %s!", user.Username)
	}
}
func generateInvitationCode() string {
	// Define the characters that can be used in the invitation code
	const chars = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"

	// Initialize a random seed using the current time
	rand.Seed(time.Now().UnixNano())

	// Initialize an empty string to store the generated code
	code := make([]byte, 10)

	// Generate a random character from the chars string and append it to the code
	for i := range code {
		code[i] = chars[rand.Intn(len(chars))]
	}

	// Return the generated invitation code as a string
	return string(code)
}
func dashboardFileHandler(w http.ResponseWriter, r *http.Request) {
	filePath := filepath.Join("frontend", "dashboard.html")
	http.ServeFile(w, r, filePath)
}

// Protected dashboard endpoint with session validation middleware

func logoutHandler(w http.ResponseWriter, r *http.Request) {
	// Clear the session token cookie
	cookie := http.Cookie{
		Name:     "session_token",
		Value:    "",
		Expires:  time.Now().AddDate(0, 0, -1), // Expire immediately
		HttpOnly: true,
		Path:     "/",
	}
	http.SetCookie(w, &cookie)
	fmt.Fprintf(w, "Logged out successfully")
}

// this Function will generate a new invitation code
func GenerateInvitationHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Parse the request body to extract admin credentials
		var admin Admin
		if err := json.NewDecoder(r.Body).Decode(&admin); err != nil {
			http.Error(w, "Failed to decode request body", http.StatusBadRequest)
			return
		}

		// Check if the provided admin credentials are valid
		valid, err := verifyAdminCredentials(db, admin)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to verify admin credentials: %v", err), http.StatusInternalServerError)
			return
		}
		if !valid {
			http.Error(w, "Invalid admin credentials", http.StatusUnauthorized)
			return
		}

		// Generate a new unique invitation code
		invitationCode := generateInvitationCode()

		// Store the invitation code in the database
		if err := storeInvitationCode(db, invitationCode); err != nil {
			http.Error(w, fmt.Sprintf("Failed to generate invitation code: %v", err), http.StatusInternalServerError)
			return
		}

		// Return the generated invitation code
		json.NewEncoder(w).Encode(map[string]string{"invitation_code": invitationCode})
	}
}

// storeInvitationCode inserts the generated invitation code into the database
func storeInvitationCode(db *sql.DB, invitationCode string) error {
	_, err := db.Exec("INSERT INTO invitations (code, used, issued_at) VALUES ($1, $2, $3)",
		invitationCode, false, time.Now())
	return err
}

// verifies the provided admin credentials against the values stored in the database
func verifyAdminCredentials(db *sql.DB, admin Admin) (bool, error) {
	var storedPasswordHash string
	err := db.QueryRow("SELECT password_hash FROM admins WHERE username = $1", admin.Username).Scan(&storedPasswordHash)
	if err != nil {
		if err == sql.ErrNoRows {
			return false, nil // Admin not found
		}
		return false, err // Other error occurred
	}

	// Compare the provided password with the stored password hash
	err = bcrypt.CompareHashAndPassword([]byte(storedPasswordHash), []byte(admin.Password))
	if err != nil {
		return false, nil // Passwords do not match
	}

	return true, nil // Admin credentials are valid
}

func StaticFileHandler(w http.ResponseWriter, r *http.Request) {
	// Construct the absolute file path to index.html
	indexPath := filepath.Join("frontend", "index.html")

	// Serve the file
	http.ServeFile(w, r, indexPath)
}
func invitePageHandler(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "frontend/invite/index.html")
}

// RegisterAdminHandler handles registration of admin users
func RegisterAdminHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var admin Admin
		if err := json.NewDecoder(r.Body).Decode(&admin); err != nil {
			http.Error(w, "Failed to decode request body", http.StatusBadRequest)
			return
		}

		// Check if the admin username already exists
		var count int
		err := db.QueryRow("SELECT COUNT(*) FROM admins WHERE username = $1", admin.Username).Scan(&count)
		if err != nil {
			http.Error(w, "Failed to check admin existence", http.StatusInternalServerError)
			return
		}
		if count > 0 {
			http.Error(w, "Admin username already exists", http.StatusConflict)
			return
		}

		// Hash admin password
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(admin.Password), bcrypt.DefaultCost)
		if err != nil {
			http.Error(w, "Failed to hash password", http.StatusInternalServerError)
			return
		}

		// Store admin details in the database
		_, err = db.Exec("INSERT INTO admins (username, password_hash) VALUES ($1, $2)", admin.Username, string(hashedPassword))
		if err != nil {
			http.Error(w, "Failed to save admin", http.StatusInternalServerError)
			return
		}

		// Return success message
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte("Admin registered successfully!"))
	}
}

func main() {
	db := SetupDatabase()
	defer db.Close()

	http.HandleFunc("/register", RegisterHandler(db))
	http.HandleFunc("/login", LoginHandler(db))
	http.HandleFunc("/generate-invitation", GenerateInvitationHandler(db))
	http.HandleFunc("/register-admin", RegisterAdminHandler(db))
	http.HandleFunc("/invite", invitePageHandler)
	http.HandleFunc("/logout", logoutHandler)
	http.Handle("/dashboard", AuthMiddleware(http.HandlerFunc(dashboardFileHandler)))
	http.HandleFunc("/", StaticFileHandler)

	log.Println("Server started on :8081")
	log.Fatal(http.ListenAndServe(":8081", nil))
}
