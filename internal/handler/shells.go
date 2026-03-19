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

type ShellSession struct {
	ID          int       `json:"id"`
	RemoteAddr  string    `json:"remote_addr"`
	ConnectedAt time.Time `json:"connected_at"`

	conn   net.Conn
	wsConn *websocket.Conn
	wsMu   sync.Mutex
}

type ShellManager struct {
	sessions map[int]*ShellSession
	mu       sync.RWMutex
	nextID   int
	logger   *zap.Logger
	upgrader websocket.Upgrader
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
	sm.logger.Info("Shell listener started", zap.String("addr", addr))

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				sm.logger.Error("Accept error", zap.Error(err))
				continue
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
		session.wsMu.Lock()
		if session.wsConn != nil {
			session.wsConn.WriteMessage(websocket.TextMessage, buf[:n])
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
	sm.mu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"shells": shells})
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

	// WS -> TCP
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
