// Package httpapi expone la validación y la generación del metrado como
// handlers HTTP. Lo usan las Vercel Functions (api/) y el servidor local (cmd/dev).
package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"runtime/debug"

	"github.com/angelivan44/pymetrados/acceso"
	"github.com/angelivan44/pymetrados/metrado"
)

// Vercel rechaza peticiones de más de 4,5 MB; se deja margen para el multipart.
const MaxUploadBytes = 4*1024*1024 + 300*1024

func init() {
	// excelize genera mucha basura al reescribir fórmulas; con menos ciclos de
	// GC la generación tarda bastante menos y la memoria sigue holgada.
	debug.SetGCPercent(400)
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func singleError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, metrado.Report{
		Errors:   []metrado.Issue{{Level: "error", Message: message}},
		Warnings: []metrado.Issue{},
	})
}

// verificador se reemplaza en las pruebas.
var verificador = acceso.DesdeEnv

// autoriza exige una sesión de electroestudio.com (salvo con METRADOS_SIN_ACCESO=1
// en desarrollo). Si devuelve false ya respondió al cliente.
func autoriza(w http.ResponseWriter, r *http.Request) (*acceso.Usuario, bool) {
	if acceso.SinAcceso() {
		return &acceso.Usuario{ID: "local"}, true
	}
	v, err := verificador()
	if errors.Is(err, acceso.ErrSinConfigurar) {
		log.Print("NEON_AUTH_URL no está configurada")
		singleError(w, http.StatusInternalServerError, "El servidor no tiene configurado el acceso. Avísanos para revisarlo.")
		return nil, false
	}
	if err != nil {
		log.Printf("acceso: %v", err)
		singleError(w, http.StatusServiceUnavailable, "No se pudo contactar el servicio de acceso. Intenta de nuevo en un momento.")
		return nil, false
	}
	u, err := v.Verifica(r)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]any{
			"ok": false, "sesion": false, "warnings": []metrado.Issue{}, "summary": nil,
			"errors": []metrado.Issue{{Level: "error", Message: "Tu sesión venció o no es válida. Vuelve a iniciar sesión en electroestudio.com."}},
		})
		return nil, false
	}
	return u, true
}

// load lee los dos archivos del formulario y los analiza. Si devuelve
// ok=false ya respondió al cliente.
func load(w http.ResponseWriter, r *http.Request) (metrado.Report, *metrado.Inputs, bool) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		singleError(w, http.StatusMethodNotAllowed, "Usa POST con los archivos bd y planilla.")
		return metrado.Report{}, nil, false
	}
	usuario, ok := autoriza(w, r)
	if !ok {
		return metrado.Report{}, nil, false
	}
	r.Body = http.MaxBytesReader(w, r.Body, MaxUploadBytes)
	if err := r.ParseMultipartForm(MaxUploadBytes); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			singleError(w, http.StatusRequestEntityTooLarge,
				"Los archivos superan los 4 MB permitidos. Quita hojas o filas que no se usen y vuelve a intentarlo.")
		} else {
			singleError(w, http.StatusBadRequest, "No se pudieron leer los archivos enviados.")
		}
		return metrado.Report{}, nil, false
	}
	files := map[string][]byte{}
	for _, field := range []struct{ name, label string }{{"bd", "BD"}, {"planilla", "Planilla"}} {
		file, _, err := r.FormFile(field.name)
		if err != nil {
			singleError(w, http.StatusBadRequest, fmt.Sprintf("Falta el archivo %s.", field.label))
			return metrado.Report{}, nil, false
		}
		data, err := io.ReadAll(file)
		file.Close()
		if err != nil || len(data) == 0 {
			singleError(w, http.StatusBadRequest, fmt.Sprintf("El archivo %s está vacío.", field.label))
			return metrado.Report{}, nil, false
		}
		files[field.name] = data
	}
	report, inputs := metrado.Analyze(files["bd"], files["planilla"])
	if !report.OK {
		var messages []string
		for _, e := range report.Errors {
			messages = append(messages, e.Message)
		}
		log.Printf("validación con errores (%s): %q", usuario.Correo, messages)
	}
	return report, inputs, true
}

// Config le dice a la página dónde pedir el token de sesión (la URL de Neon
// Auth es pública) o que el login está desactivado.
func Config(w http.ResponseWriter, r *http.Request) {
	if acceso.SinAcceso() {
		writeJSON(w, http.StatusOK, map[string]any{"sinAcceso": true, "authUrl": nil})
		return
	}
	v, err := verificador()
	if err != nil {
		log.Printf("acceso: %v", err)
		singleError(w, http.StatusServiceUnavailable, "No se pudo preparar el acceso. Intenta de nuevo en un momento.")
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, map[string]any{"sinAcceso": false, "authUrl": v.AuthURL()})
}

// Validate responde con el reporte de validación (errores, advertencias y resumen).
func Validate(w http.ResponseWriter, r *http.Request) {
	report, _, ok := load(w, r)
	if ok {
		writeJSON(w, http.StatusOK, report)
	}
}

// Generate devuelve Metrado.xlsx, o el reporte con status 422 si hay errores.
func Generate(w http.ResponseWriter, r *http.Request) {
	report, inputs, ok := load(w, r)
	if !ok {
		return
	}
	if inputs == nil || !report.OK {
		writeJSON(w, http.StatusUnprocessableEntity, report)
		return
	}
	content, err := metrado.Generate(inputs)
	if err != nil {
		log.Printf("fallo al generar el metrado: %v", err)
		singleError(w, http.StatusInternalServerError,
			"Ocurrió un error inesperado al generar el metrado. Los archivos pasaron la validación, "+
				"así que es un problema de la aplicación: envíanos ambos archivos para revisarlo.")
		return
	}
	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", `attachment; filename="Metrado.xlsx"`)
	_, _ = w.Write(content)
}
