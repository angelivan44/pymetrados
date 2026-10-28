package acceso

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// neonFalso simula Neon Auth: publica la clave en /neondb/auth/.well-known/jwks.json.
func neonFalso(t *testing.T) (authURL string, firma func(jwt.MapClaims) string, ajena func(jwt.MapClaims) string) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	_, otra, _ := ed25519.GenerateKey(rand.Reader)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/neondb/auth/.well-known/jwks.json" {
			http.NotFound(w, r)
			return
		}
		fmt.Fprintf(w, `{"keys":[{"alg":"EdDSA","crv":"Ed25519","kty":"OKP","kid":"k1","x":%q}]}`,
			base64.RawURLEncoding.EncodeToString(pub))
	}))
	t.Cleanup(srv.Close)
	sign := func(key ed25519.PrivateKey) func(jwt.MapClaims) string {
		return func(c jwt.MapClaims) string {
			tok := jwt.NewWithClaims(jwt.SigningMethodEdDSA, c)
			tok.Header["kid"] = "k1"
			s, err := tok.SignedString(key)
			if err != nil {
				t.Fatal(err)
			}
			return s
		}
	}
	return srv.URL + "/neondb/auth", sign(priv), sign(otra)
}

func peticion(token string) *http.Request {
	r := httptest.NewRequest(http.MethodPost, "/api/generate", nil)
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	return r
}

func TestVerifica(t *testing.T) {
	authURL, firma, ajena := neonFalso(t)
	v, err := Nuevo(context.Background(), authURL)
	if err != nil {
		t.Fatal(err)
	}
	origen := authURL[:len(authURL)-len("/neondb/auth")]
	valido := func() jwt.MapClaims {
		return jwt.MapClaims{
			"iss": origen, "aud": origen, "sub": "u-1", "email": "ana@ejemplo.com", "name": "Ana",
			"exp": time.Now().Add(15 * time.Minute).Unix(), "iat": time.Now().Unix(), "role": "authenticated",
		}
	}

	u, err := v.Verifica(peticion(firma(valido())))
	if err != nil {
		t.Fatalf("token válido rechazado: %v", err)
	}
	if *u != (Usuario{ID: "u-1", Correo: "ana@ejemplo.com", Nombre: "Ana"}) {
		t.Fatalf("usuario = %+v", u)
	}

	casos := map[string]string{
		"sin token":       "",
		"basura":          "no-es-un-jwt",
		"otra clave":      ajena(valido()),
		"emisor ajeno":    firma(func() jwt.MapClaims { c := valido(); c["iss"] = "https://otro.com"; return c }()),
		"audiencia ajena": firma(func() jwt.MapClaims { c := valido(); c["aud"] = "https://otro.com"; return c }()),
		"vencido":         firma(func() jwt.MapClaims { c := valido(); c["exp"] = time.Now().Add(-time.Hour).Unix(); return c }()),
		"sin vencimiento": firma(func() jwt.MapClaims { c := valido(); delete(c, "exp"); return c }()),
		"sin usuario":     firma(func() jwt.MapClaims { c := valido(); delete(c, "sub"); return c }()),
	}
	for nombre, token := range casos {
		if _, err := v.Verifica(peticion(token)); !errors.Is(err, ErrSinSesion) {
			t.Errorf("%s: err = %v, se esperaba ErrSinSesion", nombre, err)
		}
	}
}

func TestURLInvalida(t *testing.T) {
	if _, err := Nuevo(context.Background(), "no es url"); err == nil {
		t.Fatal("se esperaba error")
	}
}

func TestSinConfigurar(t *testing.T) {
	t.Setenv("NEON_AUTH_URL", "")
	if _, err := DesdeEnv(); !errors.Is(err, ErrSinConfigurar) {
		t.Fatalf("err = %v", err)
	}
}
