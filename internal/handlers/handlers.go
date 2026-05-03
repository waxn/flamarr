package handlers

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"embed"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"html/template"
	"io"
	"log"
	"net/http"
	neturl "net/url"
	"regexp"
	"strconv"
	"strings"
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

type simpleIcon struct {
	Title string
	Slug  string
}

var (
	simpleIconsOnce sync.Once
	simpleIcons     []simpleIcon
	simpleIconsErr  error
	iconCache       sync.Map // cacheKey -> resolved icon URL or empty sentinel
)

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

func normalizeURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil
	}
	if strings.Contains(raw, "://") {
		if _, err := neturl.ParseRequestURI(raw); err != nil {
			return "", err
		}
		return raw, nil
	}
	normalized := "https://" + raw
	if _, err := neturl.ParseRequestURI(normalized); err != nil {
		return "", err
	}
	return normalized, nil
}

func fetchFaviconDataURL(rawURL string) (string, error) {
	parsed, err := neturl.Parse(rawURL)
	if err != nil {
		return "", err
	}

	client := &http.Client{Timeout: 5 * time.Second}
	schemes := []string{parsed.Scheme}
	if parsed.Scheme == "https" {
		schemes = append(schemes, "http")
	} else if parsed.Scheme == "http" {
		schemes = append(schemes, "https")
	}

	iconHrefRe := regexp.MustCompile(`(?i)<link[^>]+rel=["'][^"']*icon[^"']*["'][^>]*href=["']([^"']+)["']|<link[^>]+href=["']([^"']+)["'][^>]*rel=["'][^"']*icon[^"']*["']`)
	toDataURL := func(resp *http.Response) (string, error) {
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		if readErr != nil || len(body) == 0 {
			return "", readErr
		}
		contentType := strings.TrimSpace(strings.Split(resp.Header.Get("Content-Type"), ";")[0])
		if contentType == "" {
			contentType = "image/x-icon"
		}
		if !strings.HasPrefix(contentType, "image/") {
			contentType = "image/x-icon"
		}
		return "data:" + contentType + ";base64," + base64.StdEncoding.EncodeToString(body), nil
	}

	tryFetch := func(iconURL string) (string, error) {
		resp, err := client.Get(iconURL)
		if err != nil {
			return "", err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return "", nil
		}
		return toDataURL(resp)
	}

	for _, scheme := range schemes {
		candidate := *parsed
		candidate.Scheme = scheme
		candidate.RawQuery = ""
		candidate.Fragment = ""

		pageResp, err := client.Get(candidate.String())
		if err == nil && pageResp.StatusCode == http.StatusOK {
			pageBody, readErr := io.ReadAll(io.LimitReader(pageResp.Body, 1<<20))
			pageResp.Body.Close()
			if readErr == nil && len(pageBody) > 0 {
				matches := iconHrefRe.FindAllStringSubmatch(string(pageBody), -1)
				for _, match := range matches {
					href := match[1]
					if href == "" {
						href = match[2]
					}
					if href == "" {
						continue
					}
					resolved, resolveErr := neturl.Parse(href)
					if resolveErr != nil {
						continue
					}
					iconURL := candidate.ResolveReference(resolved).String()
					if dataURL, fetchErr := tryFetch(iconURL); fetchErr == nil && dataURL != "" {
						return dataURL, nil
					}
				}
			}
		}

		faviconCandidate := candidate
		faviconCandidate.Path = "/favicon.ico"
		faviconCandidate.RawQuery = ""
		faviconCandidate.Fragment = ""
		faviconURL := faviconCandidate.String()
		if dataURL, fetchErr := tryFetch(faviconURL); fetchErr == nil && dataURL != "" {
			return dataURL, nil
		}
	}

	return "", nil
}

