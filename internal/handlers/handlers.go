package handlers

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"html/template"
	"log"
	"net/http"
	"strconv"
	"sync"
	"time"

	"flamarr/internal/db"
	"golang.org/x/crypto/bcrypt"
)

const sessionCookie = "flamarr_session"

type Handler struct {
	db       *db.DB
	webFS    embed.FS
	tmpl     map[string]*template.Template
	sessions sync.Map // token -> userID
	hmacKey  []byte
}

func New(database *db.DB, webFS embed.FS) *Handler {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		panic(err)
	}
	h := &Handler{db: database, webFS: webFS, hmacKey: key, tmpl: make(map[string]*template.Template)}
	h.loadTemplates()
	return h
}

func (h *Handler) loadTemplates() {
	pages := []string{"setup", "login", "dashboard", "forgot_password", "forgot_username"}
	for _, p := range pages {
		t := template.Must(template.ParseFS(h.webFS,
			"web/templates/layout.html",
			"web/templates/"+p+".html",
		))
		h.tmpl[p] = t
	}
}

func (h *Handler) render(w http.ResponseWriter, page string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := h.tmpl[page].ExecuteTemplate(w, "layout", data); err != nil {
		log.Printf("template error: %v", err)
		http.Error(w, "render error", 500)
	}
}

func (h *Handler) sign(token string) string {
	mac := hmac.New(sha256.New, h.hmacKey)
	mac.Write([]byte(token))
	return token + "." + hex.EncodeToString(mac.Sum(nil))
}

func (h *Handler) verify(signed string) (string, bool) {
	if len(signed) < 65 {
		return "", false
	}
	sep := len(signed) - 65
	token := signed[:sep]
	expected := h.sign(token)
	return token, hmac.Equal([]byte(signed), []byte(expected))
}

func (h *Handler) newSession(userID int64) string {
	b := make([]byte, 16)
	rand.Read(b)
	token := hex.EncodeToString(b)
	h.sessions.Store(token, userID)
	return h.sign(token)
}

func (h *Handler) sessionUserID(r *http.Request) (int64, bool) {
	c, err := r.Cookie(sessionCookie)
	if err != nil {
		return 0, false
	}
	token, ok := h.verify(c.Value)
	if !ok {
		return 0, false
	}
	v, ok := h.sessions.Load(token)
	if !ok {
		return 0, false
	}
	return v.(int64), true
}

func (h *Handler) AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hasUsers, _ := h.db.HasUsers()
		if !hasUsers {
			http.Redirect(w, r, "/setup", http.StatusFound)
			return
		}
		if _, ok := h.sessionUserID(r); !ok {
			http.Redirect(w, r, "/login", http.StatusFound)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// Setup

type setupData struct {
	Error string
}

func (h *Handler) Setup(w http.ResponseWriter, r *http.Request) {
	hasUsers, _ := h.db.HasUsers()
	if hasUsers {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}
	h.render(w, "setup", setupData{})
}

func (h *Handler) HandleSetup(w http.ResponseWriter, r *http.Request) {
	hasUsers, _ := h.db.HasUsers()
	if hasUsers {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}
	username := r.FormValue("username")
	password := r.FormValue("password")
	if len(username) < 2 || len(password) < 4 {
		h.render(w, "setup", setupData{Error: "Username must be 2+ chars, password 4+ chars."})
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		h.render(w, "setup", setupData{Error: "Internal error."})
		return
	}
	if err := h.db.CreateUser(username, string(hash)); err != nil {
		h.render(w, "setup", setupData{Error: "Could not create user."})
		return
	}
	http.Redirect(w, r, "/login", http.StatusFound)
}

// Login

type loginData struct {
	Error string
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	hasUsers, _ := h.db.HasUsers()
	if !hasUsers {
		http.Redirect(w, r, "/setup", http.StatusFound)
		return
	}
	if _, ok := h.sessionUserID(r); ok {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}
	h.render(w, "login", loginData{})
}

func (h *Handler) HandleLogin(w http.ResponseWriter, r *http.Request) {
	username := r.FormValue("username")
	password := r.FormValue("password")
	user, err := h.db.GetUserByUsername(username)
	if err != nil || user == nil {
		h.render(w, "login", loginData{Error: "Invalid credentials."})
		return
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		h.render(w, "login", loginData{Error: "Invalid credentials."})
		return
	}
	signed := h.newSession(user.ID)
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    signed,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Now().Add(30 * 24 * time.Hour),
	})
	http.Redirect(w, r, "/", http.StatusFound)
}

// Forgot password: allow resetting password by providing username

type forgotPasswordData struct {
	Error string
}

func (h *Handler) ForgotPassword(w http.ResponseWriter, r *http.Request) {
	h.render(w, "forgot_password", forgotPasswordData{})
}

