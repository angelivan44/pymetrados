package metrado

import (
	"bytes"
	"errors"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"github.com/xuri/excelize/v2"
	"golang.org/x/text/unicode/norm"
)

const (
	labelBD       = "BD"
	labelPlanilla = "Planilla"
	maxSheetName  = 31
)

var cellRef = regexp.MustCompile(`^\$?([A-Z]{1,3})\$?(\d+)$`)

// Value es el valor mostrado por una celda: nil, float64, string o bool.
type Value = any

type Issue struct {
	Level   string `json:"level"`
	Message string `json:"message"`
	Where   string `json:"where,omitempty"`
	Hint    string `json:"hint,omitempty"`
}

type Config struct {
	NameCell, IDCell, SystemCell, LocationCell string
	FirstRowCell, LengthCell                   string
	FirstRowMontaje                            int
	FirstValueCell, HeaderEndCell              string
	FirstRow, FirstCol                         int
}

type Project struct {
	Key                              string // ID normalizado; se usa en los nombres de hoja
	ID, Name, Code, System, Location Value  // valores originales de la fila de General
	Row                              int
}

type Group struct {
	Project    *Project
	SourceRows []int
	Rows       [][]Value
}

// Sheets guarda el nombre real de cada hoja esperada de la BD.
type Sheets struct {
	General, Configuracion, P0, M0 string
}

type Inputs struct {
	BD       []byte
	Sheets   Sheets
	Config   Config
	Projects []*Project // orden de la hoja General
	Groups   []*Group   // orden por ID, como la versión de escritorio
	Headers  []Value
}

type ProjectSummary struct {
	ID     string  `json:"id"`
	Nombre *string `json:"nombre"`
	Filas  int     `json:"filas"`
}

type Summary struct {
	Localidades int              `json:"localidades"`
	Filas       int              `json:"filas"`
	Columnas    int              `json:"columnas"`
	Detalle     []ProjectSummary `json:"detalle"`
}

// Check es una comprobación que salió bien o un dato informativo sobre un
// archivo; los problemas van en Errors y Warnings.
type Check struct {
	Archivo string `json:"archivo"` // "bd" o "planilla"
	Nivel   string `json:"nivel"`   // "ok", "info" o "pendiente"
	Texto   string `json:"texto"`
}

type Report struct {
	OK       bool              `json:"ok"`
	Errors   []Issue           `json:"errors"`
	Warnings []Issue           `json:"warnings"`
	Summary  *Summary          `json:"summary"`
	Checks   []Check           `json:"checks"`
	Config   []CampoConfig     `json:"config,omitempty"`
	Vistas   map[string]*Vista `json:"vistas,omitempty"`
	Firma    string            `json:"firma,omitempty"`
}

type collector struct {
	issues []Issue
	checks []Check
}

func (c *collector) check(archivo, nivel, texto string) {
	c.checks = append(c.checks, Check{archivo, nivel, texto})
}

func (c *collector) error(message, where, hint string) {
	c.issues = append(c.issues, Issue{"error", message, where, hint})
}

func (c *collector) warning(message, where, hint string) {
	c.issues = append(c.issues, Issue{"warning", message, where, hint})
}

func (c *collector) report(summary *Summary) Report {
	r := Report{Errors: []Issue{}, Warnings: []Issue{}, Summary: summary, Checks: c.checks}
	if r.Checks == nil {
		r.Checks = []Check{}
	}
	for _, i := range c.issues {
		if i.Level == "error" {
			r.Errors = append(r.Errors, i)
		} else {
			r.Warnings = append(r.Warnings, i)
		}
	}
	r.OK = len(r.Errors) == 0
	return r
}

var errAbort = errors.New("abort")

// cellValue devuelve el valor mostrado por la celda (el resultado si es fórmula).
func cellValue(f *excelize.File, sheet string, col, row int) Value {
	cell, _ := excelize.CoordinatesToCellName(col, row)
	raw, err := f.GetCellValue(sheet, cell, excelize.Options{RawCellValue: true})
	if err != nil || raw == "" {
		return nil
	}
	kind, _ := f.GetCellType(sheet, cell)
	switch kind {
	case excelize.CellTypeSharedString, excelize.CellTypeInlineString, excelize.CellTypeFormula, excelize.CellTypeError:
		return raw
	case excelize.CellTypeBool:
		return raw == "1" || strings.EqualFold(raw, "true")
	}
	if n, err := strconv.ParseFloat(raw, 64); err == nil {
		return n
	}
	return raw
}

