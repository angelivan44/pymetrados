// Servidor local equivalente al despliegue en Vercel: sirve public/ y las
// funciones de api/. Uso: go run ./cmd/dev  →  http://localhost:3000
//
// Sin NEON_AUTH_URL el login queda desactivado (METRADOS_SIN_ACCESO=1).
package main

import (
	"flag"
	"log"
	"net/http"
	"os"

	"github.com/angelivan44/pymetrados/httpapi"
)

func main() {
	addr := flag.String("addr", "localhost:3000", "dirección donde escuchar")
	flag.Parse()
	if os.Getenv("NEON_AUTH_URL") == "" {
		os.Setenv("METRADOS_SIN_ACCESO", "1")
		log.Print("sin NEON_AUTH_URL: login desactivado")
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/config", httpapi.Config)
	mux.HandleFunc("/api/validate", httpapi.Validate)
	mux.HandleFunc("/api/generate", httpapi.Generate)
	mux.Handle("/", http.FileServer(http.Dir("public")))
	log.Printf("PyMetrados en http://%s", *addr)
	log.Fatal(http.ListenAndServe(*addr, mux))
}
