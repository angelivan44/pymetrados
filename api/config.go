// Vercel Function: GET /api/config
package handler

import (
	"net/http"

	"github.com/angelivan44/pymetrados/internal/httpapi"
)

func Config(w http.ResponseWriter, r *http.Request) {
	httpapi.Config(w, r)
}