func (h *Handler) HandleForgotPassword(w http.ResponseWriter, r *http.Request) {
	username := r.FormValue("username")
	password := r.FormValue("password")
	confirm := r.FormValue("confirm")
	if password == "" || password != confirm {
		h.render(w, "forgot_password", forgotPasswordData{Error: "Passwords must match and not be empty."})
		return
	}
	user, err := h.db.GetUserByUsername(username)
	if err != nil || user == nil {
		h.render(w, "forgot_password", forgotPasswordData{Error: "Unknown username."})
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		h.render(w, "forgot_password", forgotPasswordData{Error: "Internal error."})
		return
	}
	if err := h.db.UpdatePassword(user.ID, string(hash)); err != nil {
		h.render(w, "forgot_password", forgotPasswordData{Error: "Could not update password."})
		return
	}
	http.Redirect(w, r, "/login", http.StatusFound)
}

// Forgot username: allow changing username if you know the password

type forgotUsernameData struct {
	Error string
}

func (h *Handler) ForgotUsername(w http.ResponseWriter, r *http.Request) {
	h.render(w, "forgot_username", forgotUsernameData{})
}

func (h *Handler) HandleForgotUsername(w http.ResponseWriter, r *http.Request) {
	password := r.FormValue("password")
	newUsername := r.FormValue("username")
	confirm := r.FormValue("confirm")
	if newUsername == "" || newUsername != confirm {
		h.render(w, "forgot_username", forgotUsernameData{Error: "Usernames must match and not be empty."})
		return
	}
	users, err := h.db.GetAllUsers()
	if err != nil {
		h.render(w, "forgot_username", forgotUsernameData{Error: "Internal error."})
		return
	}
	var matched *db.User
	for _, u := range users {
		if bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)) == nil {
			matched = &u
			break
		}
	}
	if matched == nil {
		h.render(w, "forgot_username", forgotUsernameData{Error: "No user matches that password."})
		return
	}
	// check new username availability
	existing, err := h.db.GetUserByUsername(newUsername)
	if err == nil && existing != nil && existing.ID != matched.ID {
		h.render(w, "forgot_username", forgotUsernameData{Error: "Username already taken."})
		return
	}
	if err := h.db.UpdateUsername(matched.ID, newUsername); err != nil {
		h.render(w, "forgot_username", forgotUsernameData{Error: "Could not update username."})
		return
	}
	http.Redirect(w, r, "/login", http.StatusFound)
}

func (h *Handler) HandleLogout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(sessionCookie); err == nil {
		if token, ok := h.verify(c.Value); ok {
			h.sessions.Delete(token)
		}
	}
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: "", Path: "/", MaxAge: -1})
	http.Redirect(w, r, "/login", http.StatusFound)
}

// Dashboard

type dashboardData struct {
	Services  []db.Item
	Bookmarks []db.Item
}

func (h *Handler) Dashboard(w http.ResponseWriter, r *http.Request) {
	items, err := h.db.GetItems()
	if err != nil {
		http.Error(w, "db error", 500)
		return
	}
	data := dashboardData{}
	for _, it := range items {
		if it.Type == "service" {
			data.Services = append(data.Services, it)
		} else {
			data.Bookmarks = append(data.Bookmarks, it)
		}
	}
	h.render(w, "dashboard", data)
}

// API

func jsonOK(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func jsonErr(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

func (h *Handler) GetItems(w http.ResponseWriter, r *http.Request) {
	items, err := h.db.GetItems()
	if err != nil {
		jsonErr(w, "db error", 500)
		return
	}
	if items == nil {
		items = []db.Item{}
	}
	jsonOK(w, items)
}

func (h *Handler) CreateItem(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name        string `json:"name"`
		URL         string `json:"url"`
		Icon        string `json:"icon"`
		Description string `json:"description"`
		Type        string `json:"type"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonErr(w, "bad request", 400)
		return
	}
	if body.Name == "" || body.URL == "" {
		jsonErr(w, "name and url required", 400)
		return
	}
	if body.Type != "service" && body.Type != "bookmark" {
		body.Type = "service"
	}
	item, err := h.db.CreateItem(body.Name, body.URL, body.Icon, body.Description, body.Type)
	if err != nil {
		jsonErr(w, "db error", 500)
		return
	}
	w.WriteHeader(http.StatusCreated)
	jsonOK(w, item)
}

func (h *Handler) UpdateItem(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		jsonErr(w, "bad id", 400)
		return
	}
	var body struct {
		Name        string `json:"name"`
		URL         string `json:"url"`
		Icon        string `json:"icon"`
		Description string `json:"description"`
		Type        string `json:"type"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonErr(w, "bad request", 400)
		return
	}
	if body.Type != "service" && body.Type != "bookmark" {
		body.Type = "service"
	}
	if err := h.db.UpdateItem(id, body.Name, body.URL, body.Icon, body.Description, body.Type); err != nil {
		jsonErr(w, "db error", 500)
		return
	}
	jsonOK(w, map[string]bool{"ok": true})
}

func (h *Handler) DeleteItem(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		jsonErr(w, "bad id", 400)
		return
	}
	if err := h.db.DeleteItem(id); err != nil {
		jsonErr(w, "db error", 500)
		return
	}
	jsonOK(w, map[string]bool{"ok": true})
}
