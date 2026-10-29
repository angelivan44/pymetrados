package metrado

// Configuración de la plantilla: dónde escribir cada dato en P_0 y M_0 y hasta
// qué columna copiar la planilla. Cada dato sale, en este orden, de lo que el
// usuario eligió en la página, de la hoja Configuracion de la BD o de una
// detección automática; si no hay ninguno, se pide en la página.

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/xuri/excelize/v2"
)

const (
	OrigenConfiguracion = "configuracion"
	OrigenDetectado     = "detectado"
	OrigenUsuario       = "usuario"
)

type campo struct {
	Clave, Titulo, Ayuda, Hoja, Tipo string
	celda                            string // celda de la hoja Configuracion
}

// campos sigue el orden de la hoja Configuracion (B2..B10).
var campos = []campo{
	{"nombre", "Nombre de la localidad", "Celda de P_0 donde se escribe el nombre (columna B de General).", "P_0", "celda", "B2"},
	{"codigo", "Código de la localidad", "Celda de P_0 donde se escribe el código o nomenclatura (columna C de General).", "P_0", "celda", "B3"},
	{"sistema", "Sistema", "Celda de P_0 donde se escribe el sistema (columna D de General).", "P_0", "celda", "B4"},
	{"ubicacion", "Ubicación", "Celda de P_0 donde se escribe la ubicación (columna E de General).", "P_0", "celda", "B5"},
	{"primeraFila", "Primera celda de datos", "Celda de P_0 donde va el primer valor del primer elemento; desde ahí se copian las filas de PLANILLA.", "P_0", "celda", "B6"},
	{"cantidad", "Cantidad de elementos", "Celda de P_0 donde se escribe cuántos elementos tiene la localidad (la usan las fórmulas de totales).", "P_0", "celda", "B7"},
	{"filaMetrado", "Fila de la localidad en el metrado", "Fila de M_0 donde se escriben el ID (columna A) y el nombre (columna C). Las filas de arriba reciben la planilla transpuesta.", "M_0", "fila", "B8"},
	{"celdaResumen", "Celda que va al Resumen", "Primera celda del metrado parcial en M_0: el Resumen la toma de cada M_ y la extiende hacia abajo.", "M_0", "celda", "B9"},
	{"ultimaColumna", "Última columna de la planilla", "Hasta qué columna de PLANILLA se copian los datos.", "PLANILLA", "columna", "B10"},
}

// CampoConfig es un dato de configuración tal como lo ve la página. Valor está
// en términos de la plantilla: la celda donde cae el dato, el número de fila o
// la letra de columna ("" si falta).
type CampoConfig struct {
	Clave   string `json:"clave"`
	Titulo  string `json:"titulo"`
	Ayuda   string `json:"ayuda"`
	Hoja    string `json:"hoja"`
	Tipo    string `json:"tipo"`
	Valor   string `json:"valor"`
	Origen  string `json:"origen"`
	Detalle string `json:"detalle,omitempty"`
}

var (
	columnRef = regexp.MustCompile(`^\$?([A-Z]{1,3})\$?(?:\d+)?$`)
	offsetRef = regexp.MustCompile(`(?i)OFFSET\(\s*\$?([A-Z]{1,3})\$?(\d+)(?::\$?[A-Z]{1,3}\$?\d+)?\s*,\s*0\s*,\s*0\s*,\s*\$?([A-Z]{1,3})\$?(\d+)\s*[,)]`)
)

