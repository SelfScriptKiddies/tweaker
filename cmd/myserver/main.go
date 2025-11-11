package main

import (
	"fmt"
	"net/http"
	"os"

	"github.com/SelfScriptKiddies/tweaker/internal/config"
	"github.com/SelfScriptKiddies/tweaker/internal/middleware"
	"github.com/SelfScriptKiddies/tweaker/internal/template"
	"go.uber.org/zap"
)

func main() {
	cfg, err := config.Load("config/config.yaml")
	if err != nil {
		fmt.Printf("Config loading error: %s", err)
		os.Exit(1)
	}

	log, err := config.InitLogger(cfg.Log)
	if err != nil {
		fmt.Printf("Initializing logger error: %s", err)
		os.Exit(1)
	}

	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	log.Info("Server started", zap.String("address", addr))

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

	if err := http.ListenAndServe(addr, authMiddleware(mux)); err != nil {
		log.Fatal("Error while running the server: " + err.Error())
	}
}
