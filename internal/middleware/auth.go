package middleware

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strings"

	"github.com/SelfScriptKiddies/tweaker/internal/template"
)

// AuthConfig holds authentication configuration
type AuthConfig struct {
	Username     string
	Password     string
	LoginURL     string
	SecretCookie string
}

func GenerateRandomHex(length int) (string, error) {
	bytes := make([]byte, length/2)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// AuthMiddleware creates an authentication middleware
func AuthMiddleware(config AuthConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip authentication for login page and static files
			if r.URL.Path == config.LoginURL || strings.HasPrefix(r.URL.Path, "/static/") {
				next.ServeHTTP(w, r)
				return
			}

			// Check for basic auth or session
			if !isAuthenticated(r, config) {
				// Redirect to login page
				http.Redirect(w, r, config.LoginURL, http.StatusSeeOther)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// isAuthenticated checks user authentication
func isAuthenticated(r *http.Request, config AuthConfig) bool {
	// Check basic auth from header
	username, password, ok := r.BasicAuth()
	if ok && username == config.Username && password == config.Password {
		return true
	}

	// Check session cookie
	cookie, err := r.Cookie("auth_session")
	if err == nil && cookie.Value == config.SecretCookie {
		return true
	}

	return false
}

// LoginHandler handles login requests
func LoginHandler(config AuthConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			username := r.FormValue("username")
			password := r.FormValue("password")

			if username == config.Username && password == config.Password {
				// Set authentication cookie
				http.SetCookie(w, &http.Cookie{
					Name:     "auth_session",
					Value:    config.SecretCookie,
					Path:     "/",
					HttpOnly: true,
					Secure:   false, // Set to true in production with HTTPS
				})
				http.Redirect(w, r, "/", http.StatusSeeOther)
				return
			}

			// Show login page with error
			template.RenderTemplate(w, "login", map[string]string{
				"Error": "Invalid credentials",
			})
			return
		}

		// Show login page
		template.RenderTemplate(w, "login", nil)
	}
}

// LogoutHandler handles logout requests
func LogoutHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Clear authentication cookie
		http.SetCookie(w, &http.Cookie{
			Name:     "auth_session",
			Value:    "",
			Path:     "/",
			MaxAge:   -1,
			HttpOnly: true,
		})
		http.Redirect(w, r, "/login", http.StatusSeeOther)
	}
}
