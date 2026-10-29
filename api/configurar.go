// Vercel Function: POST /api/configurar
package handler

import (
	"net/http"

	"github.com/angelivan44/pymetrados/httpapi"
)

func Configurar(w http.ResponseWriter, r *http.Request) {
	httpapi.Configurar(w, r)
}
