package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/SelfScriptKiddies/tweaker/internal/config"
	"github.com/SelfScriptKiddies/tweaker/internal/middleware"
	"github.com/SelfScriptKiddies/tweaker/internal/template"
)

func main() {
	cfg, err := config.Load("config/config.yaml")
	if err != nil {
		log.Fatal(err)
	}

	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	log.Printf("starting server on %s", addr)

	// Create auth middleware config
	authConfig := middleware.AuthConfig{
		Username:     cfg.Auth.Username,
		Password:     cfg.Auth.Password,
		LoginURL:     "/login",
		SecretCookie: cfg.Auth.SecretCookie,
	}

	// Create router
	mux := http.NewServeMux()

	// Public routes (no auth required)
	mux.HandleFunc("/login", middleware.LoginHandler(authConfig))

	// Protected routes
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		template.RenderTemplate(w, "default", nil)
	})

	// Static files (no auth required)
	fs := http.FileServer(http.Dir("web/static/"))
	mux.Handle("/static/", http.StripPrefix("/static/", fs))

	// Apply auth middleware to all routes
	authMiddleware := middleware.AuthMiddleware(authConfig)

	log.Fatal(http.ListenAndServe(addr, authMiddleware(mux)))
}