// normalize lleva un valor escrito (en la hoja o en la página) al formato de
// CampoConfig.Valor. ok=false si no tiene sentido para ese tipo de dato.
func normalize(c campo, raw string, desdeHoja bool) (string, bool) {
	text := strings.ToUpper(strings.TrimSpace(raw))
	text = strings.TrimPrefix(text, "=")
	switch c.Tipo {
	case "fila":
		n, err := strconv.ParseFloat(text, 64)
		if err != nil || n != float64(int(n)) || n < 3 || n > excelize.TotalRows {
			return "", false
		}
		return strconv.Itoa(int(n)), true
	case "columna":
		m := columnRef.FindStringSubmatch(text)
		if m == nil {
			return "", false
		}
		return m[1], true
	}
	m := cellRef.FindStringSubmatch(text)
	if m == nil {
		return "", false
	}
	col, err := excelize.ColumnNameToNumber(m[1])
	row, _ := strconv.Atoi(m[2])
	if err != nil || row < 1 {
		return "", false
	}
	// En la hoja Configuracion la "primera fila" es la celda a la izquierda del
	// primer dato (A15 → los datos empiezan en B15); la página usa el primer dato.
	if c.Clave == "primeraFila" && desdeHoja {
		col++
	}
	if c.Clave == "primeraFila" && col < 2 {
		return "", false
	}
	return cellName(col, row), true
}

// sheetValue es lo que se escribe en la hoja Configuracion para un valor normalizado.
func sheetValue(c campo, valor string) any {
	switch c.Tipo {
	case "fila":
		n, _ := strconv.Atoi(valor)
		return n
	case "columna":
		return valor + "1"
	}
	if c.Clave == "primeraFila" {
		col, row, _ := excelize.CellNameToCoordinates(valor)
		return cellName(col-1, row)
	}
	return valor
}

type detection struct{ valor, detalle string }

// filaDatos guarda la fila de datos detectada aunque no se sepa la columna.
const filaDatos = "_filaDatos"

// detectP0 busca las fórmulas OFFSET($X$15,0,0,$C$10) típicas de P_0: dan la
// fila de los datos y la celda con la cantidad de elementos.
func detectP0(f *excelize.File, sheet string, grid [][]string) map[string]detection {
	type pair struct {
		row  int
		cell string
	}
	counts := map[pair]int{}
	examples := map[pair]string{}
	for r, row := range grid {
		for c := range row {
			formula, _ := f.GetCellFormula(sheet, cellName(c+1, r+1))
			for _, m := range offsetRef.FindAllStringSubmatch(formula, -1) {
				n, _ := strconv.Atoi(m[2])
				p := pair{n, strings.ToUpper(m[3]) + m[4]}
				counts[p]++
				if examples[p] == "" {
					examples[p] = m[0]
				}
			}
		}
	}
	var best pair
	for p, n := range counts {
		if n > counts[best] || (n == counts[best] && p.row < best.row) {
			best = p
		}
	}
	out := map[string]detection{}
	if counts[best] == 0 {
		return out
	}
	detalle := fmt.Sprintf("Detectado en las fórmulas de P_0, como %s).", strings.TrimRight(examples[best], ",)"))
	out["cantidad"] = detection{best.cell, detalle}
	out[filaDatos] = detection{strconv.Itoa(best.row), detalle}

	// Los datos empiezan en la columna del encabezado más a la izquierda,
	// justo arriba de la primera fila.
	left := 0
	for r := max(1, best.row-4); r < best.row && r <= len(grid); r++ {
		for c, v := range grid[r-1] {
			if strings.TrimSpace(v) != "" {
				if left == 0 || c+1 < left {
					left = c + 1
				}
				break
			}
		}
	}
	if left >= 2 {
		out["primeraFila"] = detection{cellName(left, best.row), detalle}
	}
	return out
}

// detectM0 usa el encabezado ITEM de la tabla del metrado: la fila de la
// localidad está dos filas arriba y el total que va al Resumen está en la
// columna METRADO, en la primera fila de partidas.
func detectM0(grid [][]string) map[string]detection {
	out := map[string]detection{}
	for r, row := range grid {
		for c, v := range row {
			if c >= 12 || simplify(v) != "item" {
				continue
			}
			header := r + 1
			if header-2 < 3 {
				return out
			}
			detalle := fmt.Sprintf("Detectado por el encabezado ITEM en %s de M_0.", cellName(c+1, header))
			out["filaMetrado"] = detection{strconv.Itoa(header - 2), detalle}
			for c2, v2 := range row {
				if strings.Contains(simplify(v2), "metrado") {
					out["celdaResumen"] = detection{cellName(c2+1, header+3), fmt.Sprintf("Detectado por el encabezado %s en %s de M_0.", oneLine(v2), cellName(c2+1, header))}
					break
				}
			}
			return out
		}
	}
	return out
}

