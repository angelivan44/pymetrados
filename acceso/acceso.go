// Package acceso verifica que las peticiones vengan de un usuario con sesión en
// electroestudio.com. El login es el de Neon Auth del portal: el navegador pide
// un JWT a <NEON_AUTH_URL>/token y lo envía como "Authorization: Bearer <jwt>";
// aquí se verifica la firma con las claves públicas (JWKS) de Neon Auth.
package acceso

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/MicahParks/keyfunc/v3"
	"github.com/golang-jwt/jwt/v5"
)

// Usuario es lo que se sabe de quien hace la petición.
type Usuario struct {
	ID     string
	Correo string
	Nombre string
}

type claims struct {
	jwt.RegisteredClaims
	Email string `json:"email"`
	Name  string `json:"name"`
}

var (
	// ErrSinSesion: no hay token o no es válido (firma, emisor o vencimiento).
	ErrSinSesion = errors.New("sin sesión")
	// ErrSinConfigurar: el servidor no tiene NEON_AUTH_URL.
	ErrSinConfigurar = errors.New("falta configurar NEON_AUTH_URL")
)

// Verificador comprueba los JWT de un Neon Auth concreto.
type Verificador struct {
	authURL string
	origen  string // emisor y audiencia esperados: el origen de authURL
	keys    jwt.Keyfunc
	parser  *jwt.Parser
}

// Nuevo crea un verificador para la URL de Neon Auth (p. ej.
// https://ep-xx.neonauth.../neondb/auth). Descarga las claves públicas.
func Nuevo(ctx context.Context, authURL string) (*Verificador, error) {
	u, err := url.Parse(strings.TrimRight(authURL, "/"))
	if err != nil || u.Scheme == "" || u.Host == "" {
		return nil, fmt.Errorf("NEON_AUTH_URL no es una URL válida: %q", authURL)
	}
	origen := u.Scheme + "://" + u.Host
	jwks, err := keyfunc.NewDefaultCtx(ctx, []string{u.String() + "/.well-known/jwks.json"})
	if err != nil {
		return nil, fmt.Errorf("no se pudieron obtener las claves de Neon Auth: %w", err)
	}
	return &Verificador{
		authURL: u.String(),
		origen:  origen,
		keys:    jwks.Keyfunc,
		parser: jwt.NewParser(
			jwt.WithValidMethods([]string{"EdDSA"}),
			jwt.WithIssuer(origen),
			jwt.WithAudience(origen),
			jwt.WithExpirationRequired(),
			jwt.WithLeeway(30*time.Second),
		),
	}, nil
}

// AuthURL es la URL pública de Neon Auth (la página la necesita para pedir el token).
func (v *Verificador) AuthURL() string { return v.authURL }

// Verifica valida el token "Bearer" de la petición.
func (v *Verificador) Verifica(r *http.Request) (*Usuario, error) {
	header := r.Header.Get("Authorization")
	token, ok := strings.CutPrefix(header, "Bearer ")
	if !ok || strings.TrimSpace(token) == "" {
		return nil, ErrSinSesion
	}
	c := &claims{}
	if _, err := v.parser.ParseWithClaims(strings.TrimSpace(token), c, v.keys); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrSinSesion, err)
	}
	if c.Subject == "" {
		return nil, fmt.Errorf("%w: el token no tiene usuario", ErrSinSesion)
	}
	return &Usuario{ID: c.Subject, Correo: c.Email, Nombre: c.Name}, nil
}

// Configuración leída de las variables de entorno:
//
//	NEON_AUTH_URL          URL de Neon Auth del portal (obligatoria en producción)
//	METRADOS_SIN_ACCESO=1  desactiva el login (solo para desarrollo local)
var (
	mu     sync.Mutex
	global *Verificador
)

// SinAcceso indica si el login está desactivado (desarrollo local).
func SinAcceso() bool { return os.Getenv("METRADOS_SIN_ACCESO") == "1" }

// DesdeEnv devuelve el verificador configurado, o ErrSinConfigurar. Se crea una
// sola vez por instancia de la función; si falla (p. ej. la red al bajar las
// claves) se reintenta en la siguiente petición.
func DesdeEnv() (*Verificador, error) {
	mu.Lock()
	defer mu.Unlock()
	if global != nil {
		return global, nil
	}
	authURL := strings.TrimSpace(os.Getenv("NEON_AUTH_URL"))
	if authURL == "" {
		return nil, ErrSinConfigurar
	}
	// El contexto controla el refresco periódico de las claves: debe durar
	// lo mismo que la instancia.
	v, err := Nuevo(context.Background(), authURL)
	if err != nil {
		return nil, err
	}
	global = v
	return v, nil
}
