package main

import (
	"flag"
	"fmt"
	"io/fs"
	"net/http"
	"os"

	"github.com/SelfScriptKiddies/tweaker/internal/config"
	"github.com/SelfScriptKiddies/tweaker/internal/handler"
	"github.com/SelfScriptKiddies/tweaker/internal/middleware"
	tmpl "github.com/SelfScriptKiddies/tweaker/internal/template"
	"github.com/SelfScriptKiddies/tweaker/web"
	"go.uber.org/zap"
)

func main() {
	configPath := flag.String("c", "config/config.yaml", "config file path")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Config error: %s\n", err)
		os.Exit(1)
	}

	log, err := config.InitLogger(cfg.Log)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Logger error: %s\n", err)
		os.Exit(1)
	}

	// Embedded templates & static files
	tmplFS, err := fs.Sub(web.EmbeddedFS, "templates")
	if err != nil {
		log.Fatal("Template FS error", zap.Error(err))
	}
	staticFS, err := fs.Sub(web.EmbeddedFS, "static")
	if err != nil {
		log.Fatal("Static FS error", zap.Error(err))
	}
	renderer := tmpl.NewRenderer(tmplFS)

	// Handlers
	fileHandler := handler.NewFileHandler(cfg.Files.Directory)
	shellManager := handler.NewShellManager(log)

	if err := shellManager.StartListener(cfg.Shells.ListenPort); err != nil {
		log.Fatal("Shell listener failed", zap.Error(err))
	}

	authConfig := middleware.AuthConfig{
		Username:     cfg.Auth.Username,
		Password:     cfg.Auth.Password,
		LoginURL:     "/login",
		SecretCookie: cfg.Auth.SecretCookie,
	}

	mux := http.NewServeMux()

	// Auth
	mux.HandleFunc("/login", middleware.LoginHandler(authConfig, renderer.Render))
	mux.HandleFunc("GET /logout", middleware.LogoutHandler())

	// Dashboard
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		renderer.Render(w, "default", map[string]interface{}{
			"ShellPort": cfg.Shells.ListenPort,
		})
	})

	// File API
	mux.HandleFunc("GET /api/files", fileHandler.List)
	mux.HandleFunc("POST /api/files/upload", fileHandler.Upload)
	mux.HandleFunc("POST /api/files/mkdir", fileHandler.Mkdir)
	mux.HandleFunc("DELETE /api/files", fileHandler.Delete)
	mux.HandleFunc("POST /api/files/rename", fileHandler.Rename)
	mux.HandleFunc("GET /api/files/download", fileHandler.Download)
	mux.HandleFunc("POST /api/files/serve", fileHandler.ServeTCP)
	mux.HandleFunc("POST /api/files/catch", fileHandler.CatchTCP)
	mux.HandleFunc("GET /api/files/preview", fileHandler.Preview)

	// Shell API
	mux.HandleFunc("GET /api/shells", shellManager.ListHandler)
	mux.HandleFunc("DELETE /api/shells/{id}", shellManager.KillHandler)
	mux.HandleFunc("GET /ws/shell/{id}", shellManager.WebSocketHandler)

	// Static (embedded)
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServerFS(staticFS)))

	// Public file download (for wget/curl from targets)
	mux.Handle("/dl/", http.StripPrefix("/dl/", http.FileServer(http.Dir(cfg.Files.Directory))))

	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	log.Info("Server started",
		zap.String("addr", addr),
		zap.Int("shell_port", cfg.Shells.ListenPort))

	if err := http.ListenAndServe(addr, middleware.AuthMiddleware(authConfig)(mux)); err != nil {
		log.Fatal("Server error", zap.Error(err))
	}
}
