// Vercel Function: POST /api/generate
package handler

import (
	"net/http"

	"github.com/angelivan44/pymetrados/internal/httpapi"
)

func Generate(w http.ResponseWriter, r *http.Request) {
	httpapi.Generate(w, r)
}
