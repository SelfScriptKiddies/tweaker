package handler

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type FileHandler struct {
	BaseDir string
}

type FileEntry struct {
	Name  string `json:"name"`
	Size  int64  `json:"size"`
	IsDir bool   `json:"is_dir"`
}

func NewFileHandler(baseDir string) *FileHandler {
	os.MkdirAll(baseDir, 0755)
	return &FileHandler{BaseDir: baseDir}
}

func (h *FileHandler) safePath(requested string) (string, error) {
	if requested == "" {
		requested = "/"
	}
	clean := filepath.Clean("/" + requested)
	full := filepath.Join(h.BaseDir, clean)
	abs, err := filepath.Abs(full)
	if err != nil {
		return "", err
	}
	base, err := filepath.Abs(h.BaseDir)
	if err != nil {
		return "", err
	}
	if abs != base && !strings.HasPrefix(abs, base+string(filepath.Separator)) {
		return "", fmt.Errorf("access denied")
	}
	return abs, nil
}

func (h *FileHandler) List(w http.ResponseWriter, r *http.Request) {
	dirPath := r.URL.Query().Get("path")
	if dirPath == "" {
		dirPath = "/"
	}

	safe, err := h.safePath(dirPath)
	if err != nil {
		jsonError(w, "invalid path", http.StatusBadRequest)
		return
	}

	entries, err := os.ReadDir(safe)
	if err != nil {
		jsonError(w, "cannot read directory", http.StatusInternalServerError)
		return
	}

	files := make([]FileEntry, 0, len(entries))
	for _, e := range entries {
		info, _ := e.Info()
		var size int64
		if info != nil {
			size = info.Size()
		}
		files = append(files, FileEntry{
			Name:  e.Name(),
			Size:  size,
			IsDir: e.IsDir(),
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"path":    dirPath,
		"entries": files,
	})
}

func (h *FileHandler) Upload(w http.ResponseWriter, r *http.Request) {
	dirPath := r.URL.Query().Get("path")
	if dirPath == "" {
		dirPath = "/"
	}

	safe, err := h.safePath(dirPath)
	if err != nil {
		jsonError(w, "invalid path", http.StatusBadRequest)
		return
	}

	r.ParseMultipartForm(100 << 20) // 100 MB

	file, header, err := r.FormFile("file")
	if err != nil {
		jsonError(w, "no file provided", http.StatusBadRequest)
		return
	}
	defer file.Close()

	destPath := filepath.Join(safe, filepath.Base(header.Filename))
	dst, err := os.Create(destPath)
	if err != nil {
		jsonError(w, "cannot create file", http.StatusInternalServerError)
		return
	}
	defer dst.Close()

	io.Copy(dst, file)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (h *FileHandler) Download(w http.ResponseWriter, r *http.Request) {
	filePath := r.URL.Query().Get("path")
	safe, err := h.safePath(filePath)
	if err != nil {
		http.Error(w, "invalid path", http.StatusBadRequest)
		return
	}

	info, err := os.Stat(safe)
	if err != nil || info.IsDir() {
		http.Error(w, "file not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", info.Name()))
	http.ServeFile(w, r, safe)
}

func (h *FileHandler) Mkdir(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Path string `json:"path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonError(w, "invalid request", http.StatusBadRequest)
		return
	}

	safe, err := h.safePath(body.Path)
	if err != nil {
		jsonError(w, "invalid path", http.StatusBadRequest)
		return
	}

	if err := os.MkdirAll(safe, 0755); err != nil {
		jsonError(w, "cannot create directory", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (h *FileHandler) Delete(w http.ResponseWriter, r *http.Request) {
	filePath := r.URL.Query().Get("path")
	safe, err := h.safePath(filePath)
	if err != nil {
		jsonError(w, "invalid path", http.StatusBadRequest)
		return
	}

	base, _ := filepath.Abs(h.BaseDir)
	if safe == base {
		jsonError(w, "cannot delete root", http.StatusBadRequest)
		return
	}

	if err := os.RemoveAll(safe); err != nil {
		jsonError(w, "cannot delete", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (h *FileHandler) Rename(w http.ResponseWriter, r *http.Request) {
	var body struct {
		OldPath string `json:"old_path"`
		NewPath string `json:"new_path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonError(w, "invalid request", http.StatusBadRequest)
		return
	}

	oldSafe, err := h.safePath(body.OldPath)
	if err != nil {
		jsonError(w, "invalid old path", http.StatusBadRequest)
		return
	}

	newSafe, err := h.safePath(body.NewPath)
	if err != nil {
		jsonError(w, "invalid new path", http.StatusBadRequest)
		return
	}

	if err := os.Rename(oldSafe, newSafe); err != nil {
		jsonError(w, "cannot rename", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// ServeTCP starts a one-shot TCP listener that sends a file to the first connection.
// Used for /dev/tcp and nc download methods on targets without wget/curl.
func (h *FileHandler) ServeTCP(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Path string `json:"path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonError(w, "invalid request", http.StatusBadRequest)
		return
	}

	safe, err := h.safePath(body.Path)
	if err != nil {
		jsonError(w, "invalid path", http.StatusBadRequest)
		return
	}

	info, err := os.Stat(safe)
	if err != nil || info.IsDir() {
		jsonError(w, "file not found", http.StatusNotFound)
		return
	}

	ln, err := net.Listen("tcp", "0.0.0.0:0")
	if err != nil {
		jsonError(w, "cannot start listener", http.StatusInternalServerError)
		return
	}
	port := ln.Addr().(*net.TCPAddr).Port

	go func() {
		defer ln.Close()
		ln.(*net.TCPListener).SetDeadline(time.Now().Add(5 * time.Minute))
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		f, err := os.Open(safe)
		if err != nil {
			return
		}
		defer f.Close()
		io.Copy(conn, f)
	}()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"port": port})
}

// CatchTCP starts a one-shot TCP listener that receives data and saves it as a file.
// Used for exfiltrating files from targets via /dev/tcp or nc.
func (h *FileHandler) CatchTCP(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Filename string `json:"filename"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonError(w, "invalid request", http.StatusBadRequest)
		return
	}
	if body.Filename == "" {
		body.Filename = fmt.Sprintf("caught_%d", time.Now().Unix())
	}

	ln, err := net.Listen("tcp", "0.0.0.0:0")
	if err != nil {
		jsonError(w, "cannot start listener", http.StatusInternalServerError)
		return
	}
	port := ln.Addr().(*net.TCPAddr).Port

	go func() {
		defer ln.Close()
		ln.(*net.TCPListener).SetDeadline(time.Now().Add(5 * time.Minute))
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()

		savePath := filepath.Join(h.BaseDir, filepath.Base(body.Filename))
		f, err := os.Create(savePath)
		if err != nil {
			return
		}
		defer f.Close()
		io.Copy(f, conn)
	}()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"port": port})
}

// Preview returns file content as text or formatted hex dump in chunks.
// Query params: path, mode (text|hex), offset (byte offset, default 0).
func (h *FileHandler) Preview(w http.ResponseWriter, r *http.Request) {
	filePath := r.URL.Query().Get("path")
	mode := r.URL.Query().Get("mode")
	if mode == "" {
		mode = "text"
	}

	offset, _ := strconv.ParseInt(r.URL.Query().Get("offset"), 10, 64)
	if offset < 0 {
		offset = 0
	}

	safe, err := h.safePath(filePath)
	if err != nil {
		jsonError(w, "invalid path", http.StatusBadRequest)
		return
	}

	info, err := os.Stat(safe)
	if err != nil || info.IsDir() {
		jsonError(w, "file not found", http.StatusNotFound)
		return
	}

	if offset >= info.Size() {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"content":  "",
			"offset":   offset,
			"read":     0,
			"size":     info.Size(),
			"has_more": false,
		})
		return
	}

	// Align offset to 16 bytes for hex mode
	if mode == "hex" {
		offset = offset - (offset % 16)
	}

	const chunkSize = 64 * 1024 // 64 KB

	f, err := os.Open(safe)
	if err != nil {
		jsonError(w, "cannot read file", http.StatusInternalServerError)
		return
	}
	defer f.Close()

	if offset > 0 {
		f.Seek(offset, io.SeekStart)
	}

	data := make([]byte, chunkSize)
	n, _ := f.Read(data)
	data = data[:n]

	hasMore := offset+int64(n) < info.Size()

	w.Header().Set("Content-Type", "application/json")

	if mode == "hex" {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"data":     base64.StdEncoding.EncodeToString(data),
			"offset":   offset,
			"read":     n,
			"size":     info.Size(),
			"has_more": hasMore,
		})
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"content":  strings.ToValidUTF8(string(data), "\ufffd"),
		"offset":   offset,
		"read":     n,
		"size":     info.Size(),
		"has_more": hasMore,
	})
}

func jsonError(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
