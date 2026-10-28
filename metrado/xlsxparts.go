package metrado

// Ajustes a nivel del archivo .xlsx que excelize no cubre bien:
//
//   - Saber qué celdas de una plantilla usan fórmulas compartidas, para
//     convertirlas en fórmulas normales antes de editar la copia (excelize mezcla
//     las fórmulas nuevas con la estructura compartida y Excel lo rechaza).
//   - Al insertar/borrar filas excelize reescribe todos los nombres definidos y
//     rompe los heredados (vínculos externos sin comillas, matrices {…}); al final
//     se restauran tal cual estaban en la BD.
//   - Se quita calcChain.xml: queda apuntando a celdas que ya no tienen fórmula
//     y Excel la reconstruye al abrir.

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"path"
	"regexp"
	"strconv"
	"strings"
)

type xlsxPackage struct {
	files map[string][]byte
	order []string
}

func readPackage(data []byte) (*xlsxPackage, error) {
	r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, err
	}
	p := &xlsxPackage{files: map[string][]byte{}}
	for _, f := range r.File {
		rc, err := f.Open()
		if err != nil {
			return nil, err
		}
		content, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			return nil, err
		}
		p.files[f.Name] = content
		p.order = append(p.order, f.Name)
	}
	return p, nil
}

func (p *xlsxPackage) bytes() ([]byte, error) {
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for _, name := range p.order {
		content, ok := p.files[name]
		if !ok {
			continue
		}
		fw, err := w.Create(name)
		if err != nil {
			return nil, err
		}
		if _, err := fw.Write(content); err != nil {
			return nil, err
		}
	}
	if err := w.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

var (
	sheetElement       = regexp.MustCompile(`<sheet\b[^>]*>`)
	attrPattern        = regexp.MustCompile(`([\w:]+)="([^"]*)"`)
	definedNamesBlock  = regexp.MustCompile(`(?s)<definedNames>.*?</definedNames>|<definedNames/>`)
	definedNameElement = regexp.MustCompile(`(?s)<definedName\b[^>]*>.*?</definedName>`)
	localSheetIDAttr   = regexp.MustCompile(`\s*localSheetId="(\d+)"`)
	relationship       = regexp.MustCompile(`<Relationship\b[^>]*?(?:/>|>\s*</Relationship>)`)
)

func attrs(element string) map[string]string {
	out := map[string]string{}
	for _, m := range attrPattern.FindAllStringSubmatch(element, -1) {
		out[m[1]] = m[2]
	}
	return out
}

func unescapeXML(s string) string {
	return strings.NewReplacer("&amp;", "&", "&lt;", "<", "&gt;", ">", "&quot;", `"`, "&apos;", "'", "&#39;", "'").Replace(s)
}

// sheets devuelve los nombres de hoja en orden y la ruta de cada una dentro del zip.
func (p *xlsxPackage) sheets() ([]string, map[string]string) {
	workbook := string(p.files["xl/workbook.xml"])
	targets := map[string]string{}
	for _, rel := range relationship.FindAllString(string(p.files["xl/_rels/workbook.xml.rels"]), -1) {
		a := attrs(rel)
		target := a["Target"]
		if strings.HasPrefix(target, "/") {
			target = strings.TrimPrefix(target, "/")
		} else {
			target = path.Join("xl", target)
		}
		targets[a["Id"]] = target
	}
	var names []string
	parts := map[string]string{}
	for _, el := range sheetElement.FindAllString(workbook, -1) {
		a := attrs(el)
		name := unescapeXML(a["name"])
		names = append(names, name)
		parts[name] = targets[a["r:id"]]
	}
	return names, parts
}

var sharedFormulaCell = regexp.MustCompile(`<c\b[^>]*\br="([A-Z]+\d+)"[^>]*>\s*<f\b[^>]*\bt="shared"`)

// sharedFormulaCells devuelve las celdas de la hoja que forman parte de una
// fórmula compartida (la principal y las que dependen de ella).
func sharedFormulaCells(data []byte, sheet string) ([]string, error) {
	p, err := readPackage(data)
	if err != nil {
		return nil, err
	}
	_, parts := p.sheets()
	xml, ok := p.files[parts[sheet]]
	if !ok {
		return nil, fmt.Errorf("no se encontró la hoja %s en el archivo", sheet)
	}
	var cells []string
	for _, m := range sharedFormulaCell.FindAllSubmatch(xml, -1) {
		cells = append(cells, string(m[1]))
	}
	return cells, nil
}

// finalize corrige el archivo generado por excelize: restaura los nombres
// definidos de la BD, conserva los de las hojas generadas, borra las filas
// pendientes (hoja → cantidad de filas a quitar desde la 1) y quita calcChain.
func finalize(output, source []byte, generated map[string]bool, removed []string, deleteTop map[string]int) ([]byte, error) {
	out, err := readPackage(output)
	if err != nil {
		return nil, err
	}
	src, err := readPackage(source)
	if err != nil {
		return nil, err
	}
	outSheets, _ := out.sheets()
	srcSheets, _ := src.sheets()
	outIndex := map[string]int{}
	for i, name := range outSheets {
		outIndex[name] = i
	}

	refersToRemoved := func(text string) bool {
		text = unescapeXML(text)
		for _, name := range removed {
			if strings.Contains(text, QuoteSheet(name)+"!") || strings.Contains(text, "'"+name+"'!") {
				return true
			}
		}
		return false
	}
	var names []string
	srcWorkbook := string(src.files["xl/workbook.xml"])
	for _, el := range definedNameElement.FindAllString(srcWorkbook, -1) {
		open := el[:strings.Index(el, ">")+1]
		body := el[len(open) : len(el)-len("</definedName>")]
		if refersToRemoved(body) {
			continue
		}
		if m := localSheetIDAttr.FindStringSubmatch(open); m != nil {
			i, _ := strconv.Atoi(m[1])
			if i >= len(srcSheets) {
				continue
			}
			target, ok := outIndex[srcSheets[i]]
			if !ok {
				continue // era local de una hoja de configuración que ya no existe
			}
			open = localSheetIDAttr.ReplaceAllString(open, fmt.Sprintf(` localSheetId="%d"`, target))
		}
		names = append(names, open+body+"</definedName>")
	}
	outWorkbook := string(out.files["xl/workbook.xml"])
	for _, el := range definedNameElement.FindAllString(outWorkbook, -1) {
		m := localSheetIDAttr.FindStringSubmatch(el)
		if m == nil {
			continue
		}
		i, _ := strconv.Atoi(m[1])
		if i < len(outSheets) && generated[outSheets[i]] {
			names = append(names, el)
		}
	}
	block := ""
	if len(names) > 0 {
		block = "<definedNames>" + strings.Join(names, "") + "</definedNames>"
	}
	if definedNamesBlock.MatchString(outWorkbook) {
		outWorkbook = definedNamesBlock.ReplaceAllLiteralString(outWorkbook, block)
	} else {
		outWorkbook = strings.Replace(outWorkbook, "</sheets>", "</sheets>"+block, 1)
	}
	out.files["xl/workbook.xml"] = []byte(outWorkbook)

	delete(out.files, "xl/calcChain.xml")
	rels := string(out.files["xl/_rels/workbook.xml.rels"])
	for _, rel := range relationship.FindAllString(rels, -1) {
		if strings.Contains(rel, "calcChain") {
			rels = strings.Replace(rels, rel, "", 1)
		}
	}
	out.files["xl/_rels/workbook.xml.rels"] = []byte(rels)
	types := regexp.MustCompile(`<Override\b[^>]*PartName="/xl/calcChain.xml"[^>]*?(?:/>|>\s*</Override>)`)
	out.files["[Content_Types].xml"] = types.ReplaceAll(out.files["[Content_Types].xml"], nil)

	for sheet, n := range deleteTop {
		if err := out.deleteRows(sheet, 1, n); err != nil {
			return nil, err
		}
	}
	return out.bytes()
}

var (
	rowElement       = regexp.MustCompile(`(?s)<row\b[^>]*?(?:/>|>.*?</row>)`)
	rowNumber        = regexp.MustCompile(`^<row\b[^>]*?\br="(\d+)"`)
	cellRefAttr      = regexp.MustCompile(`(<c\b[^>]*?\br=")([A-Z]+)(\d+)(")`)
	formulaElement   = regexp.MustCompile(`(?s)(<f\b[^>]*>)(.*?)(</f>)`)
	refAttr          = regexp.MustCompile(`(\bref=")([^"]*)(")`)
	sqrefAttr        = regexp.MustCompile(`(\bsqref=")([^"]*)(")`)
	mergeCellElement = regexp.MustCompile(`<mergeCell\b[^>]*?(?:/>|>\s*</mergeCell>)`)
	cfBlock          = regexp.MustCompile(`(?s)<conditionalFormatting\b[^>]*>.*?</conditionalFormatting>`)
	dvElement        = regexp.MustCompile(`(?s)<dataValidation\b[^>]*?(?:/>|>.*?</dataValidation>)`)
	cfFormula        = regexp.MustCompile(`(?s)(<(formula|formula1|formula2)>)(.*?)(</(?:formula|formula1|formula2)>)`)
	activeCellAttr   = regexp.MustCompile(`(\b(?:activeCell|topLeftCell)=")([^"]*)(")`)
	breakElement     = regexp.MustCompile(`<brk\b[^>]*?\bid="(\d+)"[^>]*?(?:/>|>\s*</brk>)`)
	breakCountAttrs  = regexp.MustCompile(`(<rowBreaks\b[^>]*?\b(?:count|manualBreakCount)=")\d+(")`)
	emptyRowBreaks   = regexp.MustCompile(`<rowBreaks\b[^>]*?(?:/>|>\s*</rowBreaks>)`)
	mergeCountAttr   = regexp.MustCompile(`(<mergeCells\b[^>]*\bcount=")\d+(")`)
	emptyMergeCells  = regexp.MustCompile(`<mergeCells\b[^>]*?(?:/>|>\s*</mergeCells>)`)
)

func escapeXMLText(s string) string {
	return strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;").Replace(s)
}

// shiftList ajusta una lista de rangos separados por espacios ("A1:B2 C5");
// devuelve "" si todos quedaron dentro de las filas borradas.
func shiftList(list string, at, n int) string {
	var out []string
	for _, ref := range strings.Fields(list) {
		if shifted, ok := DeleteRowsRef(ref, at, n); ok {
			out = append(out, shifted)
		}
	}
	return strings.Join(out, " ")
}

// deleteRows borra n filas desde `at` en la hoja `sheet` del paquete, como
// Excel: desplaza filas, celdas, fórmulas que apuntan a la propia hoja,
// combinadas, formatos condicionales, validaciones, selección, saltos de
// página y los nombres definidos que apuntan a la hoja. Se hace aquí y no con
// RemoveRow de excelize porque éste revisa todas las fórmulas del libro en
// cada fila: con el Resumen (45 filas) era ~40% del tiempo de generación.
//
// Supone que ninguna otra hoja tiene fórmulas que apunten a `sheet` (es el
// caso del Resumen, que se crea en la generación).
func (p *xlsxPackage) deleteRows(sheet string, at, n int) error {
	if n <= 0 {
		return nil
	}
	_, parts := p.sheets()
	part, ok := parts[sheet]
	if !ok {
		return fmt.Errorf("no se encontró la hoja %s", sheet)
	}
	x := string(p.files[part])
	end := at + n - 1
	formula := func(text string) string {
		return escapeXMLText(DeleteRowsFormula(unescapeXML(text), sheet, at, n))
	}
	shiftFormulas := func(s string) string {
		return formulaElement.ReplaceAllStringFunc(s, func(f string) string {
			m := formulaElement.FindStringSubmatch(f)
			open := refAttr.ReplaceAllStringFunc(m[1], func(a string) string {
				r := refAttr.FindStringSubmatch(a)
				return r[1] + shiftList(r[2], at, n) + r[3]
			})
			return open + formula(m[2]) + m[3]
		})
	}

	x = rowElement.ReplaceAllStringFunc(x, func(row string) string {
		m := rowNumber.FindStringSubmatch(row)
		if m == nil {
			return row
		}
		r, _ := strconv.Atoi(m[1])
		if r >= at && r <= end {
			return ""
		}
		if r > end {
			row = strings.Replace(row, fmt.Sprintf(`r="%d"`, r), fmt.Sprintf(`r="%d"`, r-n), 1)
			row = cellRefAttr.ReplaceAllStringFunc(row, func(c string) string {
				cm := cellRefAttr.FindStringSubmatch(c)
				cr, _ := strconv.Atoi(cm[3])
				return cm[1] + cm[2] + strconv.Itoa(cr-n) + cm[4]
			})
		}
		return shiftFormulas(row)
	})

	x = mergeCellElement.ReplaceAllStringFunc(x, func(el string) string {
		r := refAttr.FindStringSubmatch(el)
		if r == nil {
			return el
		}
		shifted, ok := DeleteRowsRef(r[2], at, n)
		if !ok || !strings.Contains(shifted, ":") {
			return ""
		}
		return strings.Replace(el, r[0], r[1]+shifted+r[3], 1)
	})
	if merges := len(mergeCellElement.FindAllString(x, -1)); merges > 0 {
		x = mergeCountAttr.ReplaceAllString(x, fmt.Sprintf("${1}%d${2}", merges))
	} else {
		x = emptyMergeCells.ReplaceAllString(x, "")
	}

	shiftBlock := func(block string) string {
		sq := sqrefAttr.FindStringSubmatch(block)
		if sq == nil {
			return block
		}
		list := shiftList(sq[2], at, n)
		if list == "" {
			return ""
		}
		block = strings.Replace(block, sq[0], sq[1]+list+sq[3], 1)
		return cfFormula.ReplaceAllStringFunc(block, func(f string) string {
			m := cfFormula.FindStringSubmatch(f)
			return m[1] + formula(m[3]) + m[4]
		})
	}
	x = cfBlock.ReplaceAllStringFunc(x, shiftBlock)
	x = dvElement.ReplaceAllStringFunc(x, shiftBlock)

	// Selección y panel: si apuntaban a filas borradas, a la primera celda.
	x = activeCellAttr.ReplaceAllStringFunc(x, func(a string) string {
		m := activeCellAttr.FindStringSubmatch(a)
		shifted, ok := DeleteRowsRef(m[2], at, n)
		if !ok {
			shifted = "A1"
		}
		return m[1] + shifted + m[3]
	})
	x = regexp.MustCompile(`<selection\b[^>]*>`).ReplaceAllStringFunc(x, func(el string) string {
		sq := sqrefAttr.FindStringSubmatch(el)
		if sq == nil {
			return el
		}
		list := shiftList(sq[2], at, n)
		if list == "" {
			list = "A1"
		}
		return strings.Replace(el, sq[0], sq[1]+list+sq[3], 1)
	})
	x = regexp.MustCompile(`<(dimension|hyperlink|autoFilter)\b[^>]*>`).ReplaceAllStringFunc(x, func(el string) string {
		r := refAttr.FindStringSubmatch(el)
		if r == nil {
			return el
		}
		shifted, ok := DeleteRowsRef(r[2], at, n)
		if !ok {
			shifted = "A1"
		}
		return strings.Replace(el, r[0], r[1]+shifted+r[3], 1)
	})
	x = breakElement.ReplaceAllStringFunc(x, func(el string) string {
		m := breakElement.FindStringSubmatch(el)
		id, _ := strconv.Atoi(m[1])
		switch {
		case id >= at && id <= end:
			return ""
		case id > end:
			return strings.Replace(el, fmt.Sprintf(`id="%d"`, id), fmt.Sprintf(`id="%d"`, id-n), 1)
		}
		return el
	})
	if breaks := len(breakElement.FindAllString(x, -1)); breaks > 0 {
		x = breakCountAttrs.ReplaceAllString(x, fmt.Sprintf("${1}%d${2}", breaks))
	} else {
		x = emptyRowBreaks.ReplaceAllString(x, "")
	}
	p.files[part] = []byte(x)

	// Nombres definidos (área y títulos de impresión): solo las referencias
	// que nombran la hoja.
	workbook := string(p.files["xl/workbook.xml"])
	workbook = definedNameElement.ReplaceAllStringFunc(workbook, func(el string) string {
		open := el[:strings.Index(el, ">")+1]
		body := unescapeXML(el[len(open) : len(el)-len("</definedName>")])
		body = rewriteRefs(body, func(prefix, ref string) string {
			if prefix == "" || unquoteSheet(strings.TrimSuffix(prefix, "!")) != sheet {
				return prefix + ref
			}
			if shifted, ok := DeleteRowsRef(ref, at, n); ok {
				return prefix + shifted
			}
			return prefix + "#REF!"
		})
		return open + escapeXMLText(body) + "</definedName>"
	})
	p.files["xl/workbook.xml"] = []byte(workbook)
	return nil
}
