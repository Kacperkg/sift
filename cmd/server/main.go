package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"job-inbox/internal/config"
	"job-inbox/internal/securestore"
)

const (
	readOnlyScope = "https://www.googleapis.com/auth/gmail.readonly"
	keychainSvc   = "com.job-inbox.local"
)

type application struct {
	ID        string `json:"id"`
	ThreadID  string `json:"threadId,omitempty"`
	Company   string `json:"company"`
	Role      string `json:"role"`
	Status    string `json:"status"`
	LastEvent string `json:"lastEvent"`
	UpdatedAt string `json:"updatedAt"`
}

type syncState struct {
	LastSuccessfulAt string `json:"lastSuccessfulAt"`
}

type blockedRole struct {
	Role      string `json:"role,omitempty"`
	MessageID string `json:"messageId,omitempty"`
	Label     string `json:"label,omitempty"`
	CreatedAt string `json:"createdAt"`
}

type syncStatus struct {
	Running   bool   `json:"running"`
	Phase     string `json:"phase"`
	Processed int    `json:"processed"`
	Total     int    `json:"total"`
}

type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	Error        string `json:"error"`
}

type server struct {
	clientID     string
	clientSecret string
	redirectURL  string
	appOrigin    string
	store        *securestore.Store
	blocksStore  *securestore.Store
	syncStore    *securestore.Store
	dataMu       sync.Mutex
	mu           sync.Mutex
	states       map[string]time.Time
	accessToken  string
	tokenExpiry  time.Time
	localAI      *localAI
	progressMu   sync.RWMutex
	syncStatus   syncStatus
}

func main() {
	if err := config.LoadProjectEnv(); err != nil {
		log.Printf("warning: could not load .env: %v", err)
	}

	dataDir, err := os.UserConfigDir()
	if err != nil {
		log.Fatal(err)
	}
	if override := os.Getenv("JOB_INBOX_DATA_DIR"); override != "" {
		dataDir = override
	}
	store, err := securestore.New(filepath.Join(dataDir, "Job Inbox", "jobs.enc"), keychainSvc)
	if err != nil {
		log.Fatal(err)
	}
	blocksStore, err := securestore.New(filepath.Join(dataDir, "Job Inbox", "blocked-roles.enc"), keychainSvc)
	if err != nil {
		log.Fatal(err)
	}
	syncStore, err := securestore.New(filepath.Join(dataDir, "Job Inbox", "sync-state.enc"), keychainSvc)
	if err != nil {
		log.Fatal(err)
	}
	localAI, err := newLocalAI(
		env("OLLAMA_URL", "http://127.0.0.1:11434"),
		env("OLLAMA_MODEL", "qwen3:4b"),
		strings.EqualFold(os.Getenv("LOCAL_AI_ENABLED"), "true"),
	)
	if err != nil {
		log.Fatal(err)
	}

	s := &server{
		clientID: os.Getenv("GOOGLE_CLIENT_ID"), clientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
		redirectURL: env("OAUTH_REDIRECT_URL", "http://127.0.0.1:8787/auth/callback"),
		appOrigin:   env("APP_ORIGIN", "http://localhost:5173"), store: store, blocksStore: blocksStore, syncStore: syncStore,
		states: map[string]time.Time{}, localAI: localAI,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/health", s.health)
	mux.HandleFunc("/api/auth/status", s.authStatus)
	mux.HandleFunc("/api/applications", s.applications)
	mux.HandleFunc("/api/sync", s.sync)
	mux.HandleFunc("/api/sync/status", s.syncProgressStatus)
	mux.HandleFunc("/api/blocked-roles", s.blockedRoles)
	mux.HandleFunc("/api/ai/status", s.aiStatus)
	mux.HandleFunc("/auth/google", s.startAuth)
	mux.HandleFunc("/auth/callback", s.callback)
	mux.Handle("/", spaHandler("web/dist"))

	addr := "127.0.0.1:" + env("PORT", "8787")
	log.Printf("Job Inbox is running at http://%s", addr)
	log.Fatal(http.ListenAndServe(addr, s.securityHeaders(s.cors(mux))))
}

func env(k, fallback string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return fallback
}

func (s *server) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, 200, map[string]bool{"ok": true})
}

func (s *server) authStatus(w http.ResponseWriter, _ *http.Request) {
	_, err := securestore.GetSecret(keychainSvc, "gmail-refresh-token")
	writeJSON(w, 200, map[string]bool{"connected": err == nil})
}

