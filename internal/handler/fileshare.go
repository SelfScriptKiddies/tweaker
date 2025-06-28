package handler

import (
	"net/http"
)

// FileshareHandler handles file sharing requests
func FileshareHandler(w http.ResponseWriter, r *http.Request) {
	// TODO: implement file sharing functionality
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Fileshare Handler"))
}
