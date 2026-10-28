package httpapi

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/angelivan44/pymetrados/acceso"
	"github.com/golang-jwt/jwt/v5"
)

// sinLogin desactiva el acceso para probar el resto de la API.
func sinLogin(t *testing.T) { t.Setenv("METRADOS_SIN_ACCESO", "1") }

// conNeonFalso configura el verificador contra un Neon Auth simulado y
// devuelve un token válido.
func conNeonFalso(t *testing.T) string {
	t.Helper()
	t.Setenv("METRADOS_SIN_ACCESO", "")
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `{"keys":[{"alg":"EdDSA","crv":"Ed25519","kty":"OKP","kid":"k1","x":%q}]}`,
			base64.RawURLEncoding.EncodeToString(pub))
	}))
	t.Cleanup(srv.Close)
	v, err := acceso.Nuevo(context.Background(), srv.URL+"/neondb/auth")
	if err != nil {
		t.Fatal(err)
	}
	anterior := verificador
	verificador = func() (*acceso.Verificador, error) { return v, nil }
	t.Cleanup(func() { verificador = anterior })
	tok := jwt.NewWithClaims(jwt.SigningMethodEdDSA, jwt.MapClaims{
		"iss": srv.URL, "aud": srv.URL, "sub": "u-1", "email": "ana@ejemplo.com",
		"exp": time.Now().Add(time.Minute).Unix(),
	})
	tok.Header["kid"] = "k1"
	s, err := tok.SignedString(priv)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func multipartBody(t *testing.T, files map[string][]byte) (*bytes.Buffer, string) {
	t.Helper()
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	for field, content := range files {
		part, err := w.CreateFormFile(field, field+".xlsx")
		if err != nil {
			t.Fatal(err)
		}
		part.Write(content)
	}
	w.Close()
	return &body, w.FormDataContentType()
}

func call(t *testing.T, handler http.HandlerFunc, method string, files map[string][]byte, token ...string) (int, map[string]any) {
	t.Helper()
	body, contentType := multipartBody(t, files)
	req := httptest.NewRequest(method, "/api/validate", body)
	req.Header.Set("Content-Type", contentType)
	if len(token) > 0 {
		req.Header.Set("Authorization", "Bearer "+token[0])
	}
	rec := httptest.NewRecorder()
	handler(rec, req)
	var out map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	return rec.Code, out
}

func firstError(out map[string]any) string {
	errs, _ := out["errors"].([]any)
	if len(errs) == 0 {
		return ""
	}
	return errs[0].(map[string]any)["message"].(string)
}

func TestSoloPOST(t *testing.T) {
	sinLogin(t)
	code, out := call(t, Validate, http.MethodGet, nil)
	if code != http.StatusMethodNotAllowed || !strings.Contains(firstError(out), "POST") {
		t.Fatalf("%d %v", code, out)
	}
}

func TestFaltaArchivo(t *testing.T) {
	sinLogin(t)
	code, out := call(t, Generate, http.MethodPost, map[string][]byte{"bd": []byte("x")})
	if code != http.StatusBadRequest || firstError(out) != "Falta el archivo Planilla." {
		t.Fatalf("%d %v", code, out)
	}
}

func TestArchivosDemasiadoGrandes(t *testing.T) {
	sinLogin(t)
	big := bytes.Repeat([]byte("x"), MaxUploadBytes)
	code, out := call(t, Validate, http.MethodPost, map[string][]byte{"bd": big, "planilla": []byte("x")})
	if code != http.StatusRequestEntityTooLarge || !strings.Contains(firstError(out), "4 MB") {
		t.Fatalf("%d %v", code, out)
	}
}

func TestArchivoInvalidoDevuelveReporte(t *testing.T) {
	sinLogin(t)
	code, out := call(t, Generate, http.MethodPost, map[string][]byte{"bd": []byte("no es excel"), "planilla": []byte("x")})
	if code != http.StatusUnprocessableEntity || !strings.Contains(firstError(out), "No se pudo abrir el archivo BD") {
		t.Fatalf("%d %v", code, out)
	}
}

func TestSinTokenDevuelve401(t *testing.T) {
	conNeonFalso(t)
	code, out := call(t, Generate, http.MethodPost, map[string][]byte{"bd": []byte("x"), "planilla": []byte("x")})
	if code != http.StatusUnauthorized || out["sesion"] != false || !strings.Contains(firstError(out), "sesión") {
		t.Fatalf("%d %v", code, out)
	}
	code, _ = call(t, Validate, http.MethodPost, nil, "token-falso")
	if code != http.StatusUnauthorized {
		t.Fatalf("token falso: %d", code)
	}
}

func TestConTokenValidoPasaElAcceso(t *testing.T) {
	token := conNeonFalso(t)
	code, out := call(t, Generate, http.MethodPost, map[string][]byte{"bd": []byte("x")}, token)
	if code != http.StatusBadRequest || firstError(out) != "Falta el archivo Planilla." {
		t.Fatalf("%d %v", code, out)
	}
}

func TestSinConfigurarNoDejaPasar(t *testing.T) {
	t.Setenv("METRADOS_SIN_ACCESO", "")
	anterior := verificador
	verificador = func() (*acceso.Verificador, error) { return nil, acceso.ErrSinConfigurar }
	t.Cleanup(func() { verificador = anterior })
	code, _ := call(t, Validate, http.MethodPost, map[string][]byte{"bd": []byte("x"), "planilla": []byte("x")})
	if code != http.StatusInternalServerError {
		t.Fatalf("code = %d", code)
	}
}

func TestConfigDaLaURLDeAcceso(t *testing.T) {
	conNeonFalso(t)
	rec := httptest.NewRecorder()
	Config(rec, httptest.NewRequest(http.MethodGet, "/api/config", nil))
	var out map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	if out["sinAcceso"] != false || !strings.HasSuffix(fmt.Sprint(out["authUrl"]), "/neondb/auth") {
		t.Fatalf("%v", out)
	}
}

func callBinario(t *testing.T, handler http.HandlerFunc, bd, planilla []byte, tamano string) (int, map[string]any) {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/generate", bytes.NewReader(append(append([]byte{}, bd...), planilla...)))
	req.Header.Set("Content-Type", "application/octet-stream")
	if tamano != "" {
		req.Header.Set(CabeceraTamanoBD, tamano)
	}
	rec := httptest.NewRecorder()
	handler(rec, req)
	var out map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	return rec.Code, out
}

func TestEnvioBinario(t *testing.T) {
	sinLogin(t)
	bd := []byte("no es excel")
	code, out := callBinario(t, Generate, bd, []byte("x"), fmt.Sprint(len(bd)))
	if code != http.StatusUnprocessableEntity || !strings.Contains(firstError(out), "No se pudo abrir el archivo BD") {
		t.Fatalf("separación BD/planilla: %d %v", code, out)
	}
	if code, out := callBinario(t, Validate, bd, []byte("x"), ""); code != http.StatusBadRequest || !strings.Contains(firstError(out), "tamaño") {
		t.Fatalf("sin cabecera: %d %v", code, out)
	}
	if code, out := callBinario(t, Validate, bd, nil, fmt.Sprint(len(bd))); code != http.StatusBadRequest || firstError(out) != "El archivo Planilla está vacío." {
		t.Fatalf("sin planilla: %d %v", code, out)
	}
	big := bytes.Repeat([]byte("x"), MaxUploadBytes)
	if code, _ := callBinario(t, Validate, big, []byte("x"), fmt.Sprint(len(big))); code != http.StatusRequestEntityTooLarge {
		t.Fatalf("muy grande: %d", code)
	}
}
