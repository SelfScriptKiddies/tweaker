package main

import (
    "io"
    "net/http"
)

func HandleIndex(w http.ResponseWriter, r *http.Request) {
    io.WriteString(w, "hello, world\n")
}

func HandleLol(w http.ResponseWriter, r *http.Request) {
    io.WriteString(w, "LOLER, AHAHHA!\n")
}

type Result struct {
    FirstName string `json:"first"`
    LastName  string `json:"last"`
}