func formatNumber(n float64) string {
	return strconv.FormatFloat(n, 'f', -1, 64)
}

func valueText(v Value) string {
	switch x := v.(type) {
	case nil:
		return ""
	case float64:
		return formatNumber(x)
	case bool:
		if x {
			return "True"
		}
		return "False"
	default:
		return fmt.Sprint(x)
	}
}

var numericText = regexp.MustCompile(`(?i)^[+-]?(\d+\.?\d*|\.\d+)(e[+-]?\d+)?$`)

// NormalizeID convierte 1, 1.0, "1" y " 1 " en la misma clave.
func NormalizeID(v Value) (string, bool) {
	switch x := v.(type) {
	case nil:
		return "", false
	case float64:
		return formatNumber(x), true
	case bool:
		return valueText(x), true
	}
	text := strings.TrimSpace(valueText(v))
	if text == "" {
		return "", false
	}
	if numericText.MatchString(text) {
		if n, err := strconv.ParseFloat(text, 64); err == nil && !math.IsInf(n, 0) {
			return formatNumber(n), true
		}
	}
	return text, true
}

func idLess(a, b string) bool {
	na, errA := strconv.ParseFloat(a, 64)
	nb, errB := strconv.ParseFloat(b, 64)
	switch {
	case errA == nil && errB == nil:
		return na < nb
	case errA == nil:
		return true
	case errB == nil:
		return false
	}
	return strings.ToLower(a) < strings.ToLower(b)
}

func simplify(name string) string {
	var b strings.Builder
	for _, r := range norm.NFKD.String(name) {
		if unicode.Is(unicode.Mn, r) || unicode.IsSpace(r) {
			continue
		}
		b.WriteRune(unicode.ToLower(r))
	}
	return b.String()
}

// similarity es el coeficiente de Dice sobre bigramas: suficiente para sugerir correcciones.
func similarity(a, b string) float64 {
	a, b = strings.ToLower(a), strings.ToLower(b)
	if a == b {
		return 1
	}
	ra, rb := []rune(a), []rune(b)
	if len(ra) < 2 || len(rb) < 2 {
		return 0
	}
	grams := map[string]int{}
	for i := 0; i < len(ra)-1; i++ {
		grams[string(ra[i:i+2])]++
	}
	common := 0
	for i := 0; i < len(rb)-1; i++ {
		g := string(rb[i : i+2])
		if grams[g] > 0 {
			grams[g]--
			common++
		}
	}
	return 2 * float64(common) / float64(len(ra)-1+len(rb)-1)
}

func closest(word string, options []string) string {
	best, score := "", 0.6
	for _, o := range options {
		if s := similarity(word, o); s >= score {
			best, score = o, s
		}
	}
	return best
}

func open(data []byte, label string, out *collector) (*excelize.File, error) {
	if len(data) >= 4 && bytes.Equal(data[:4], []byte{0xd0, 0xcf, 0x11, 0xe0}) {
		out.error(
			fmt.Sprintf("El archivo %s está en formato .xls antiguo o protegido con contraseña.", label),
			label,
			"Ábrelo en Excel, quita la contraseña si tiene y usa 'Guardar como' → Libro de Excel (.xlsx).",
		)
		return nil, errAbort
	}
	f, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		out.error(fmt.Sprintf("No se pudo abrir el archivo %s.", label), label, "Verifica que sea un .xlsx válido y que no esté dañado.")
		return nil, errAbort
	}
	return f, nil
}

func findSheet(f *excelize.File, expected, label string, out *collector) string {
	return lookupSheet(f, expected, label, true, out)
}

