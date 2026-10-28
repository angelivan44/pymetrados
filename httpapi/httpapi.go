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
	"strconv"
	"strings"
	"time"

	"github.com/angelivan44/pymetrados/acceso"
	"github.com/angelivan44/pymetrados/metrado"
)

// Vercel rechaza peticiones de más de 4,5 MB; se deja margen para el multipart.
const MaxUploadBytes = 4*1024*1024 + 300*1024

func init() {
	// excelize genera mucha basura al reescribir fórmulas. La función tiene 1
	// vCPU y 2 GB: con GOGC=1000 la generación baja ~20% (pico medido ~0,9 GB) y
	// el tope de memoria obliga a recolectar antes de acercarse al límite.
	debug.SetGCPercent(1000)
	debug.SetMemoryLimit(1536 << 20)
}

// tiempos registra cuánto tarda cada etapa de una petición (se ve en los
// logs de Vercel y en la cabecera Server-Timing).
type tiempos struct {
	inicio time.Time
	etapas []string
	ultimo time.Time
}

func nuevosTiempos() *tiempos {
	ahora := time.Now()
	return &tiempos{inicio: ahora, ultimo: ahora}
}

func (t *tiempos) marca(etapa string) {
	ahora := time.Now()
	t.etapas = append(t.etapas, fmt.Sprintf("%s;dur=%d", etapa, ahora.Sub(t.ultimo).Milliseconds()))
	t.ultimo = ahora
}

func (t *tiempos) cabecera() string {
	return strings.Join(append(t.etapas, fmt.Sprintf("total;dur=%d", time.Since(t.inicio).Milliseconds())), ", ")
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

// CabeceraTamanoBD indica, en el envío binario, cuántos bytes del cuerpo son la BD
// (el resto es la planilla).
const CabeceraTamanoBD = "X-Tamano-Bd"

type archivos struct{ bd, planilla []byte }

const mensajeMuyGrande = "Los archivos superan los 4 MB permitidos. Quita hojas o filas que no se usen y vuelve a intentarlo."

// leeArchivos acepta dos formatos:
//   - binario (application/octet-stream): la BD seguida de la planilla, con el tamaño
//     de la BD en X-Tamano-Bd. Es el que usa la página: en Vercel un cuerpo multipart
//     tarda ~3 veces más en llegar a la función (medido: 2,5 s contra 0,75 s).
//   - multipart/form-data con los campos bd y planilla (curl, pruebas).
//
// Si falla devuelve el mensaje para el usuario y el status HTTP.
func leeArchivos(r *http.Request) (archivos, string, int) {
	esMuyGrande := func(err error) bool {
		var tooLarge *http.MaxBytesError
		return errors.As(err, &tooLarge)
	}
	if strings.HasPrefix(r.Header.Get("Content-Type"), "application/octet-stream") {
		n, err := strconv.Atoi(r.Header.Get(CabeceraTamanoBD))
		if err != nil || n <= 0 {
			return archivos{}, "Falta el tamaño del archivo BD en el envío.", http.StatusBadRequest
		}
		body, err := io.ReadAll(r.Body)
		if esMuyGrande(err) {
			return archivos{}, mensajeMuyGrande, http.StatusRequestEntityTooLarge
		}
		if err != nil {
			return archivos{}, "No se pudieron leer los archivos enviados.", http.StatusBadRequest
		}
		if n >= len(body) {
			return archivos{}, "El archivo Planilla está vacío.", http.StatusBadRequest
		}
		return archivos{bd: body[:n], planilla: body[n:]}, "", 0
	}

	if err := r.ParseMultipartForm(MaxUploadBytes); err != nil {
		if esMuyGrande(err) {
			return archivos{}, mensajeMuyGrande, http.StatusRequestEntityTooLarge
		}
		return archivos{}, "No se pudieron leer los archivos enviados.", http.StatusBadRequest
	}
	var out archivos
	for _, field := range []struct {
		name, label string
		dest        *[]byte
	}{{"bd", "BD", &out.bd}, {"planilla", "Planilla", &out.planilla}} {
		file, _, err := r.FormFile(field.name)
		if err != nil {
			return archivos{}, fmt.Sprintf("Falta el archivo %s.", field.label), http.StatusBadRequest
		}
		data, err := io.ReadAll(file)
		file.Close()
		if err != nil || len(data) == 0 {
			return archivos{}, fmt.Sprintf("El archivo %s está vacío.", field.label), http.StatusBadRequest
		}
		*field.dest = data
	}
	return out, "", 0
}

// load lee los dos archivos del formulario y los analiza. Si devuelve
// ok=false ya respondió al cliente.
func load(w http.ResponseWriter, r *http.Request, t *tiempos) (metrado.Report, *metrado.Inputs, *acceso.Usuario, bool) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		singleError(w, http.StatusMethodNotAllowed, "Usa POST con los archivos bd y planilla.")
		return metrado.Report{}, nil, nil, false
	}
	usuario, ok := autoriza(w, r)
	if !ok {
		return metrado.Report{}, nil, nil, false
	}
	t.marca("acceso")
	r.Body = http.MaxBytesReader(w, r.Body, MaxUploadBytes)
	files, mensaje, status := leeArchivos(r)
	if mensaje != "" {
		singleError(w, status, mensaje)
		return metrado.Report{}, nil, nil, false
	}
	t.marca("lectura")
	report, inputs := metrado.Analyze(files.bd, files.planilla)
	t.marca("validacion")
	if !report.OK {
		var messages []string
		for _, e := range report.Errors {
			messages = append(messages, e.Message)
		}
		log.Printf("validación con errores (%s): %q", usuario.Correo, messages)
	}
	return report, inputs, usuario, true
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

// registra deja en el log de Vercel los tiempos de la petición.
func registra(w http.ResponseWriter, accion string, u *acceso.Usuario, t *tiempos, extra string) {
	w.Header().Set("Server-Timing", t.cabecera())
	correo := ""
	if u != nil {
		correo = u.Correo
	}
	log.Printf("%s usuario=%s %s %s", accion, correo, strings.ReplaceAll(t.cabecera(), ";dur=", "="), extra)
}

// Validate responde con el reporte de validación (errores, advertencias y resumen).
func Validate(w http.ResponseWriter, r *http.Request) {
	t := nuevosTiempos()
	report, _, usuario, ok := load(w, r, t)
	if ok {
		registra(w, "validate", usuario, t, fmt.Sprintf("ok=%v", report.OK))
		writeJSON(w, http.StatusOK, report)
	}
}

// Generate devuelve Metrado.xlsx, o el reporte con status 422 si hay errores.
func Generate(w http.ResponseWriter, r *http.Request) {
	t := nuevosTiempos()
	report, inputs, usuario, ok := load(w, r, t)
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
	t.marca("generacion")
	registra(w, "generate", usuario, t, fmt.Sprintf("filas=%d bytes=%d", report.Summary.Filas, len(content)))
	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", `attachment; filename="Metrado.xlsx"`)
	_, _ = w.Write(content)
}
