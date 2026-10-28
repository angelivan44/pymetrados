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
	var out strings.Builder
	// Lo que está entre comillas dobles es texto y no se toca.
	for i, segment := range splitStrings(formula) {
		if i%2 == 1 {
			out.WriteString(segment)
		} else {
			out.WriteString(translateSegment(segment, dRows, dCols))
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

func translateSegment(s string, dRows, dCols int) string {
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
		ref := s[pos+loc[4] : pos+loc[5]]
		out.WriteString(translateRef(prefix, ref, dRows, dCols))
		pos = end
	}
	out.WriteString(s[pos:])
	return out.String()
}

func translateRef(prefix, ref string, dRows, dCols int) string {
	parts := strings.Split(ref, ":")
	out := make([]string, len(parts))
	for i, part := range parts {
		m := refPart.FindStringSubmatch(part)
		if m == nil || m[2] == "" && m[4] == "" {
			return prefix + ref
		}
		colAbs, col, rowAbs, row := m[1], m[2], m[3], m[4]
		if col == "" && colAbs == "$" {
			// "$5" en filas completas: el "$" es de la fila.
			colAbs, rowAbs = "", "$"
		}
		var b strings.Builder
		if col != "" {
			n, _ := excelize.ColumnNameToNumber(col)
			if colAbs == "" {
				n += dCols
			}
			if n < 1 {
				return "#REF!"
			}
			name, _ := excelize.ColumnNumberToName(n)
			b.WriteString(colAbs + name)
		}
		if row != "" {
			n, _ := strconv.Atoi(row)
			if rowAbs == "" {
				n += dRows
			}
			if n < 1 {
				return "#REF!"
			}
			b.WriteString(rowAbs + strconv.Itoa(n))
		}
		out[i] = b.String()
	}
	return prefix + strings.Join(out, ":")
}
