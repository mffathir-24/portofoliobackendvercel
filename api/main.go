package handler

import (
	"net/http"

	"gintugas/vercelapp"
)

var app *vercelapp.GinApp

func init() {
	// Inisialisasi aplikasi Gin saat function load pertama kali
	app = vercelapp.InitializeApp()
}

// Handler untuk Vercel - WAJIB ADA dengan nama ini
func Handler(w http.ResponseWriter, r *http.Request) {
	// Serahkan request ke Gin router
	app.ServeHTTP(w, r)
}