func oneLine(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// sheetGrid lee hasta maxRows filas de la hoja con el valor mostrado de cada celda.
func sheetGrid(f *excelize.File, sheet string, maxRows int) [][]string {
	rows, err := f.Rows(sheet)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var grid [][]string
	for rows.Next() && len(grid) < maxRows {
		cols, err := rows.Columns()
		if err != nil {
			break
		}
		grid = append(grid, cols)
	}
	return grid
}

// Vista es un recorte de una hoja para mostrarlo en la página.
type Vista struct {
	Hoja       string            `json:"hoja"`
	Filas      int               `json:"filas"`
	Columnas   int               `json:"columnas"`
	Celdas     map[string]string `json:"celdas"`
	Combinadas []string          `json:"combinadas"`
}

func makeVista(f *excelize.File, sheet string, grid [][]string, rows, minCols, maxCols int) *Vista {
	v := &Vista{Hoja: sheet, Filas: min(rows, len(grid)), Celdas: map[string]string{}, Combinadas: []string{}}
	cols := minCols
	for r := 0; r < v.Filas; r++ {
		for c, text := range grid[r] {
			if c >= maxCols || strings.TrimSpace(text) == "" {
				continue
			}
			text = oneLine(text)
			if runes := []rune(text); len(runes) > 40 {
				text = string(runes[:39]) + "…"
			}
			v.Celdas[cellName(c+1, r+1)] = text
			cols = max(cols, c+1)
		}
	}
	v.Columnas = min(cols+1, maxCols)
	v.Filas = max(v.Filas, min(rows, 12))
	merged, _ := f.GetMergeCells(sheet)
	for _, m := range merged {
		c1, r1, _ := excelize.CellNameToCoordinates(m.GetStartAxis())
		if c1 <= v.Columnas && r1 <= v.Filas {
			v.Combinadas = append(v.Combinadas, m.GetStartAxis()+":"+m.GetEndAxis())
		}
	}
	sort.Strings(v.Combinadas)
	return v
}

// templateSignature identifica la plantilla de la BD para que la página
// recuerde la configuración elegida aunque el archivo cambie de nombre.
func templateSignature(sheets []string, p0, m0 [][]string) string {
	h := sha256.New()
	fmt.Fprintln(h, strings.Join(sheets, "\x00"))
	for _, grid := range [][][]string{p0, m0} {
		for _, row := range grid {
			fmt.Fprintln(h, strings.Join(row, "\x00"))
		}
		fmt.Fprintln(h, "\x01")
	}
	return hex.EncodeToString(h.Sum(nil))[:16]
}

type template struct {
	f        *excelize.File
	sheets   Sheets
	p0, m0   [][]string
	detected map[string]detection
}

func readTemplate(f *excelize.File, sheets Sheets) *template {
	t := &template{f: f, sheets: sheets, detected: map[string]detection{}}
	t.p0 = sheetGrid(f, sheets.P0, 80)
	t.m0 = sheetGrid(f, sheets.M0, 200)
	for k, d := range detectP0(f, sheets.P0, t.p0) {
		t.detected[k] = d
	}
	for k, d := range detectM0(t.m0) {
		t.detected[k] = d
	}
	return t
}

// resolveConfig arma la configuración. lastHeader es la última columna con
// encabezado en PLANILLA (0 si no se subió la planilla). Devuelve nil si falta
// algún dato o alguno es inválido; los problemas quedan en out.
func resolveConfig(t *template, override map[string]string, lastHeader int, out *collector) (*Config, []CampoConfig) {
	where := labelBD + " › " + t.sheets.Configuracion
	var fields []CampoConfig
	var missing []campo
	valid := true
	for _, c := range campos {
		field := CampoConfig{Clave: c.Clave, Titulo: c.Titulo, Ayuda: c.Ayuda, Hoja: c.Hoja, Tipo: c.Tipo}
		if raw := strings.TrimSpace(override[c.Clave]); raw != "" {
			if v, ok := normalize(c, raw, false); ok {
				field.Valor, field.Origen = v, OrigenUsuario
			} else {
				out.error(fmt.Sprintf("El valor elegido para «%s» no es válido: '%s'.", c.Titulo, raw), "Configuración de la página", "Vuelve a elegirlo en la vista de la plantilla.")
				valid = false
			}
		}
		if field.Origen == "" && t.sheets.Configuracion != "" {
			col, row, _ := excelize.CellNameToCoordinates(c.celda)
			raw := cellValue(t.f, t.sheets.Configuracion, col, row)
			if !isBlank(raw) {
				if v, ok := normalize(c, valueText(raw), true); ok {
					field.Valor, field.Origen = v, OrigenConfiguracion
				} else {
					out.error(
						fmt.Sprintf("%s!%s no es válido para «%s». Valor actual: %s.", t.sheets.Configuracion, c.celda, c.Titulo, shown(raw)),
						where+"!"+c.celda,
						configHint(c)+" También puedes elegirlo en la vista de la plantilla, más abajo.",
					)
					valid = false
					fields = append(fields, field)
					continue
				}
			}
		}
		if field.Origen == "" {
			if d, ok := t.detected[c.Clave]; ok {
				field.Valor, field.Origen, field.Detalle = d.valor, OrigenDetectado, d.detalle
			} else if c.Clave == "ultimaColumna" && lastHeader > 0 {
				col, _ := excelize.ColumnNumberToName(lastHeader)
				field.Valor, field.Origen, field.Detalle = col, OrigenDetectado, "Detectado: es la última columna con encabezado en PLANILLA."
			}
		}
		if field.Origen == "" && !(c.Clave == "ultimaColumna" && lastHeader == 0) {
			missing = append(missing, c)
			valid = false
		}
		fields = append(fields, field)
	}
	if len(missing) > 0 {
		var titles []string
		for _, c := range missing {
			titles = append(titles, "«"+c.Titulo+"»")
		}
		w := labelBD + " › " + missing[0].Hoja
		if t.sheets.Configuracion != "" && len(missing) == 1 {
			w = where + "!" + missing[0].celda
		}
		message := fmt.Sprintf("Falta indicar %s.", titles[0])
		if len(titles) > 1 {
			message = fmt.Sprintf("Falta indicar dónde va %s y %s.", strings.Join(titles[:len(titles)-1], ", "), titles[len(titles)-1])
		}
		out.error(message, w, "No se pudieron detectar solos: elígelos con un clic en la vista de la plantilla.")
	}
	if !valid {
		return nil, fields
	}

	value := map[string]string{}
	for _, f := range fields {
		value[f.Clave] = f.Valor
	}
	cfg := &Config{
		NameCell: value["nombre"], IDCell: value["codigo"], SystemCell: value["sistema"], LocationCell: value["ubicacion"],
		LengthCell: value["cantidad"], FirstValueCell: value["celdaResumen"],
	}
	cfg.FirstRowMontaje, _ = strconv.Atoi(value["filaMetrado"])
	dataCol, dataRow, _ := excelize.CellNameToCoordinates(value["primeraFila"])
	cfg.FirstCol, cfg.FirstRow = dataCol-1, dataRow
	cfg.FirstRowCell = cellName(cfg.FirstCol, cfg.FirstRow)
	if value["ultimaColumna"] != "" {
		cfg.HeaderEndCell = value["ultimaColumna"] + "1"
	}

	// Si la hoja Configuracion contradice las fórmulas de P_0, avisar: es el
	// error típico al mover la tabla de la plantilla.
	if d, ok := t.detected[filaDatos]; ok {
		if f := fieldByKey(fields, "primeraFila"); f.Origen == OrigenConfiguracion && d.valor != strconv.Itoa(cfg.FirstRow) {
			out.warning(
				fmt.Sprintf("Configuracion!B6 indica que los datos empiezan en la fila %d, pero las fórmulas de P_0 usan la fila %s.", cfg.FirstRow, d.valor),
				where+"!B6", "Revisa cuál es la correcta en la vista de la plantilla.")
		}
	}
	if d, ok := t.detected["cantidad"]; ok {
		if f := fieldByKey(fields, "cantidad"); f.Origen == OrigenConfiguracion && d.valor != f.Valor {
			out.warning(
				fmt.Sprintf("Configuracion!B7 indica %s como celda de la cantidad de elementos, pero las fórmulas de P_0 usan %s.", f.Valor, d.valor),
				where+"!B7", "Revisa cuál es la correcta en la vista de la plantilla.")
		}
	}
	return cfg, fields
}

func configHint(c campo) string {
	switch c.Tipo {
	case "fila":
		return "Escribe solo el número de fila, por ejemplo 47."
	case "columna":
		return "Escribe la última celda de encabezados, por ejemplo AG1."
	}
	return "Escribe una dirección como A15, sin signo '=' ni nombre de hoja."
}

func fieldByKey(fields []CampoConfig, key string) CampoConfig {
	for _, f := range fields {
		if f.Clave == key {
			return f
		}
	}
	return CampoConfig{}
}

func campoByKey(key string) campo {
	for _, c := range campos {
		if c.Clave == key {
			return c
		}
	}
	return campo{}
}

// vistas recorta P_0 y M_0 alrededor de las celdas configuradas.
func (t *template) vistas(fields []CampoConfig) map[string]*Vista {
	p0Rows, m0Rows, m0Cols := 30, 60, 10
	for _, f := range fields {
		if f.Valor == "" {
			continue
		}
		switch f.Clave {
		case "primeraFila", "nombre", "codigo", "sistema", "ubicacion", "cantidad":
			_, r, _ := excelize.CellNameToCoordinates(f.Valor)
			p0Rows = max(p0Rows, r+3)
		case "filaMetrado":
			r, _ := strconv.Atoi(f.Valor)
			m0Rows = max(m0Rows, r+8)
		case "celdaResumen":
			c, r, _ := excelize.CellNameToCoordinates(f.Valor)
			m0Rows, m0Cols = max(m0Rows, r+3), max(m0Cols, c+2)
		}
	}
	return map[string]*Vista{
		"P_0": makeVista(t.f, t.sheets.P0, t.p0, min(p0Rows, 80), 8, 50),
		"M_0": makeVista(t.f, t.sheets.M0, t.m0, min(m0Rows, 200), m0Cols, min(m0Cols, 30)),
	}
}

// WriteConfig devuelve la BD con la hoja Configuracion escrita (o actualizada)
// con los valores de la configuración resuelta.
func WriteConfig(bd []byte, fields []CampoConfig) ([]byte, error) {
	f, err := excelize.OpenReader(bytes.NewReader(bd))
	if err != nil {
		return nil, err
	}
	defer f.Close()
	sheet := ""
	for _, n := range f.GetSheetList() {
		if simplify(n) == simplify("Configuracion") {
			sheet = n
			break
		}
	}
	if sheet == "" {
		sheet = "Configuracion"
		if _, err := f.NewSheet(sheet); err != nil {
			return nil, err
		}
		if err := f.SetSheetRow(sheet, "A1", &[]any{"Dato", "Celda", "Qué es"}); err != nil {
			return nil, err
		}
		_ = f.SetColWidth(sheet, "A", "A", 34)
		_ = f.SetColWidth(sheet, "C", "C", 90)
	}
	for _, c := range campos {
		field := fieldByKey(fields, c.Clave)
		if field.Valor == "" {
			continue
		}
		_, row, _ := excelize.CellNameToCoordinates(c.celda)
		if v, _ := f.GetCellValue(sheet, cellName(1, row)); strings.TrimSpace(v) == "" {
			_ = f.SetCellValue(sheet, cellName(1, row), c.Titulo)
			_ = f.SetCellValue(sheet, cellName(3, row), c.Ayuda)
		}
		if err := f.SetCellValue(sheet, c.celda, sheetValue(c, field.Valor)); err != nil {
			return nil, err
		}
	}
	buf, err := f.WriteToBuffer()
	if err != nil {
		return nil, err
	}
	// Mismo arreglo que el metrado: excelize reescribe los nombres definidos.
	return finalize(buf.Bytes(), bd, map[string]bool{}, nil, nil)
}
