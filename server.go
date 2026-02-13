package main

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"golang.org/x/crypto/bcrypt"
)

// ============================================================
//  Config
// ============================================================

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// ============================================================
//  Database
// ============================================================

var db *sql.DB

func initDB(path string) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		log.Fatalf("Cannot create data dir %s: %v", dir, err)
	}

	var err error
	db, err = sql.Open("sqlite3", path+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		log.Fatalf("Cannot open database: %v", err)
	}

	migrations := []string{
		`CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			username TEXT UNIQUE NOT NULL,
			password_hash TEXT NOT NULL,
			display_name TEXT NOT NULL,
			state TEXT DEFAULT 'BY',
			default_quota REAL DEFAULT 30,
			is_admin INTEGER DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS quotas (
			user_id INTEGER NOT NULL,
			year INTEGER NOT NULL,
			quota REAL NOT NULL,
			PRIMARY KEY (user_id, year),
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
		)`,
		`CREATE TABLE IF NOT EXISTS absences (
			user_id INTEGER NOT NULL,
			date TEXT NOT NULL,
			type TEXT NOT NULL,
			PRIMARY KEY (user_id, date),
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
		)`,
	}

	for _, m := range migrations {
		if _, err := db.Exec(m); err != nil {
			log.Fatalf("Migration failed: %v\n%s", err, m)
		}
	}

	// Enable foreign keys
	db.Exec("PRAGMA foreign_keys = ON")
}

func seedAdminUser() {
	username := env("URLAUBSPLANER_ADMIN_USER", "admin")
	password := env("URLAUBSPLANER_ADMIN_PASS", "changeme")

	var count int
	db.QueryRow("SELECT COUNT(*) FROM users").Scan(&count)
	if count > 0 {
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		log.Fatalf("Cannot hash password: %v", err)
	}

	_, err = db.Exec(
		"INSERT INTO users (username, password_hash, display_name, state, default_quota, is_admin) VALUES (?, ?, ?, 'BY', 30, 1)",
		username, string(hash), username,
	)
	if err != nil {
		log.Fatalf("Cannot create admin user: %v", err)
	}

	log.Printf("Created admin user '%s'", username)
}

// ============================================================
//  Sessions
// ============================================================

type Session struct {
	UserID    int64
	Username  string
	IsAdmin   bool
	CreatedAt time.Time
}

var (
	sessions   = map[string]*Session{}
	sessionsMu sync.RWMutex
)

func generateToken() string {
	b := make([]byte, 32)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func createSession(userID int64, username string, isAdmin bool) string {
	token := generateToken()
	sessionsMu.Lock()
	sessions[token] = &Session{
		UserID:    userID,
		Username:  username,
		IsAdmin:   isAdmin,
		CreatedAt: time.Now(),
	}
	sessionsMu.Unlock()
	return token
}

func getSession(r *http.Request) *Session {
	cookie, err := r.Cookie("session")
	if err != nil {
		return nil
	}
	sessionsMu.RLock()
	s := sessions[cookie.Value]
	sessionsMu.RUnlock()
	return s
}

func deleteSession(token string) {
	sessionsMu.Lock()
	delete(sessions, token)
	sessionsMu.Unlock()
}

// ============================================================
//  HTTP Helpers
// ============================================================

func jsonResponse(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func jsonError(w http.ResponseWriter, status int, msg string) {
	jsonResponse(w, status, map[string]string{"error": msg})
}

func readJSON(r *http.Request, v any) error {
	defer r.Body.Close()
	return json.NewDecoder(r.Body).Decode(v)
}

func requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if getSession(r) == nil {
			jsonError(w, 401, "Nicht angemeldet")
			return
		}
		next(w, r)
	}
}

func requireAdmin(next http.HandlerFunc) http.HandlerFunc {
	return requireAuth(func(w http.ResponseWriter, r *http.Request) {
		s := getSession(r)
		if !s.IsAdmin {
			jsonError(w, 403, "Keine Berechtigung")
			return
		}
		next(w, r)
	})
}

// ============================================================
//  Auth Handlers
// ============================================================

func handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		jsonError(w, 405, "Method not allowed")
		return
	}

	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := readJSON(r, &req); err != nil {
		jsonError(w, 400, "Ungültige Anfrage")
		return
	}

	req.Username = strings.TrimSpace(req.Username)
	if req.Username == "" || req.Password == "" {
		jsonError(w, 400, "Benutzername und Passwort erforderlich")
		return
	}

	var id int64
	var hash string
	var isAdmin bool
	err := db.QueryRow("SELECT id, password_hash, is_admin FROM users WHERE username = ?", req.Username).
		Scan(&id, &hash, &isAdmin)
	if err != nil {
		jsonError(w, 401, "Ungültige Anmeldedaten")
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(req.Password)); err != nil {
		jsonError(w, 401, "Ungültige Anmeldedaten")
		return
	}

	token := createSession(id, req.Username, isAdmin)
	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   86400 * 30,
	})

	jsonResponse(w, 200, map[string]any{"ok": true})
}

func handleLogout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie("session"); err == nil {
		deleteSession(cookie.Value)
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1,
	})
	jsonResponse(w, 200, map[string]any{"ok": true})
}

func handleMe(w http.ResponseWriter, r *http.Request) {
	s := getSession(r)

	var displayName, state string
	var defaultQuota float64
	var isAdmin bool
	err := db.QueryRow("SELECT display_name, state, default_quota, is_admin FROM users WHERE id = ?", s.UserID).
		Scan(&displayName, &state, &defaultQuota, &isAdmin)
	if err != nil {
		jsonError(w, 500, "Benutzerdaten nicht gefunden")
		return
	}

	jsonResponse(w, 200, map[string]any{
		"username":     s.Username,
		"displayName":  displayName,
		"state":        state,
		"defaultQuota": defaultQuota,
		"isAdmin":      isAdmin,
	})
}

// ============================================================
//  Absences Handlers
// ============================================================

func handleGetAbsences(w http.ResponseWriter, r *http.Request) {
	s := getSession(r)
	yearStr := r.URL.Query().Get("year")

	var rows *sql.Rows
	var err error

	if yearStr != "" {
		rows, err = db.Query("SELECT date, type FROM absences WHERE user_id = ? AND date LIKE ?",
			s.UserID, yearStr+"-%")
	} else {
		rows, err = db.Query("SELECT date, type FROM absences WHERE user_id = ?", s.UserID)
	}
	if err != nil {
		jsonError(w, 500, "Datenbankfehler")
		return
	}
	defer rows.Close()

	result := map[string]string{}
	for rows.Next() {
		var date, typ string
		rows.Scan(&date, &typ)
		result[date] = typ
	}

	jsonResponse(w, 200, result)
}

func handlePutAbsences(w http.ResponseWriter, r *http.Request) {
	s := getSession(r)

	var req struct {
		Dates map[string]string `json:"dates"`
	}
	if err := readJSON(r, &req); err != nil {
		jsonError(w, 400, "Ungültige Anfrage")
		return
	}

	validTypes := map[string]bool{"UR": true, "UR/2": true, "SUR": true, "UUR": true}

	tx, err := db.Begin()
	if err != nil {
		jsonError(w, 500, "Datenbankfehler")
		return
	}

	stmt, _ := tx.Prepare("INSERT OR REPLACE INTO absences (user_id, date, type) VALUES (?, ?, ?)")
	defer stmt.Close()

	for date, typ := range req.Dates {
		if !validTypes[typ] {
			tx.Rollback()
			jsonError(w, 400, fmt.Sprintf("Ungültiger Typ: %s", typ))
			return
		}
		if _, err := time.Parse("2006-01-02", date); err != nil {
			tx.Rollback()
			jsonError(w, 400, fmt.Sprintf("Ungültiges Datum: %s", date))
			return
		}
		stmt.Exec(s.UserID, date, typ)
	}

	tx.Commit()
	jsonResponse(w, 200, map[string]any{"ok": true, "count": len(req.Dates)})
}

func handleDeleteAbsences(w http.ResponseWriter, r *http.Request) {
	s := getSession(r)

	var req struct {
		Dates []string `json:"dates"`
	}
	if err := readJSON(r, &req); err != nil {
		jsonError(w, 400, "Ungültige Anfrage")
		return
	}

	tx, _ := db.Begin()
	stmt, _ := tx.Prepare("DELETE FROM absences WHERE user_id = ? AND date = ?")
	defer stmt.Close()

	for _, date := range req.Dates {
		stmt.Exec(s.UserID, date)
	}

	tx.Commit()
	jsonResponse(w, 200, map[string]any{"ok": true, "count": len(req.Dates)})
}

// ============================================================
//  Quotas Handlers
// ============================================================