func (s *server) aiStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if r.Method == http.MethodPost {
		var update struct {
			Enabled bool `json:"enabled"`
		}
		if err := json.NewDecoder(io.LimitReader(r.Body, 1024)).Decode(&update); err != nil {
			http.Error(w, "invalid settings", http.StatusBadRequest)
			return
		}
		if update.Enabled {
			ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
			available, installed := s.localAI.availability(ctx)
			cancel()
			if !available {
				http.Error(w, "Ollama is offline. Start Ollama first.", http.StatusServiceUnavailable)
				return
			}
			if !installed {
				http.Error(w, "The configured Ollama model is not installed.", http.StatusServiceUnavailable)
				return
			}
		}
		s.localAI.setEnabled(update.Enabled)
	}
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	available, installed := s.localAI.availability(ctx)
	cancel()
	writeJSON(w, http.StatusOK, map[string]any{
		"enabled":        s.localAI.isEnabled(),
		"available":      available,
		"modelInstalled": installed,
		"model":          s.localAI.model,
	})
}

func (s *server) startAuth(w http.ResponseWriter, r *http.Request) {
	if s.clientID == "" || s.clientSecret == "" {
		http.Error(w, "Google OAuth is not configured", 503)
		return
	}
	state, err := securestore.RandomToken(32)
	if err != nil {
		http.Error(w, "could not create OAuth state", 500)
		return
	}
	s.mu.Lock()
	s.states[state] = time.Now().Add(10 * time.Minute)
	s.mu.Unlock()
	q := url.Values{"client_id": {s.clientID}, "redirect_uri": {s.redirectURL}, "response_type": {"code"},
		"scope": {readOnlyScope}, "access_type": {"offline"}, "prompt": {"consent"}, "state": {state}}
	http.Redirect(w, r, "https://accounts.google.com/o/oauth2/v2/auth?"+q.Encode(), http.StatusFound)
}

func (s *server) callback(w http.ResponseWriter, r *http.Request) {
	state := r.URL.Query().Get("state")
	s.mu.Lock()
	expiry, ok := s.states[state]
	delete(s.states, state)
	s.mu.Unlock()
	if !ok || time.Now().After(expiry) {
		http.Error(w, "invalid or expired OAuth state", 400)
		return
	}
	if oauthErr := r.URL.Query().Get("error"); oauthErr != "" {
		http.Error(w, "Google authorization failed: "+oauthErr, 400)
		return
	}
	tok, err := s.exchangeCode(r.Context(), r.URL.Query().Get("code"))
	if err != nil {
		http.Error(w, err.Error(), 502)
		return
	}
	if tok.RefreshToken != "" {
		if err := securestore.SetSecret(keychainSvc, "gmail-refresh-token", tok.RefreshToken); err != nil {
			http.Error(w, "could not store token in Keychain", 500)
			return
		}
	}
	s.mu.Lock()
	s.accessToken = tok.AccessToken
	s.tokenExpiry = time.Now().Add(time.Duration(tok.ExpiresIn-60) * time.Second)
	s.mu.Unlock()
	http.Redirect(w, r, s.appOrigin+"/?connected=1", http.StatusFound)
}

func (s *server) exchangeCode(ctx context.Context, code string) (tokenResponse, error) {
	return s.tokenRequest(ctx, url.Values{"code": {code}, "client_id": {s.clientID}, "client_secret": {s.clientSecret},
		"redirect_uri": {s.redirectURL}, "grant_type": {"authorization_code"}})
}

func (s *server) tokenRequest(ctx context.Context, form url.Values) (tokenResponse, error) {
	req, _ := http.NewRequestWithContext(ctx, "POST", "https://oauth2.googleapis.com/token", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return tokenResponse{}, err
	}
	defer resp.Body.Close()
	var tok tokenResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&tok); err != nil {
		return tok, err
	}
	if resp.StatusCode != 200 || tok.Error != "" {
		return tok, fmt.Errorf("Google token exchange failed: %s", tok.Error)
	}
	return tok, nil
}

func (s *server) validAccessToken(ctx context.Context) (string, error) {
	s.mu.Lock()
	if s.accessToken != "" && time.Now().Before(s.tokenExpiry) {
		token := s.accessToken
		s.mu.Unlock()
		return token, nil
	}
	s.mu.Unlock()
	refresh, err := securestore.GetSecret(keychainSvc, "gmail-refresh-token")
	if err != nil {
		return "", errors.New("Gmail is not connected")
	}
	tok, err := s.tokenRequest(ctx, url.Values{"refresh_token": {refresh}, "client_id": {s.clientID},
		"client_secret": {s.clientSecret}, "grant_type": {"refresh_token"}})
	if err != nil {
		return "", err
	}
	s.mu.Lock()
	s.accessToken = tok.AccessToken
	s.tokenExpiry = time.Now().Add(time.Duration(tok.ExpiresIn-60) * time.Second)
	s.mu.Unlock()
	return tok.AccessToken, nil
}