// lookupSheet busca la hoja por nombre exacto o parecido; si no está y es
// obligatoria, lo reporta como error.
func lookupSheet(f *excelize.File, expected, label string, required bool, out *collector) string {
	names := f.GetSheetList()
	for _, n := range names {
		if n == expected {
			return n
		}
	}
	for _, n := range names {
		if simplify(n) == simplify(expected) {
			out.warning(fmt.Sprintf("Se usó la hoja '%s' como '%s'.", n, expected), label,
				fmt.Sprintf("Renómbrala a '%s' para evitar ambigüedades.", expected))
			return n
		}
	}
	if !required {
		return ""
	}
	hint := ""
	if c := closest(expected, names); c != "" {
		hint = fmt.Sprintf("¿Quisiste decir '%s'? ", c)
	}
	out.error(fmt.Sprintf("Falta la hoja '%s' en el archivo %s.", expected, label), label,
		hint+fmt.Sprintf("Hojas encontradas: %s.", strings.Join(names, ", ")))
	return ""
}

func shown(v Value) string {
	if v == nil || strings.TrimSpace(valueText(v)) == "" {
		return "vacío"
	}
	return "'" + valueText(v) + "'"
}

func isBlank(v Value) bool {
	return v == nil || strings.TrimSpace(valueText(v)) == ""
}

func rowCount(f *excelize.File, sheet string) int {
	rows, err := f.GetRows(sheet, excelize.Options{RawCellValue: true})
	if err != nil {
		return 0
	}
	return len(rows)
}

func readGeneral(f *excelize.File, sheet string, out *collector) []*Project {
	where := labelBD + " › " + sheet
	var projects []*Project
	seen := map[string]int{}
	for r := 2; r <= rowCount(f, sheet); r++ {
		row := make([]Value, 5)
		blank := true
		for c := range row {
			row[c] = cellValue(f, sheet, c+1, r)
			blank = blank && isBlank(row[c])
		}
		if blank {
			continue
		}
		key, ok := NormalizeID(row[0])
		if !ok {
			out.warning(fmt.Sprintf("La fila %d de General tiene datos pero no tiene ID; se ignora.", r), fmt.Sprintf("%s!A%d", where, r), "")
			continue
		}
		if first, dup := seen[key]; dup {
			out.error(fmt.Sprintf("El ID '%s' está repetido en General (filas %d y %d).", key, first, r),
				fmt.Sprintf("%s!A%d", where, r), "Cada localidad debe tener un ID único.")
			continue
		}
		seen[key] = r
		if row[1] == nil {
			out.warning(fmt.Sprintf("La localidad '%s' no tiene nombre.", key), fmt.Sprintf("%s!B%d", where, r), "")
		}
		projects = append(projects, &Project{key, row[0], row[1], row[2], row[3], row[4], r})
	}
	if len(projects) == 0 {
		out.error("La hoja General no tiene localidades.", where, "Desde la fila 2: A=ID, B=Nombre, C=Código, D=Sistema, E=Ubicación.")
	}
	return projects
}

