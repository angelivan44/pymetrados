# PyMetrados

Aplicación web que genera `Metrado.xlsx` a partir de una **BD** y una **planilla** en Excel.
No necesita Windows ni Excel instalado: los archivos se validan y procesan en una función
serverless en Go ([excelize](https://github.com/xuri/excelize)) desplegada en Vercel.

Es una de las herramientas de **electroestudio.com** y usa su mismo login: se publica en
`electroestudio.com/metrados/` (el portal reenvía esa ruta a este proyecto), así que comparte la
sesión de Neon Auth del portal.

## Acceso

```
navegador ──► electroestudio.com/metrados/  ──(rewrite del portal)──► este proyecto en Vercel
   │                                                                     │
   └─ GET <NEON_AUTH_URL>/token (cookie de sesión) ─► JWT                │
      POST api/generate  Authorization: Bearer <JWT> ───────────────────►┘ verifica firma (JWKS),
                                                                           emisor y vencimiento
```

| Variable | Dónde | Para qué |
|---|---|---|
| `NEON_AUTH_URL` | Vercel (obligatoria) | URL de Neon Auth del portal, la misma que `VITE_NEON_AUTH_URL` de `dltcad-web` |
| `METRADOS_SIN_ACCESO=1` | solo desarrollo | desactiva el login; `go run ./cmd/dev` la activa sola si no hay `NEON_AUTH_URL` |

Sin `NEON_AUTH_URL` (y sin `METRADOS_SIN_ACCESO`) la API rechaza todo: nunca queda abierta por error.

## Uso

1. Sube la **BD** (hojas `General`, `Configuracion`, `P_0`, `M_0`).
2. Sube la **Planilla** (hojas `DB`, `PLANILLA`).
3. La página valida ambos archivos al instante y muestra cada problema con su ubicación
   (por ejemplo `BD › Configuracion!B6`) y cómo corregirlo.
4. Si no hay errores, pulsa **Generar Metrado.xlsx**.

Entre los dos archivos pueden pesar hasta 4 MB (límite de Vercel por petición).

## Formato de entrada

**BD**

| Hoja | Contenido |
|---|---|
| `General` | Desde la fila 2: A = ID, B = Nombre, C = Código, D = Sistema, E = Ubicación |
| `Configuracion` | B2–B5: celdas de nombre, código, sistema y ubicación en `P_0`. B6: primera celda de datos en `P_0` (ej. `A15`). B7: celda de cantidad de elementos. B8: primera fila del metrado en `M_0`. B9: celda total de cada `M_` que va al Resumen. B10: última celda de encabezados de `PLANILLA` (ej. `AG1`) |
| `P_0` | Plantilla de planilla por localidad |
| `M_0` | Plantilla de metrado por localidad (columna I con las fórmulas por elemento) |
| Otras (`DATOS`, `BD_LP`…) | Tablas que usan las plantillas; se copian tal cual al Metrado |

**Planilla**

| Hoja | Contenido |
|---|---|
| `DB` | Columna A desde la fila 2: ID de la localidad de cada fila de `PLANILLA` |
| `PLANILLA` | Fila 1: encabezados. Desde la fila 2: un elemento por fila |

La fila *n* de `DB` corresponde a la fila *n* de `PLANILLA`. Las filas no necesitan estar
ordenadas por ID: se agrupan automáticamente. Las filas sin ID al final (relleno con
fórmulas) se ignoran.

## Desarrollo

Requiere Go 1.24+.

```bash
go run ./cmd/dev          # http://localhost:3000, sin login
go test ./...
```

Con el login real, junto al portal (`dltcad-web`, que en desarrollo reenvía `/metrados/` al puerto 3000):

```bash
NEON_AUTH_URL=<url de Neon Auth> go run ./cmd/dev
# en dltcad-web/apps/web: npx vite  → http://localhost:5173, iniciar sesión y abrir PyMetrados
```

Prueba con los ejemplos de la release v1.0 (`DM_LP.xlsx`, `Planilla LP.xlsx`):

```bash
METRADO_REAL_DIR=/ruta/a/los/ejemplos go test ./metrado -run ArchivosReales -v
```

Deja el resultado en `Metrado_go.xlsx` dentro de esa carpeta.

## Despliegue en Vercel

```bash
npm i -g vercel
vercel                                   # primera vez: vincula el proyecto
vercel env add NEON_AUTH_URL production  # la URL de Neon Auth del portal
vercel --prod
```

Vercel sirve `public/` como sitio estático y cada archivo de `api/` como una función Go
(`/api/config`, `/api/validate` y `/api/generate`). Después, en el `vercel.json` del portal
(`dltcad-web/apps/web`), la ruta `/metrados/` debe apuntar al dominio de este proyecto. El plan
gratuito (Hobby) es solo para uso no comercial.

## Estructura

```
public/index.html       página única
api/                    Vercel Functions (config.go, validate.go, generate.go)
httpapi/                handlers HTTP compartidos por Vercel y el servidor local
acceso/                 verificación del JWT de Neon Auth (sesión de electroestudio.com)
metrado/
  inputs.go             lectura y validación de BD y planilla (mensajes para el usuario)
  generator.go          construcción de Metrado.xlsx (equivalente a la versión COM)
  formula.go            traslado de referencias para el AutoFill
  xlsxparts.go          ajustes finales al .xlsx para que Excel lo abra sin reparar
cmd/dev/                servidor local
legacy/                 aplicación de escritorio anterior (Tkinter + Excel COM), solo referencia
```

## Diferencias con la versión de escritorio

Verificado con los ejemplos de la release v1.0: tras recalcular en Excel, las 188.355 celdas
coinciden con el `Metrado.xlsx` de referencia, salvo las dos columnas que antes se perdían
(ver abajo).

- No depende de Windows, de Excel ni de su idioma.
- El Metrado es autónomo: las hojas auxiliares de la BD (`DATOS`, `BD_LP`…) y sus nombres
  definidos se copian al resultado. Antes quedaban como vínculos externos al archivo BD en el
  disco de quien lo generó.
- Las filas de la planilla se agrupan por ID aunque estén desordenadas (antes se mezclaban
  datos entre localidades sin avisar).
- Se copian todas las columnas hasta `Configuracion!B10`, incluidas las que no tienen
  encabezado. Antes se copiaban tantas columnas como encabezados únicos, así que se perdían
  las últimas (en el ejemplo, "Abrazadera…" y "Comentarios").
- Los IDs `1`, `1.0` y `"1"` se consideran el mismo.
- Las localidades de `General` sin filas en la planilla aparecen en el Resumen sin fórmula
  (antes apuntaban a una hoja inexistente).
- Las celdas vacías usan la fuente de la BD (antes, la del libro nuevo que creaba Excel).
