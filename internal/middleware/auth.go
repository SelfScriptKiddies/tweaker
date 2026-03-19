package middleware

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strings"
)

type AuthConfig struct {
	Username     string
	Password     string
	LoginURL     string
	SecretCookie string
}

type RenderFunc func(http.ResponseWriter, string, interface{})

func GenerateRandomHex(length int) (string, error) {
	bytes := make([]byte, length/2)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

func AuthMiddleware(config AuthConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == config.LoginURL ||
				strings.HasPrefix(r.URL.Path, "/static/") ||
				strings.HasPrefix(r.URL.Path, "/dl/") {
				next.ServeHTTP(w, r)
				return
			}

			if !isAuthenticated(r, config) {
				if strings.HasPrefix(r.URL.Path, "/api/") || strings.HasPrefix(r.URL.Path, "/ws/") {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusUnauthorized)
					w.Write([]byte(`{"error":"unauthorized"}`))
					return
				}
				http.Redirect(w, r, config.LoginURL, http.StatusSeeOther)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func isAuthenticated(r *http.Request, config AuthConfig) bool {
	username, password, ok := r.BasicAuth()
	if ok && username == config.Username && password == config.Password {
		return true
	}

	cookie, err := r.Cookie("auth_session")
	if err == nil && cookie.Value == config.SecretCookie {
		return true
	}

	return false
}

func LoginHandler(config AuthConfig, render RenderFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			username := r.FormValue("username")
			password := r.FormValue("password")

			if username == config.Username && password == config.Password {
				http.SetCookie(w, &http.Cookie{
					Name:     "auth_session",
					Value:    config.SecretCookie,
					Path:     "/",
					HttpOnly: true,
					Secure:   false,
				})
				http.Redirect(w, r, "/", http.StatusSeeOther)
				return
			}

			render(w, "login", map[string]string{"Error": "Invalid credentials"})
			return
		}

		render(w, "login", nil)
	}
}

func LogoutHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
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
