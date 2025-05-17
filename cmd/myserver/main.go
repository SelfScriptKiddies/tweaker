package main

import (
	"fmt"
	"log"
	"net/http"
	"github.com/SelfScriptKiddies/tweaker/internal/config"
)

func main() {
	cfg, err := config.Load("config/config.yaml")
	if err != nil {
		log.Fatal(err)
	}

	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	log.Printf("starting server on %s", addr)

	http.HandleFunc("/", HandleIndex)
	http.HandleFunc("/lol", HandleLol)

	// fs := http.FileServer(http.Dir("assets/"))
	// http.Handle("/static/", http.StripPrefix("/static/", fs))

	log.Fatal(http.ListenAndServe(addr, nil))
}
