package main

import (
	"embed"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"

	"flamarr/internal/db"
	"flamarr/internal/handlers"
)

//go:embed web
var webFS embed.FS

func main() {
	dataDir := os.Getenv("DATA_DIR")
	if dataDir == "" {
		dataDir = "data"
	}
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		log.Fatal(err)
	}

	database, err := db.Init(dataDir + "/flamarr.db")
	if err != nil {
		log.Fatalf("db init: %v", err)
	}
	defer database.Close()

	staticFS, err := fs.Sub(webFS, "web/static")
	if err != nil {
		log.Fatal(err)
	}

	h := handlers.New(database, webFS)

	mux := http.NewServeMux()
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticFS))))

	mux.HandleFunc("GET /setup", h.Setup)
	mux.HandleFunc("POST /setup", h.HandleSetup)
	mux.HandleFunc("GET /login", h.Login)
	mux.HandleFunc("POST /login", h.HandleLogin)
	mux.HandleFunc("POST /logout", h.HandleLogout)

	// Forgot password / username (public)
	mux.HandleFunc("GET /forgot-password", h.ForgotPassword)
	mux.HandleFunc("POST /forgot-password", h.HandleForgotPassword)
	mux.HandleFunc("GET /forgot-username", h.ForgotUsername)
	mux.HandleFunc("POST /forgot-username", h.HandleForgotUsername)

	mux.Handle("GET /{$}", h.AuthMiddleware(http.HandlerFunc(h.Dashboard)))
	mux.Handle("GET /api/settings", h.AuthMiddleware(http.HandlerFunc(h.GetSettings)))
	mux.Handle("PUT /api/settings", h.AuthMiddleware(http.HandlerFunc(h.PutSettings)))
	mux.Handle("GET /api/items", h.AuthMiddleware(http.HandlerFunc(h.GetItems)))
	mux.Handle("POST /api/items", h.AuthMiddleware(http.HandlerFunc(h.CreateItem)))
	mux.Handle("PUT /api/items/{id}", h.AuthMiddleware(http.HandlerFunc(h.UpdateItem)))
	mux.Handle("DELETE /api/items/{id}", h.AuthMiddleware(http.HandlerFunc(h.DeleteItem)))

	port := os.Getenv("PORT")
	if port == "" {
		port = "5005"
	}
	fmt.Printf("flamarr listening on http://localhost:%s\n", port)
	log.Fatal(http.ListenAndServe(":"+port, mux))
}