func (s *server) applications(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", 405)
		return
	}
	s.dataMu.Lock()
	defer s.dataMu.Unlock()
	var apps []application
	if err := s.store.Load(&apps); err != nil && !errors.Is(err, os.ErrNotExist) {
		http.Error(w, "could not read local data", 500)
		return
	}
	if apps == nil {
		apps = []application{}
	}
	writeJSON(w, 200, apps)
}

func (s *server) blockedRoles(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	s.dataMu.Lock()
	defer s.dataMu.Unlock()
	blocks, err := s.loadBlockedRoles()
	if err != nil {
		http.Error(w, "could not read blocked roles", http.StatusInternalServerError)
		return
	}
	if r.Method == http.MethodPost {
		var update struct {
			Role      string `json:"role"`
			MessageID string `json:"messageId"`
			Label     string `json:"label"`
			Blocked   bool   `json:"blocked"`
		}
		if err := json.NewDecoder(io.LimitReader(r.Body, 2048)).Decode(&update); err != nil {
			http.Error(w, "invalid block setting", http.StatusBadRequest)
			return
		}
		update.Role = strings.TrimSpace(strings.Join(strings.Fields(update.Role), " "))
		update.MessageID = strings.TrimSpace(update.MessageID)
		update.Label = strings.TrimSpace(strings.Join(strings.Fields(update.Label), " "))
		if (update.Role == "" && update.MessageID == "") || len(update.Role) > 140 || len(update.MessageID) > 100 || len(update.Label) > 180 {
			http.Error(w, "invalid blocked item", http.StatusBadRequest)
			return
		}
		blocks = updateBlockedItems(blocks, update.Role, update.MessageID, update.Label, update.Blocked)
		if err := s.blocksStore.Save(blocks); err != nil {
			http.Error(w, "could not save blocked roles", http.StatusInternalServerError)
			return
		}
		if update.Blocked {
			var apps []application
			if err := s.store.Load(&apps); err != nil && !errors.Is(err, os.ErrNotExist) {
				http.Error(w, "could not read applications", http.StatusInternalServerError)
				return
			}
			apps = filterBlockedApplications(apps, blocks)
			if err := s.store.Save(apps); err != nil {
				http.Error(w, "could not update applications", http.StatusInternalServerError)
				return
			}
		}
	}
	writeJSON(w, http.StatusOK, blocks)
}

func (s *server) loadBlockedRoles() ([]blockedRole, error) {
	var blocks []blockedRole
	if err := s.blocksStore.Load(&blocks); err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	if blocks == nil {
		blocks = []blockedRole{}
	}
	return blocks, nil
}

func (s *server) sync(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}
	if !s.beginSync() {
		http.Error(w, "a Gmail scan is already running", http.StatusConflict)
		return
	}
	finished := false
	defer func() {
		if !finished {
			s.finishSync("Scan stopped")
		}
	}()
	token, err := s.validAccessToken(r.Context())
	if err != nil {
		http.Error(w, err.Error(), 401)
		return
	}
	fullScan := strings.EqualFold(r.URL.Query().Get("mode"), "full")
	var existing []application
	var state syncState
	if !fullScan {
		s.dataMu.Lock()
		loadErr := s.store.Load(&existing)
		if loadErr == nil || errors.Is(loadErr, os.ErrNotExist) {
			loadErr = s.syncStore.Load(&state)
		}
		s.dataMu.Unlock()
		if loadErr != nil && !errors.Is(loadErr, os.ErrNotExist) {
			http.Error(w, "could not read local sync state", http.StatusInternalServerError)
			return
		}
	}

	var after time.Time
	if !fullScan && state.LastSuccessfulAt != "" {
		if parsed, parseErr := time.Parse(time.RFC3339, state.LastSuccessfulAt); parseErr == nil {
			// Small overlap avoids missing messages that arrive on the timestamp boundary.
			after = parsed.Add(-5 * time.Minute)
		}
	}
	if after.IsZero() {
		fullScan = true
	}

	found, err := scanGmail(r.Context(), token, s.localAI, after, s.reportSyncProgress)
	if err != nil {
		http.Error(w, err.Error(), 502)
		return
	}
	s.dataMu.Lock()
	defer s.dataMu.Unlock()
	blocks, err := s.loadBlockedRoles()
	if err != nil {
		http.Error(w, "could not read blocked roles", 500)
		return
	}
	apps := found
	if !fullScan {
		apps = mergeApplications(existing, found)
	}
	apps = filterBlockedApplications(apps, blocks)
	s.reportSyncProgress(scanProgress{Phase: "Saving private index", Processed: len(apps), Total: len(apps)})
	if err := s.store.Save(apps); err != nil {
		http.Error(w, "could not save encrypted data", 500)
		return
	}
	if err := s.syncStore.Save(syncState{LastSuccessfulAt: time.Now().UTC().Format(time.RFC3339)}); err != nil {
		http.Error(w, "could not save sync state", 500)
		return
	}
	completion := "Inbox updated"
	if fullScan {
		completion = "Full scan complete"
	}
	s.finishSync(completion)
	finished = true
	writeJSON(w, 200, apps)
}

