// Vercel Function: POST /api/validate
package handler

import (
	"net/http"

	"github.com/angelivan44/pymetrados/internal/httpapi"
)

func Validate(w http.ResponseWriter, r *http.Request) {
	httpapi.Validate(w, r)
}