func checkSheetName(key string, out *collector) bool {
	var bad []string
	for _, ch := range `[]:*?/\` {
		if strings.ContainsRune(key, ch) {
			bad = append(bad, string(ch))
		}
	}
	if len(bad) > 0 {
		sort.Strings(bad)
		out.error(fmt.Sprintf("El ID '%s' contiene caracteres no permitidos en nombres de hoja: %s", key, strings.Join(bad, " ")),
			"", "Usa solo letras, números, guiones o espacios en los IDs.")
		return false
	}
	if len([]rune(key))+2 > maxSheetName {
		out.error(fmt.Sprintf("El ID '%s' es demasiado largo: las hojas P_%s y M_%s superan %d caracteres.", key, key, key, maxSheetName),
			"", fmt.Sprintf("Usa IDs de máximo %d caracteres.", maxSheetName-2))
		return false
	}
	return true
}

func rowsText(rows []int) string {
	const limit = 8
	var parts []string
	for i, r := range rows {
		if i == limit {
			break
		}
		parts = append(parts, strconv.Itoa(r))
	}
	text := strings.Join(parts, ", ")
	if len(rows) > limit {
		text += fmt.Sprintf(" y %d más", len(rows)-limit)
	}
	return text
}

// isMeaningful descarta los 0 y vacíos que muestran las fórmulas de relleno (=DB!C124).
func isMeaningful(v Value) bool {
	if v == nil {
		return false
	}
	if n, ok := v.(float64); ok && n == 0 {
		return false
	}
	text := strings.TrimSpace(valueText(v))
	return text != "" && text != "0"
}

// lastHeaderCol devuelve la última columna con encabezado en la fila 1 de PLANILLA.
func lastHeaderCol(f *excelize.File, sheet string) int {
	rows, err := f.Rows(sheet)
	if err != nil {
		return 0
	}
	defer rows.Close()
	if !rows.Next() {
		return 0
	}
	cols, _ := rows.Columns(excelize.Options{RawCellValue: true})
	for i := len(cols) - 1; i >= 0; i-- {
		if strings.TrimSpace(cols[i]) != "" {
			return i + 1
		}
	}
	return 0
}

func columnName(col int) string {
	name, _ := excelize.ColumnNumberToName(col)
	return name
}

func plural(n int, singular, plural string) string {
	if n == 1 {
		return "1 " + singular
	}
	return fmt.Sprintf("%d %s", n, plural)
}

type planillaData struct {
	headers []Value
	groups  []*Group
}

// readPlanilla lee los encabezados (hasta headerEnd; 0 = hasta el último) y
// agrupa las filas por ID. Sin projects (no se subió la BD) no se cruzan los
// IDs con General.
func readPlanilla(f *excelize.File, dbName, planillaName string, headerEnd, lastHeader int, projects []*Project, out *collector) (*planillaData, error) {
	headerWhere := labelPlanilla + " › " + planillaName + "!1"
	if lastHeader == 0 {
		out.error("La fila 1 de PLANILLA no tiene encabezados.", headerWhere, "Escribe en la fila 1 el nombre de cada columna y los datos desde la fila 2.")
		return nil, errAbort
	}
	out.check("planilla", "ok", fmt.Sprintf("PLANILLA: %s con encabezado (A–%s).", plural(lastHeader, "columna", "columnas"), columnName(lastHeader)))
	if headerEnd == 0 {
		headerEnd = lastHeader
	}
	var headers []Value
	for c := 1; c <= headerEnd; c++ {
		headers = append(headers, cellValue(f, planillaName, c, 1))
	}
	for len(headers) > 0 && headers[len(headers)-1] == nil {
		headers = headers[:len(headers)-1]
	}
	if len(headers) == 0 {
		out.error(fmt.Sprintf("La fila 1 de PLANILLA no tiene encabezados en el rango configurado (A–%s).", columnName(headerEnd)),
			headerWhere, "Revisa «Última columna de la planilla» en la configuración.")
		return nil, errAbort
	}
	if headerEnd < lastHeader {
		var left []string
		for c := headerEnd + 1; c <= lastHeader && len(left) < 5; c++ {
			if h := cellValue(f, planillaName, c, 1); !isBlank(h) {
				left = append(left, fmt.Sprintf("%s (%s)", columnName(c), oneLine(valueText(h))))
			}
		}
		more := ""
		if lastHeader-headerEnd > len(left) {
			more = "…"
		}
		out.check("planilla", "info", fmt.Sprintf("Se copian las columnas A–%s; quedan fuera: %s%s.", columnName(headerEnd), strings.Join(left, ", "), more))
	}
	var blank, names []string
	for i, h := range headers {
		if h == nil {
			blank = append(blank, columnName(i+1))
		} else {
			names = append(names, strings.ToLower(strings.TrimSpace(valueText(h))))
		}
	}
	if len(blank) > 0 {
		out.warning(fmt.Sprintf("Columnas de PLANILLA sin encabezado: %s. Se copian igual.", strings.Join(blank, ", ")), headerWhere, "")
	}
	count := map[string]int{}
	for _, n := range names {
		count[n]++
	}
	var dupes []string
	for n, c := range count {
		if c > 1 {
			dupes = append(dupes, n)
		}
	}
	if len(dupes) > 0 {
		sort.Strings(dupes)
		out.warning(fmt.Sprintf("Encabezados repetidos en PLANILLA: %s.", strings.Join(dupes, ", ")), headerWhere,
			"Se copian todas las columnas; la versión de escritorio omitía columnas repetidas.")
	}
	width := len(headers)

	byKey := map[string]*Project{}
	var projectKeys []string
	for _, p := range projects {
		byKey[p.Key] = p
		projectKeys = append(projectKeys, p.Key)
	}
	type entry struct {
		row    int
		values []Value
	}
	rowsByKey := map[string][]entry{}
	unknown := map[string][]int{}
	var unknownOrder, order []string
	var orphanRows []int
	last := max(rowCount(f, dbName), rowCount(f, planillaName))
	firstIDRow, idRows := 0, 0
	for r := 2; r <= last; r++ {
		key, ok := NormalizeID(cellValue(f, dbName, 1, r))
		values := make([]Value, width)
		for c := range values {
			values[c] = cellValue(f, planillaName, c+1, r)
		}
		if !ok {
			for _, v := range values {
				if isMeaningful(v) {
					orphanRows = append(orphanRows, r)
					break
				}
			}
			continue
		}
		if firstIDRow == 0 {
			firstIDRow = r
		}
		idRows++
		if projects == nil {
			if _, seen := byKey[key]; !seen {
				byKey[key] = &Project{Key: key, ID: key}
			}
		} else if _, known := byKey[key]; !known {
			if _, seen := unknown[key]; !seen {
				unknownOrder = append(unknownOrder, key)
			}
			unknown[key] = append(unknown[key], r)
			continue
		}
		rowsByKey[key] = append(rowsByKey[key], entry{r, values})
		order = append(order, key)
	}

	// Tras el último ID, las planillas suelen traer filas de relleno con fórmulas
	// (=DB!C124) que muestran valores por defecto: no son datos reales.
	lastIDRow := 0
	for _, entries := range rowsByKey {
		for _, e := range entries {
			lastIDRow = max(lastIDRow, e.row)
		}
	}
	for _, rows := range unknown {
		for _, r := range rows {
			lastIDRow = max(lastIDRow, r)
		}
	}
	var orphans, filler []int
	for _, r := range orphanRows {
		if r < lastIDRow {
			orphans = append(orphans, r)
		} else {
			filler = append(filler, r)
		}
	}
	if idRows > 0 {
		out.check("planilla", "ok", fmt.Sprintf("DB: %s con ID, de la fila %d a la %d; %s.",
			plural(idRows, "fila", "filas"), firstIDRow, lastIDRow, plural(len(rowsByKey)+len(unknown), "localidad", "localidades")))
	}
	if len(filler) > 0 {
		out.check("planilla", "info", fmt.Sprintf("Se ignoran %s sin ID al final (desde la fila %d): son relleno.",
			plural(len(filler), "fila", "filas"), filler[0]))
	}
	if len(orphans) > 0 {
		out.error(fmt.Sprintf("Hay filas con datos en PLANILLA sin ID en la hoja %s (filas %s).", dbName, rowsText(orphans)),
			labelPlanilla+" › "+dbName+"!A",
			fmt.Sprintf("Cada fila de PLANILLA necesita su ID en la misma fila de %s, columna A.", dbName))
	}
	for _, key := range unknownOrder {
		rows := unknown[key]
		hint := ""
		if c := closest(key, projectKeys); c != "" {
			hint = fmt.Sprintf("¿Quisiste decir '%s'? ", c)
		}
		out.error(fmt.Sprintf("El ID '%s' (filas %s de %s) no existe en la hoja General de la BD.", key, rowsText(rows), dbName),
			fmt.Sprintf("%s › %s!A%d", labelPlanilla, dbName, rows[0]),
			hint+"Agrega la localidad a General o corrige el ID.")
	}
	if len(rowsByKey) == 0 && len(unknown) == 0 && len(orphans) == 0 {
		out.error(fmt.Sprintf("La hoja %s no tiene IDs desde la fila 2.", dbName), labelPlanilla+" › "+dbName+"!A2", "")
	}

	var sortedKeys []string
	for k := range rowsByKey {
		sortedKeys = append(sortedKeys, k)
	}
	sort.Slice(sortedKeys, func(i, j int) bool { return idLess(sortedKeys[i], sortedKeys[j]) })
	var contiguous []string
	for i, k := range order {
		if i == 0 || order[i-1] != k {
			contiguous = append(contiguous, k)
		}
	}
	if strings.Join(contiguous, "\x00") != strings.Join(sortedKeys, "\x00") {
		out.warning("Las filas de PLANILLA no están agrupadas ni ordenadas por ID. Se agruparon automáticamente.",
			labelPlanilla+" › "+dbName+"!A",
			"La versión de escritorio habría mezclado filas entre localidades con este orden.")
	}

	var missing []string
	for _, p := range projects {
		if _, ok := rowsByKey[p.Key]; !ok {
			missing = append(missing, p.Key)
		}
	}
	if len(missing) > 0 && len(rowsByKey) > 0 {
		list := missing
		if len(list) > 10 {
			list = list[:10]
		}
		more := ""
		if len(missing) > 10 {
			more = "…"
		}
		out.warning(fmt.Sprintf("%d localidad(es) de General no tienen filas en la planilla: %s%s. Aparecerán en el Resumen sin valores.",
			len(missing), strings.Join(list, ", "), more), labelBD+" › General", "")
	}

	var groups []*Group
	for _, key := range sortedKeys {
		if !checkSheetName(key, out) {
			continue
		}
		g := &Group{Project: byKey[key]}
		for _, e := range rowsByKey[key] {
			g.SourceRows = append(g.SourceRows, e.row)
			g.Rows = append(g.Rows, e.values)
		}
		groups = append(groups, g)
	}
	return &planillaData{headers, groups}, nil
}

// Opciones ajusta el análisis. Configuracion trae los datos elegidos en la
// página (clave de campo → valor), que mandan sobre la hoja Configuracion.
type Opciones struct {
	Configuracion map[string]string
}

// Analyze lee y valida la BD y la planilla. Nunca falla por culpa del usuario:
// todo problema se devuelve en el Report. Inputs es nil si no se puede generar.
func Analyze(bd, planilla []byte) (Report, *Inputs) {
	return AnalyzeWith(bd, planilla, Opciones{})
}

// AnalyzeWith acepta también uno solo de los archivos (el otro vacío): revisa
// lo que puede para que la página muestre el avance mientras falta el otro.
func AnalyzeWith(bd, planilla []byte, opts Opciones) (Report, *Inputs) {
	out := &collector{}
	var (
		sheets   Sheets
		tpl      *template
		projects []*Project
		bdFile   *excelize.File
	)
	if len(bd) > 0 {
		if f, err := open(bd, labelBD, out); err == nil {
			defer f.Close()
			bdFile = f
			sheets = Sheets{
				General:       findSheet(f, "General", labelBD, out),
				Configuracion: lookupSheet(f, "Configuracion", labelBD, false, out),
				P0:            findSheet(f, "P_0", labelBD, out),
				M0:            findSheet(f, "M_0", labelBD, out),
			}
			if sheets.General != "" {
				projects = readGeneral(f, sheets.General, out)
				if n := len(projects); n > 0 {
					out.check("bd", "ok", fmt.Sprintf("General: %s (filas %d–%d).", plural(n, "localidad", "localidades"), projects[0].Row, projects[n-1].Row))
				}
			}
			if sheets.P0 != "" && sheets.M0 != "" {
				tpl = readTemplate(f, sheets)
				out.check("bd", "ok", "Plantillas P_0 y M_0 encontradas.")
			}
		}
	}

	var planillaFile *excelize.File
	var dbName, planillaName string
	lastHeader := 0
	if len(planilla) > 0 {
		if f, err := open(planilla, labelPlanilla, out); err == nil {
			defer f.Close()
			planillaFile = f
			dbName = findSheet(f, "DB", labelPlanilla, out)
			planillaName = findSheet(f, "PLANILLA", labelPlanilla, out)
			if planillaName != "" {
				lastHeader = lastHeaderCol(f, planillaName)
			}
		}
	}

	var cfg *Config
	var fields []CampoConfig
	if tpl != nil {
		cfg, fields = resolveConfig(tpl, opts.Configuracion, lastHeader, out)
		configCheck(fields, sheets.Configuracion != "", out)
	}

	var pl *planillaData
	if dbName != "" && planillaName != "" {
		headerEnd := 0
		if cfg != nil && cfg.HeaderEndCell != "" {
			m := cellRef.FindStringSubmatch(cfg.HeaderEndCell)
			headerEnd, _ = excelize.ColumnNameToNumber(m[1])
		}
		var known []*Project
		if len(projects) > 0 {
			known = projects
		}
		pl, _ = readPlanilla(planillaFile, dbName, planillaName, headerEnd, lastHeader, known, out)
	}

	var summary *Summary
	var inputs *Inputs
	if cfg != nil && pl != nil && len(projects) > 0 {
		if len(pl.headers) >= cfg.FirstRowMontaje-1 {
			out.error(
				fmt.Sprintf("Hay %d encabezados, pero M_0 solo reserva %d filas antes del metrado («%s» = %d).",
					len(pl.headers), cfg.FirstRowMontaje-2, campoByKey("filaMetrado").Titulo, cfg.FirstRowMontaje),
				labelBD+" › M_0",
				"Las filas superiores de cada M_ reciben los datos transpuestos de la planilla; baja la fila de la localidad en M_0 o reduce columnas.",
			)
		}
		summary = &Summary{Localidades: len(pl.groups), Columnas: len(pl.headers), Detalle: []ProjectSummary{}}
		for _, g := range pl.groups {
			summary.Filas += len(g.Rows)
			var name *string
			if g.Project.Name != nil {
				s := valueText(g.Project.Name)
				name = &s
			}
			summary.Detalle = append(summary.Detalle, ProjectSummary{g.Project.Key, name, len(g.Rows)})
		}
		inputs = &Inputs{bd, sheets, *cfg, projects, pl.groups, pl.headers}
	}

	report := out.report(summary)
	report.OK = report.OK && inputs != nil
	if tpl != nil {
		report.Config = fields
		report.Vistas = tpl.vistas(fields)
		report.Firma = templateSignature(bdFile.GetSheetList(), tpl.p0, tpl.m0)
	}
	if planillaName != "" && lastHeader > 0 {
		if report.Vistas == nil {
			report.Vistas = map[string]*Vista{}
		}
		report.Vistas["PLANILLA"] = makeVista(planillaFile, planillaName, sheetGrid(planillaFile, planillaName, 4), 4, lastHeader, min(lastHeader+1, 120))
	}
	return report, inputs
}

// configCheck resume de dónde salió cada dato de configuración.
func configCheck(fields []CampoConfig, hasSheet bool, out *collector) {
	counts := map[string]int{}
	var missing []string
	for _, f := range fields {
		if f.Valor == "" {
			if f.Clave != "ultimaColumna" || hasSheet {
				missing = append(missing, f.Titulo)
			}
			continue
		}
		counts[f.Origen]++
	}
	var parts []string
	if n := counts[OrigenConfiguracion]; n > 0 {
		parts = append(parts, fmt.Sprintf("%d de la hoja Configuracion", n))
	}
	if n := counts[OrigenDetectado]; n > 0 {
		parts = append(parts, plural(n, "detectado", "detectados"))
	}
	if n := counts[OrigenUsuario]; n > 0 {
		parts = append(parts, plural(n, "elegido en la página", "elegidos en la página"))
	}
	origin := strings.Join(parts, ", ")
	switch {
	case len(missing) > 0:
		out.check("bd", "pendiente", fmt.Sprintf("Configuración: falta indicar %s.", strings.Join(missing, ", ")))
	case len(parts) == 1 && counts[OrigenConfiguracion] > 0:
		out.check("bd", "ok", "Configuración tomada de la hoja Configuracion.")
	default:
		out.check("bd", "ok", "Configuración completa ("+origin+").")
	}
}
