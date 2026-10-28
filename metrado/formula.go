package metrado

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/xuri/excelize/v2"
)

// Referencia con hoja opcional: A1, $A$1:B2, 3:5, A:C, DATOS!J47, 'Mi hoja'!A1.
// RE2 no tiene lookbehind/lookahead; los bordes se validan en translateSegment.
var reference = regexp.MustCompile(
	`((?:'(?:[^']|'')+'|\[\d+\][A-Za-z_][\w.]*|[A-Za-z_][\w.]*)!)?` +
		`(\$?[A-Za-z]{1,3}\$?\d+(?::\$?[A-Za-z]{1,3}\$?\d+)?|\$?\d+:\$?\d+|\$?[A-Za-z]{1,3}:\$?[A-Za-z]{1,3})`,
)

var refPart = regexp.MustCompile(`^(\$?)([A-Za-z]{1,3})?(\$?)(\d+)?$`)

var plainSheet = regexp.MustCompile(`^[A-Za-z_][\w.]*$`)
var looksLikeCell = regexp.MustCompile(`^[A-Za-z]{1,3}\d+$`)

// QuoteSheet devuelve el nombre de hoja listo para usar en una fórmula.
func QuoteSheet(name string) string {
	if plainSheet.MatchString(name) && !looksLikeCell.MatchString(name) {
		return name
	}
	return "'" + strings.ReplaceAll(name, "'", "''") + "'"
}

func isWordByte(b byte) bool {
	return b == '_' || b >= '0' && b <= '9' || b >= 'A' && b <= 'Z' || b >= 'a' && b <= 'z'
}

// TranslateFormula mueve las referencias relativas como lo hace Excel al copiar
// una celda dRows filas y dCols columnas (AutoFill / copiar-pegar).
func TranslateFormula(formula string, dRows, dCols int) string {
	if dRows == 0 && dCols == 0 {
		return formula
	}
	return rewriteRefs(formula, func(prefix, ref string) string {
		return translateRef(prefix, ref, dRows, dCols)
	})
}

// DeleteRowsFormula ajusta la fórmula de una celda de `sheet` como Excel al
// borrar `n` filas desde la fila `at`: las referencias a la propia hoja que
// apuntaban más abajo suben, las que apuntaban a filas borradas pasan a #REF!
// y los rangos que las cruzaban se recortan.
func DeleteRowsFormula(formula, sheet string, at, n int) string {
	return rewriteRefs(formula, func(prefix, ref string) string {
		if prefix != "" && unquoteSheet(strings.TrimSuffix(prefix, "!")) != sheet {
			return prefix + ref
		}
		shifted, ok := DeleteRowsRef(ref, at, n)
		if !ok {
			return "#REF!"
		}
		return prefix + shifted
	})
}

// DeleteRowsRef ajusta una referencia sin hoja ("A5", "$B$3:C9", "4:6") al
// borrar n filas desde at. ok=false si la referencia queda entera dentro de
// las filas borradas.
func DeleteRowsRef(ref string, at, n int) (string, bool) {
	end := at + n - 1
	parts := strings.Split(ref, ":")
	parsed := make([]refPartInfo, len(parts))
	for i, part := range parts {
		info, ok := parseRefPart(part)
		if !ok {
			return ref, true
		}
		parsed[i] = info
	}
	first, last := &parsed[0], &parsed[len(parsed)-1]
	if first.row == 0 || last.row == 0 {
		return ref, true // columnas completas: borrar filas no las cambia
	}
	if first.row >= at && last.row <= end {
		return "", false
	}
	shift := func(r int) int {
		if r > end {
			return r - n
		}
		return r
	}
	r1, r2 := first.row, last.row
	switch {
	case r1 >= at && r1 <= end:
		r1 = at
	default:
		r1 = shift(r1)
	}
	switch {
	case r2 >= at && r2 <= end:
		r2 = at - 1
	default:
		r2 = shift(r2)
	}
	first.row = r1
	last.row = r2
	out := make([]string, len(parsed))
	for i, p := range parsed {
		out[i] = p.String()
	}
	return strings.Join(out, ":"), true
}

