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
// definidos de la BD, conserva los de las hojas generadas y quita calcChain.
func finalize(output, source []byte, generated map[string]bool, removed []string) ([]byte, error) {
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

	return out.bytes()
}
