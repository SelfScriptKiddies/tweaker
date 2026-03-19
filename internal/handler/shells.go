package handler

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

const outputBufMax = 64 * 1024 // 64KB ring buffer per session

type ShellSession struct {
	ID          int       `json:"id"`
	RemoteAddr  string    `json:"remote_addr"`
	ConnectedAt time.Time `json:"connected_at"`

	conn      net.Conn
	wsConn    *websocket.Conn
	wsMu      sync.Mutex
	outputBuf []byte
	bufMu     sync.Mutex
}

type ShellManager struct {
	sessions map[int]*ShellSession
	mu       sync.RWMutex
	nextID   int
	logger   *zap.Logger
	upgrader websocket.Upgrader
	listener net.Listener
	Port     int
}

func NewShellManager(logger *zap.Logger) *ShellManager {
	return &ShellManager{
		sessions: make(map[int]*ShellSession),
		logger:   logger,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		},
	}
}

func (sm *ShellManager) StartListener(port int) error {
	addr := fmt.Sprintf("0.0.0.0:%d", port)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}

	sm.mu.Lock()
	if sm.listener != nil {
		sm.listener.Close()
	}
	sm.listener = ln
	sm.Port = port
	sm.mu.Unlock()

	sm.logger.Info("Shell listener started", zap.String("addr", addr))

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				sm.mu.RLock()
				replaced := sm.listener != ln
				sm.mu.RUnlock()
				if replaced {
					return
				}
				sm.logger.Error("Accept error", zap.Error(err))
				return
			}
			sm.addSession(conn)
		}
	}()

	return nil
}

func (sm *ShellManager) addSession(conn net.Conn) {
	sm.mu.Lock()
	sm.nextID++
	session := &ShellSession{
		ID:          sm.nextID,
		RemoteAddr:  conn.RemoteAddr().String(),
		ConnectedAt: time.Now(),
		conn:        conn,
	}
	sm.sessions[session.ID] = session
	sm.mu.Unlock()

	sm.logger.Info("New shell",
		zap.Int("id", session.ID),
		zap.String("remote", session.RemoteAddr))

	go sm.readTCP(session)
}

func (sm *ShellManager) readTCP(session *ShellSession) {
	buf := make([]byte, 4096)
	for {
		n, err := session.conn.Read(buf)
		if err != nil {
			sm.logger.Info("Shell disconnected",
				zap.Int("id", session.ID),
				zap.String("remote", session.RemoteAddr))
			sm.removeSession(session.ID)
			return
		}

		data := buf[:n]

		// Append to output buffer (ring: keep last 64KB)
		session.bufMu.Lock()
		session.outputBuf = append(session.outputBuf, data...)
		if len(session.outputBuf) > outputBufMax {
			session.outputBuf = session.outputBuf[len(session.outputBuf)-outputBufMax:]
		}
		session.bufMu.Unlock()

		// Forward to WS as binary
		session.wsMu.Lock()
		if session.wsConn != nil {
			session.wsConn.WriteMessage(websocket.BinaryMessage, data)
		}
		session.wsMu.Unlock()
	}
}

func (sm *ShellManager) removeSession(id int) {
	sm.mu.Lock()
	session, ok := sm.sessions[id]
	if ok {
		session.conn.Close()
		session.wsMu.Lock()
		if session.wsConn != nil {
			session.wsConn.Close()
		}
		session.wsMu.Unlock()
		delete(sm.sessions, id)
	}
	sm.mu.Unlock()
}

func (sm *ShellManager) ListHandler(w http.ResponseWriter, r *http.Request) {
	sm.mu.RLock()
	shells := make([]ShellSession, 0, len(sm.sessions))
	for _, s := range sm.sessions {
		shells = append(shells, ShellSession{
			ID:          s.ID,
			RemoteAddr:  s.RemoteAddr,
			ConnectedAt: s.ConnectedAt,
		})
	}
	port := sm.Port
	sm.mu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"shells": shells, "port": port})
}

func (sm *ShellManager) RestartHandler(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Port  int  `json:"port"`
		Force bool `json:"force"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonError(w, "invalid request", http.StatusBadRequest)
		return
	}
	if body.Port < 1 || body.Port > 65535 {
		jsonError(w, "port must be 1-65535", http.StatusBadRequest)
		return
	}

	sm.mu.RLock()
	active := len(sm.sessions)
	sm.mu.RUnlock()

	if active > 0 && !body.Force {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"warning": fmt.Sprintf("%d active session(s) will be disconnected", active),
			"active":  active,
		})
		return
	}

	if err := sm.StartListener(body.Port); err != nil {
		jsonError(w, "failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"status": "ok", "port": body.Port})
}

func (sm *ShellManager) KillHandler(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		jsonError(w, "invalid id", http.StatusBadRequest)
		return
	}

	sm.mu.RLock()
	_, ok := sm.sessions[id]
	sm.mu.RUnlock()

	if !ok {
		jsonError(w, "not found", http.StatusNotFound)
		return
	}

	sm.removeSession(id)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (sm *ShellManager) WebSocketHandler(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	sm.mu.RLock()
	session, ok := sm.sessions[id]
	sm.mu.RUnlock()

	if !ok {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	wsConn, err := sm.upgrader.Upgrade(w, r, nil)
	if err != nil {
		sm.logger.Error("WebSocket upgrade failed", zap.Error(err))
		return
	}

	// Send buffered output as first message
	session.bufMu.Lock()
	if len(session.outputBuf) > 0 {
		replay := make([]byte, len(session.outputBuf))
		copy(replay, session.outputBuf)
		session.bufMu.Unlock()
		wsConn.WriteMessage(websocket.BinaryMessage, replay)
	} else {
		session.bufMu.Unlock()
	}

	// Replace existing WS connection if any
	session.wsMu.Lock()
	if session.wsConn != nil {
		session.wsConn.Close()
	}
	session.wsConn = wsConn
	session.wsMu.Unlock()

	defer func() {
		session.wsMu.Lock()
		if session.wsConn == wsConn {
			session.wsConn = nil
		}
		session.wsMu.Unlock()
		wsConn.Close()
	}()

	// WS -> TCP (binary)
	for {
		_, msg, err := wsConn.ReadMessage()
		if err != nil {
			return
		}
		if _, err := session.conn.Write(msg); err != nil {
			return
		}
	}
}