func unquoteSheet(s string) string {
	if strings.HasPrefix(s, "'") && strings.HasSuffix(s, "'") && len(s) >= 2 {
		return strings.ReplaceAll(s[1:len(s)-1], "''", "'")
	}
	return s
}

// refPartInfo es un extremo de una referencia: columna y/o fila, con sus "$".
type refPartInfo struct {
	colAbs, col, rowAbs string
	row                 int // 0 si no tiene fila (columna completa)
}

func parseRefPart(part string) (refPartInfo, bool) {
	m := refPart.FindStringSubmatch(part)
	if m == nil || m[2] == "" && m[4] == "" {
		return refPartInfo{}, false
	}
	info := refPartInfo{colAbs: m[1], col: m[2], rowAbs: m[3]}
	if info.col == "" && info.colAbs == "$" {
		// "$5" en filas completas: el "$" es de la fila.
		info.colAbs, info.rowAbs = "", "$"
	}
	if m[4] != "" {
		info.row, _ = strconv.Atoi(m[4])
	}
	return info, true
}

func (p refPartInfo) String() string {
	s := ""
	if p.col != "" {
		s = p.colAbs + p.col
	}
	if p.row > 0 {
		s += p.rowAbs + strconv.Itoa(p.row)
	}
	return s
}

// rewriteRefs aplica fn a cada referencia de la fórmula (prefijo de hoja con
// su "!", y la referencia), sin tocar los textos entre comillas.
func rewriteRefs(formula string, fn func(prefix, ref string) string) string {
	var out strings.Builder
	for i, segment := range splitStrings(formula) {
		if i%2 == 1 {
			out.WriteString(segment)
		} else {
			out.WriteString(rewriteSegment(segment, fn))
		}
	}
	return out.String()
}

// splitStrings separa la fórmula en tramos: los índices impares son textos
// entre comillas dobles (con su escape "").
func splitStrings(formula string) []string {
	var parts []string
	start, inString := 0, false
	for i := 0; i < len(formula); i++ {
		if formula[i] != '"' {
			continue
		}
		if !inString {
			parts = append(parts, formula[start:i])
			start, inString = i, true
		} else if i+1 < len(formula) && formula[i+1] == '"' {
			i++
		} else {
			parts = append(parts, formula[start:i+1])
			start, inString = i+1, false
		}
	}
	return append(parts, formula[start:])
}

func rewriteSegment(s string, fn func(prefix, ref string) string) string {
	var out strings.Builder
	pos := 0
	for pos < len(s) {
		loc := reference.FindStringSubmatchIndex(s[pos:])
		if loc == nil {
			break
		}
		start, end := pos+loc[0], pos+loc[1]
		before := byte(0)
		if start > 0 {
			before = s[start-1]
		}
		after := byte(0)
		if end < len(s) {
			after = s[end]
		}
		valid := !isWordByte(before) && strings.IndexByte(".$'!]", before) < 0 &&
			!isWordByte(after) && after != '(' && after != '!'
		if !valid {
			// Puede haber una referencia válida empezando más adelante.
			out.WriteString(s[pos : start+1])
			pos = start + 1
			continue
		}
		out.WriteString(s[pos:start])
		prefix := ""
		if loc[2] >= 0 {
			prefix = s[pos+loc[2] : pos+loc[3]]
		}
		out.WriteString(fn(prefix, s[pos+loc[4]:pos+loc[5]]))
		pos = end
	}
	out.WriteString(s[pos:])
	return out.String()
}

func translateRef(prefix, ref string, dRows, dCols int) string {
	parts := strings.Split(ref, ":")
	out := make([]string, len(parts))
	for i, part := range parts {
		p, ok := parseRefPart(part)
		if !ok {
			return prefix + ref
		}
		if p.col != "" {
			n, _ := excelize.ColumnNameToNumber(p.col)
			if p.colAbs == "" {
				n += dCols
			}
			if n < 1 {
				return "#REF!"
			}
			p.col, _ = excelize.ColumnNumberToName(n)
		}
		if p.row > 0 {
			if p.rowAbs == "" {
				p.row += dRows
			}
			if p.row < 1 {
				return "#REF!"
			}
		}
		out[i] = p.String()
	}
	return prefix + strings.Join(out, ":")
}