func handleGetQuotas(w http.ResponseWriter, r *http.Request) {
	s := getSession(r)

	rows, err := db.Query("SELECT year, quota FROM quotas WHERE user_id = ?", s.UserID)
	if err != nil {
		jsonError(w, 500, "Datenbankfehler")
		return
	}
	defer rows.Close()

	result := map[string]float64{}
	for rows.Next() {
		var year int
		var quota float64
		rows.Scan(&year, &quota)
		result[strconv.Itoa(year)] = quota
	}

	jsonResponse(w, 200, result)
}

func handlePutQuota(w http.ResponseWriter, r *http.Request) {
	s := getSession(r)

	// Extract year from path: /api/quotas/2025
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 4 {
		jsonError(w, 400, "Jahr fehlt")
		return
	}
	yearStr := parts[len(parts)-1]
	year, err := strconv.Atoi(yearStr)
	if err != nil || year < 2000 || year > 2100 {
		jsonError(w, 400, "Ungültiges Jahr")
		return
	}

	var req struct {
		Quota float64 `json:"quota"`
	}
	if err := readJSON(r, &req); err != nil {
		jsonError(w, 400, "Ungültige Anfrage")
		return
	}

	_, err = db.Exec("INSERT OR REPLACE INTO quotas (user_id, year, quota) VALUES (?, ?, ?)",
		s.UserID, year, req.Quota)
	if err != nil {
		jsonError(w, 500, "Datenbankfehler")
		return
	}

	jsonResponse(w, 200, map[string]any{"ok": true})
}

// ============================================================
//  Settings Handler
// ============================================================

func handlePutSettings(w http.ResponseWriter, r *http.Request) {
	s := getSession(r)

	var req struct {
		State        *string  `json:"state"`
		DefaultQuota *float64 `json:"defaultQuota"`
		DisplayName  *string  `json:"displayName"`
	}
	if err := readJSON(r, &req); err != nil {
		jsonError(w, 400, "Ungültige Anfrage")
		return
	}

	if req.State != nil {
		db.Exec("UPDATE users SET state = ? WHERE id = ?", *req.State, s.UserID)
	}
	if req.DefaultQuota != nil {
		db.Exec("UPDATE users SET default_quota = ? WHERE id = ?", *req.DefaultQuota, s.UserID)
	}
	if req.DisplayName != nil {
		db.Exec("UPDATE users SET display_name = ? WHERE id = ?", *req.DisplayName, s.UserID)
	}

	jsonResponse(w, 200, map[string]any{"ok": true})
}

// ============================================================
//  Admin Handlers
// ============================================================

func handleAdminGetUsers(w http.ResponseWriter, r *http.Request) {
	rows, err := db.Query("SELECT id, username, display_name, state, default_quota, is_admin, created_at FROM users ORDER BY id")
	if err != nil {
		jsonError(w, 500, "Datenbankfehler")
		return
	}
	defer rows.Close()

	var users []map[string]any
	for rows.Next() {
		var id int64
		var username, displayName, state, createdAt string
		var defaultQuota float64
		var isAdmin bool
		rows.Scan(&id, &username, &displayName, &state, &defaultQuota, &isAdmin, &createdAt)
		users = append(users, map[string]any{
			"id":           id,
			"username":     username,
			"displayName":  displayName,
			"state":        state,
			"defaultQuota": defaultQuota,
			"isAdmin":      isAdmin,
			"createdAt":    createdAt,
		})
	}

	if users == nil {
		users = []map[string]any{}
	}
	jsonResponse(w, 200, users)
}

func handleAdminCreateUser(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username    string  `json:"username"`
		Password    string  `json:"password"`
		DisplayName string  `json:"displayName"`
		IsAdmin     bool    `json:"isAdmin"`
		State       string  `json:"state"`
		Quota       float64 `json:"defaultQuota"`
	}
	if err := readJSON(r, &req); err != nil {
		jsonError(w, 400, "Ungültige Anfrage")
		return
	}

	req.Username = strings.TrimSpace(req.Username)
	if req.Username == "" || req.Password == "" {
		jsonError(w, 400, "Benutzername und Passwort erforderlich")
		return
	}
	if req.DisplayName == "" {
		req.DisplayName = req.Username
	}
	if req.State == "" {
		req.State = "BY"
	}
	if req.Quota == 0 {
		req.Quota = 30
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		jsonError(w, 500, "Passwort-Hashing fehlgeschlagen")
		return
	}

	res, err := db.Exec(
		"INSERT INTO users (username, password_hash, display_name, state, default_quota, is_admin) VALUES (?, ?, ?, ?, ?, ?)",
		req.Username, string(hash), req.DisplayName, req.State, req.Quota, req.IsAdmin,
	)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			jsonError(w, 409, "Benutzername bereits vergeben")
			return
		}
		jsonError(w, 500, "Datenbankfehler")
		return
	}

	id, _ := res.LastInsertId()
	jsonResponse(w, 201, map[string]any{"id": id, "username": req.Username})
}