func (s *server) syncProgressStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	s.progressMu.RLock()
	status := s.syncStatus
	s.progressMu.RUnlock()
	writeJSON(w, http.StatusOK, status)
}

func (s *server) beginSync() bool {
	s.progressMu.Lock()
	defer s.progressMu.Unlock()
	if s.syncStatus.Running {
		return false
	}
	s.syncStatus = syncStatus{Running: true, Phase: "Starting scan"}
	return true
}

func (s *server) reportSyncProgress(progress scanProgress) {
	s.progressMu.Lock()
	s.syncStatus = syncStatus{Running: true, Phase: progress.Phase, Processed: progress.Processed, Total: progress.Total}
	s.progressMu.Unlock()
}

func (s *server) finishSync(phase string) {
	s.progressMu.Lock()
	s.syncStatus.Running = false
	s.syncStatus.Phase = phase
	s.progressMu.Unlock()
}

func normalizeRole(role string) string {
	return strings.ToLower(strings.Join(strings.Fields(role), " "))
}

func updateBlockedItems(blocks []blockedRole, role, messageID, label string, blocked bool) []blockedRole {
	item := blockedRole{Role: role, MessageID: messageID, Label: label}
	key := blockedItemKey(item)
	result := make([]blockedRole, 0, len(blocks)+1)
	found := false
	for _, existing := range blocks {
		if blockedItemKey(existing) == key {
			found = true
			if !blocked {
				continue
			}
		}
		result = append(result, existing)
	}
	if blocked && !found {
		item.CreatedAt = time.Now().Format(time.RFC3339)
		result = append(result, item)
	}
	return result
}

func blockedItemKey(item blockedRole) string {
	if item.MessageID != "" {
		return "message:" + strings.ToLower(item.MessageID)
	}
	return "role:" + normalizeRole(item.Role)
}

func filterBlockedApplications(apps []application, blocks []blockedRole) []application {
	blockedRoles := make(map[string]bool, len(blocks))
	blockedMessages := make(map[string]bool, len(blocks))
	for _, item := range blocks {
		if item.MessageID != "" {
			blockedMessages[strings.ToLower(item.MessageID)] = true
		} else if item.Role != "" {
			blockedRoles[normalizeRole(item.Role)] = true
		}
	}
	result := make([]application, 0, len(apps))
	for _, app := range apps {
		if !blockedMessages[strings.ToLower(app.ID)] && !blockedRoles[normalizeRole(app.Role)] {
			result = append(result, app)
		}
	}
	return result
}

func (s *server) securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Security-Policy", "default-src 'self'; style-src 'self' 'unsafe-inline'; connect-src 'self' http://127.0.0.1:8787; img-src 'self' data:; frame-ancestors 'none'")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		next.ServeHTTP(w, r)
	})
}

func (s *server) cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		localOrigin := "http://" + r.Host
		allowedOrigin := origin == "" || origin == s.appOrigin || origin == localOrigin
		if r.Method != http.MethodGet && r.Method != http.MethodHead && !allowedOrigin {
			http.Error(w, "origin not allowed", http.StatusForbidden)
			return
		}
		if origin == s.appOrigin {
			w.Header().Set("Access-Control-Allow-Origin", s.appOrigin)
		}
		if r.Method == http.MethodOptions {
			w.Header().Set("Access-Control-Allow-Methods", "GET,POST,OPTIONS")
			w.WriteHeader(204)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func spaHandler(root string) http.Handler {
	fs := http.FileServer(http.Dir(root))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := filepath.Join(root, filepath.Clean(r.URL.Path))
		if _, err := os.Stat(path); err != nil {
			r.URL.Path = "/"
		}
		fs.ServeHTTP(w, r)
	})
}