func normalizeIconKey(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" {
		return ""
	}
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func loadSimpleIcons() ([]simpleIcon, error) {
	simpleIconsOnce.Do(func() {
		client := &http.Client{Timeout: 10 * time.Second}
		resp, err := client.Get("https://raw.githubusercontent.com/simple-icons/simple-icons/main/slugs.md")
		if err != nil {
			simpleIconsErr = err
			return
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			simpleIconsErr = io.ErrUnexpectedEOF
			return
		}
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			simpleIconsErr = err
			return
		}
		re := regexp.MustCompile(`^\|\s*` + "`" + `([^` + "`" + `]+)` + "`" + `\s*\|\s*` + "`" + `([^` + "`" + `]+)` + "`" + `\s*\|$`)
		lines := strings.Split(string(body), "\n")
		icons := make([]simpleIcon, 0, len(lines))
		for _, line := range lines {
			match := re.FindStringSubmatch(line)
			if len(match) != 3 {
				continue
			}
			icons = append(icons, simpleIcon{Title: match[1], Slug: match[2]})
		}
		simpleIcons = icons
	})
	return simpleIcons, simpleIconsErr
}

func chooseSimpleIcon(name, rawURL string) string {
	icons, err := loadSimpleIcons()
	if err != nil || len(icons) == 0 {
		return ""
	}

	queries := []string{normalizeIconKey(name)}
	if parsed, parseErr := neturl.Parse(rawURL); parseErr == nil {
		host := strings.TrimPrefix(parsed.Hostname(), "www.")
		queries = append(queries, normalizeIconKey(host))
		queries = append(queries, normalizeIconKey(strings.TrimSuffix(host, ".local")))
	}

	bestSlug := ""
	bestScore := 0
	for _, icon := range icons {
		titleKey := normalizeIconKey(icon.Title)
		slugKey := normalizeIconKey(icon.Slug)
		for _, query := range queries {
			if query == "" {
				continue
			}
			score := 0
			switch {
			case query == titleKey || query == slugKey:
				score = 4
			case strings.Contains(titleKey, query) || strings.Contains(slugKey, query) || strings.Contains(query, titleKey) || strings.Contains(query, slugKey):
				score = 3
			case strings.HasPrefix(titleKey, query) || strings.HasPrefix(slugKey, query) || strings.HasPrefix(query, titleKey) || strings.HasPrefix(query, slugKey):
				score = 2
			case len(query) >= 3 && strings.Contains(titleKey, query[:3]):
				score = 1
			}
			if score > bestScore {
				bestScore = score
				bestSlug = icon.Slug
			}
		}
	}

	if bestSlug == "" {
		return ""
	}
	return "https://simpleicons.org/icons/" + bestSlug + ".svg"
}

func resolveItemIcon(name, rawURL, customIcon string) string {
	if customIcon != "" {
		return customIcon
	}
	cacheKey := normalizeIconKey(name) + "|" + strings.TrimSpace(rawURL)
	if cached, ok := iconCache.Load(cacheKey); ok {
		if icon, _ := cached.(string); icon != "" {
			return icon
		}
		return ""
	}
	if icon, _ := fetchFaviconDataURL(rawURL); icon != "" {
		iconCache.Store(cacheKey, icon)
		return icon
	}
	if icon := chooseSimpleIcon(name, rawURL); icon != "" {
		iconCache.Store(cacheKey, icon)
		return icon
	}
	iconCache.Store(cacheKey, "")
	return ""
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
	Today     string
}

func (h *Handler) Dashboard(w http.ResponseWriter, r *http.Request) {
	items, err := h.db.GetItems()
	if err != nil {
		http.Error(w, "db error", 500)
		return
	}
	data := dashboardData{}
	data.Today = time.Now().Format("Monday, 2 January 2006")
	for _, it := range items {
		if it.Icon == "" {
			it.Icon = resolveItemIcon(it.Name, it.URL, "")
		}
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
	normalizedURL, err := normalizeURL(body.URL)
	if err != nil || normalizedURL == "" {
		jsonErr(w, "invalid url", 400)
		return
	}
	body.URL = normalizedURL
	if body.Name == "" || body.URL == "" {
		jsonErr(w, "name and url required", 400)
		return
	}
	if body.Type != "service" && body.Type != "bookmark" {
		body.Type = "service"
	}
	body.Icon = resolveItemIcon(body.Name, body.URL, body.Icon)
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
	normalizedURL, err := normalizeURL(body.URL)
	if err != nil || normalizedURL == "" {
		jsonErr(w, "invalid url", 400)
		return
	}
	body.URL = normalizedURL
	if body.Type != "service" && body.Type != "bookmark" {
		body.Type = "service"
	}
	body.Icon = resolveItemIcon(body.Name, body.URL, body.Icon)
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