func handleAdminDeleteUser(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 4 {
		jsonError(w, 400, "User-ID fehlt")
		return
	}
	idStr := parts[len(parts)-1]
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		jsonError(w, 400, "Ungültige User-ID")
		return
	}

	s := getSession(r)
	if id == s.UserID {
		jsonError(w, 400, "Eigenen Account kann man nicht löschen")
		return
	}

	db.Exec("DELETE FROM absences WHERE user_id = ?", id)
	db.Exec("DELETE FROM quotas WHERE user_id = ?", id)
	db.Exec("DELETE FROM users WHERE id = ?", id)

	jsonResponse(w, 200, map[string]any{"ok": true})
}

func handleAdminResetPassword(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(r.URL.Path, "/")
	// /api/admin/users/{id}/password
	if len(parts) < 5 {
		jsonError(w, 400, "User-ID fehlt")
		return
	}
	idStr := parts[len(parts)-2]
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		jsonError(w, 400, "Ungültige User-ID")
		return
	}

	var req struct {
		Password string `json:"password"`
	}
	if err := readJSON(r, &req); err != nil || req.Password == "" {
		jsonError(w, 400, "Passwort erforderlich")
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		jsonError(w, 500, "Passwort-Hashing fehlgeschlagen")
		return
	}

	db.Exec("UPDATE users SET password_hash = ? WHERE id = ?", string(hash), id)
	jsonResponse(w, 200, map[string]any{"ok": true})
}

// ============================================================
//  Router
// ============================================================

func main() {
	port := env("URLAUBSPLANER_PORT", "8080")
	dbPath := env("URLAUBSPLANER_DB_PATH", "./data/urlaubsplaner.db")

	initDB(dbPath)
	seedAdminUser()

	mux := http.NewServeMux()

	// Auth
	mux.HandleFunc("/api/login", handleLogin)
	mux.HandleFunc("/api/logout", handleLogout)
	mux.HandleFunc("/api/me", requireAuth(handleMe))

	// Absences
	mux.HandleFunc("/api/absences", requireAuth(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "GET":
			handleGetAbsences(w, r)
		case "PUT":
			handlePutAbsences(w, r)
		case "DELETE":
			handleDeleteAbsences(w, r)
		default:
			jsonError(w, 405, "Method not allowed")
		}
	}))

	// Quotas
	mux.HandleFunc("/api/quotas", requireAuth(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			handleGetQuotas(w, r)
		} else {
			jsonError(w, 405, "Method not allowed")
		}
	}))
	mux.HandleFunc("/api/quotas/", requireAuth(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "PUT" {
			handlePutQuota(w, r)
		} else {
			jsonError(w, 405, "Method not allowed")
		}
	}))

	// Settings
	mux.HandleFunc("/api/settings", requireAuth(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "PUT" {
			handlePutSettings(w, r)
		} else {
			jsonError(w, 405, "Method not allowed")
		}
	}))

	// Admin
	mux.HandleFunc("/api/admin/users", requireAdmin(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "GET":
			handleAdminGetUsers(w, r)
		case "POST":
			handleAdminCreateUser(w, r)
		default:
			jsonError(w, 405, "Method not allowed")
		}
	}))
	mux.HandleFunc("/api/admin/users/", requireAdmin(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/password") && r.Method == "PUT" {
			handleAdminResetPassword(w, r)
			return
		}
		if r.Method == "DELETE" {
			handleAdminDeleteUser(w, r)
		} else {
			jsonError(w, 405, "Method not allowed")
		}
	}))

	// Static files — serve index.html for root, otherwise from current directory
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" || r.URL.Path == "/index.html" {
			http.ServeFile(w, r, "index.html")
			return
		}
		http.ServeFile(w, r, "."+r.URL.Path)
	})

	log.Printf("Urlaubsplaner listening on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, mux))
}
