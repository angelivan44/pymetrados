package metrado

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/xuri/excelize/v2"
)

// Replica paso a paso lo que hacía `scrip_metrados.metados_py` vía COM, usando
// la BD como libro base para conservar estilos y las hojas auxiliares que las
// plantillas P_0 y M_0 usan en sus fórmulas.

const (
	// Columna I: inicio del bloque de elementos en cada hoja M_ y en el Resumen.
	firstElementCol = 9
	// Columna G: formatos que se replican en el bloque de valores del Resumen.
	resumenFormatCol = 7
)

type builder struct {
	f      *excelize.File
	in     *Inputs
	cfg    Config
	maxRow map[string]int      // última fila con contenido de cada plantilla
	shared map[string][]string // celdas con fórmula compartida de cada plantilla
}

func cellName(col, row int) string {
	name, _ := excelize.CoordinatesToCellName(col, row)
	return name
}

// copySheet duplica una plantilla: excelize no copia la configuración de
// página ni los nombres locales (área y títulos de impresión).
func (b *builder) copySheet(template, name string) error {
	from, err := b.f.GetSheetIndex(template)
	if err != nil {
		return err
	}
	to, err := b.f.NewSheet(name)
	if err != nil {
		return err
	}
	if err := b.f.CopySheet(from, to); err != nil {
		return err
	}
	if err := b.expandSharedFormulas(template, name); err != nil {
		return err
	}
	if layout, err := b.f.GetPageLayout(template); err == nil {
		if err := b.f.SetPageLayout(name, &layout); err != nil {
			return err
		}
	}
	for _, dn := range b.f.GetDefinedName() {
		if dn.Scope != template {
			continue
		}
		refersTo := strings.ReplaceAll(dn.RefersTo, QuoteSheet(template)+"!", QuoteSheet(name)+"!")
		refersTo = strings.ReplaceAll(refersTo, "'"+template+"'!", QuoteSheet(name)+"!")
		if err := b.f.SetDefinedName(&excelize.DefinedName{Name: dn.Name, RefersTo: refersTo, Scope: name}); err != nil {
			return err
		}
	}
	return nil
}

// expandSharedFormulas convierte las fórmulas compartidas de la copia en
// fórmulas normales. SetCellFormula sobre una celda compartida solo cambia el
// texto y conserva la estructura compartida, lo que Excel luego rechaza.
func (b *builder) expandSharedFormulas(template, sheet string) error {
	cells, ok := b.shared[template]
	if !ok {
		var err error
		if cells, err = sharedFormulaCells(b.in.BD, template); err != nil {
			return err
		}
		b.shared[template] = cells
	}
	// Primero se leen todas: al vaciar la principal, excelize borra las que dependen de ella.
	formulas := make([]string, len(cells))
	for i, cell := range cells {
		formula, err := b.f.GetCellFormula(sheet, cell)
		if err != nil {
			return err
		}
		formulas[i] = formula
	}
	for i, cell := range cells {
		if err := b.f.SetCellFormula(sheet, cell, ""); err != nil {
			return err
		}
		if formulas[i] == "" {
			continue
		}
		if err := b.f.SetCellFormula(sheet, cell, formulas[i]); err != nil {
			return err
		}
	}
	return nil
}

// setValue escribe un valor como "pegar valores" de Excel: reemplaza también
// la fórmula que hubiera en la celda.
func (b *builder) setValue(sheet string, col, row int, v Value) error {
	cell := cellName(col, row)
	if err := b.f.SetCellFormula(sheet, cell, ""); err != nil {
		return err
	}
	return b.f.SetCellValue(sheet, cell, v)
}

// fillCell copia una celda como AutoFill: traslada fórmulas y copia el formato.
func (b *builder) fillCell(sheet string, srcCol, srcRow, dstCol, dstRow int) error {
	src, dst := cellName(srcCol, srcRow), cellName(dstCol, dstRow)
	formula, err := b.f.GetCellFormula(sheet, src)
	if err != nil {
		return err
	}
	if formula != "" {
		if err := b.f.SetCellFormula(sheet, dst, TranslateFormula(formula, dstRow-srcRow, dstCol-srcCol)); err != nil {
			return err
		}
	} else if err := b.setValue(sheet, dstCol, dstRow, cellValue(b.f, sheet, srcCol, srcRow)); err != nil {
		return err
	}
	return b.copyStyle(sheet, srcCol, srcRow, dstCol, dstRow)
}

func (b *builder) copyStyle(sheet string, srcCol, srcRow, dstCol, dstRow int) error {
	style, err := b.f.GetCellStyle(sheet, cellName(srcCol, srcRow))
	if err != nil {
		return err
	}
	dst := cellName(dstCol, dstRow)
	return b.f.SetCellStyle(sheet, dst, dst, style)
}

// lastCell devuelve la última columna y fila de la hoja contando también las
// celdas que solo tienen formato (como max_row/max_column de la versión
// anterior). La dimensión guardada en el archivo puede estar desactualizada si
// no lo escribió Excel, así que también se mira el contenido.
func (b *builder) lastCell(sheet string) (col, row int) {
	col, row = 1, 1
	if dim, err := b.f.GetSheetDimension(sheet); err == nil && dim != "" {
		parts := strings.Split(dim, ":")
		if c, r, err := excelize.CellNameToCoordinates(parts[len(parts)-1]); err == nil {
			col, row = c, r
		}
	}
	rows, err := b.f.GetRows(sheet, excelize.Options{RawCellValue: true})
	if err != nil {
		return col, row
	}
	row = max(row, len(rows))
	for _, r := range rows {
		col = max(col, len(r))
	}
	return col, row
}

// insertRows inserta n filas antes de `at` y les da el formato de la fila
// superior, como Insert en Excel (excelize ajusta fórmulas, combinadas y
// formatos condicionales, pero deja las filas nuevas sin formato).
func (b *builder) insertRows(sheet string, at, n int) error {
	if n <= 0 {
		return nil
	}
	if err := b.f.InsertRows(sheet, at, n); err != nil {
		return err
	}
	if at <= 1 {
		return nil
	}
	height, err := b.f.GetRowHeight(sheet, at-1)
	if err != nil {
		return err
	}
	// Sin alto propio, GetRowHeight devuelve un valor de respaldo: el mismo que
	// da para una fila que no existe.
	fallback, _ := b.f.GetRowHeight(sheet, excelize.TotalRows)
	customHeight := height != fallback
	lastCol, _ := b.lastCell(sheet)
	for r := at; r < at+n; r++ {
		if customHeight {
			if err := b.f.SetRowHeight(sheet, r, height); err != nil {
				return err
			}
		}
		for c := 1; c <= lastCol; c++ {
			if err := b.copyStyle(sheet, c, at-1, c, r); err != nil {
				return err
			}
		}
	}
	return nil
}

func (b *builder) buildPlanilla(g *Group) (string, error) {
	cfg, p := b.cfg, g.Project
	name := "P_" + p.Key
	if err := b.copySheet(b.in.Sheets.P0, name); err != nil {
		return "", err
	}
	// Excel escribía estas celdas antes de insertar las filas.
	for _, w := range []struct {
		cell  string
		value Value
	}{{cfg.NameCell, p.Name}, {cfg.IDCell, p.Code}, {cfg.SystemCell, p.System}, {cfg.LocationCell, p.Location}} {
		if err := b.f.SetCellValue(name, w.cell, w.value); err != nil {
			return "", err
		}
	}
	if err := b.insertRows(name, cfg.FirstRow+1, len(g.Rows)); err != nil {
		return "", err
	}
	if err := b.f.SetCellValue(name, cfg.LengthCell, len(g.Rows)); err != nil {
		return "", err
	}
	for i, values := range g.Rows {
		for j, v := range values {
			if err := b.setValue(name, cfg.FirstCol+1+j, cfg.FirstRow+i, v); err != nil {
				return "", err
			}
		}
	}
	return name, nil
}

func (b *builder) buildMetrado(g *Group, planilla string) (string, error) {
	cfg, p := b.cfg, g.Project
	name := "M_" + p.Key
	size := len(g.Rows)
	if err := b.copySheet(b.in.Sheets.M0, name); err != nil {
		return "", err
	}
	if err := b.f.SetCellValue(name, fmt.Sprintf("A%d", cfg.FirstRowMontaje), p.ID); err != nil {
		return "", err
	}
	if err := b.f.SetCellValue(name, fmt.Sprintf("C%d", cfg.FirstRowMontaje), p.Name); err != nil {
		return "", err
	}

	// Equivale al TRANSPOSE: cada columna de la planilla queda como fila y cada
	// elemento como columna, desde I1.
	source := QuoteSheet(planilla)
	for i := range b.in.Headers {
		column, _ := excelize.ColumnNumberToName(cfg.FirstCol + 1 + i)
		for j := 0; j <= size; j++ {
			formula := fmt.Sprintf("%s!%s%d", source, column, cfg.FirstRow+j)
			if err := b.f.SetCellFormula(name, cellName(firstElementCol+j, 1+i), formula); err != nil {
				return "", err
			}
		}
	}

	// AutoFill de la columna I hacia la derecha, una columna por elemento.
	for r := cfg.FirstRowMontaje; r <= b.maxRow[b.in.Sheets.M0]; r++ {
		for j := 1; j < size; j++ {
			if err := b.fillCell(name, firstElementCol, r, firstElementCol+j, r); err != nil {
				return "", err
			}
		}
	}
	return name, nil
}

func (b *builder) buildResumen(built map[string]bool) (string, error) {
	cfg := b.cfg
	fmr := cfg.FirstRowMontaje
	const name = "Resumen"
	if err := b.copySheet(b.in.Sheets.M0, name); err != nil {
		return "", err
	}
	n := len(b.in.Projects)
	lastCol := firstElementCol + n
	maxRow := b.maxRow[b.in.Sheets.M0]

	var err error
	for idx, p := range b.in.Projects {
		c := firstElementCol + idx
		if err := b.f.SetCellValue(name, cellName(c, fmr+3), p.Name); err != nil {
			return "", err
		}
		if err := b.f.SetCellValue(name, cellName(c, fmr+4), p.ID); err != nil {
			return "", err
		}
		if built[p.Key] {
			formula := fmt.Sprintf("+%s!%s", QuoteSheet("M_"+p.Key), cfg.FirstValueCell)
			err = b.f.SetCellFormula(name, cellName(c, fmr+5), formula)
		} else {
			// La localidad no tiene filas en la planilla: sin fórmula a una hoja inexistente.
			err = b.setValue(name, c, fmr+5, nil)
		}
		if err != nil {
			return "", err
		}
	}

	for r := fmr + 6; r <= maxRow; r++ {
		for c := firstElementCol; c <= lastCol; c++ {
			if err := b.fillCell(name, c, fmr+5, c, r); err != nil {
				return "", err
			}
		}
	}
	for r := fmr + 5; r <= maxRow; r++ {
		for c := firstElementCol; c <= lastCol; c++ {
			if err := b.copyStyle(name, resumenFormatCol, r, c, r); err != nil {
				return "", err
			}
		}
	}
	for r := fmr + 2; r < fmr+5; r++ {
		for c := firstElementCol; c < lastCol; c++ {
			if err := b.copyStyle(name, firstElementCol, r, c, r); err != nil {
				return "", err
			}
		}
	}

	// La zona superior (filas 1..fmr-2), que en las M_ recibe los datos
	// transpuestos, se quita al final en finalize: con RemoveRow de excelize
	// era la parte más lenta de la generación.
	return name, nil
}

// removeSheets quita las hojas de configuración y los nombres globales que
// apuntaban a ellas.
func (b *builder) removeSheets(names []string) error {
	for _, name := range names {
		if err := b.f.DeleteSheet(name); err != nil {
			return err
		}
	}
	for _, dn := range b.f.GetDefinedName() {
		for _, name := range names {
			if strings.Contains(dn.RefersTo, QuoteSheet(name)+"!") || strings.Contains(dn.RefersTo, "'"+name+"'!") {
				if err := b.f.DeleteDefinedName(&excelize.DefinedName{Name: dn.Name, Scope: dn.Scope}); err != nil {
					return err
				}
				break
			}
		}
	}
	return nil
}

// Generate construye Metrado.xlsx a partir de los datos ya validados.
func Generate(in *Inputs) ([]byte, error) {
	f, err := excelize.OpenReader(bytes.NewReader(in.BD))
	if err != nil {
		return nil, err
	}
	defer f.Close()
	b := &builder{f: f, in: in, cfg: in.Config, maxRow: map[string]int{}, shared: map[string][]string{}}
	_, b.maxRow[in.Sheets.M0] = b.lastCell(in.Sheets.M0)

	// Hojas de la BD que solo sirven para configurar el proceso. El resto
	// (tablas como DATOS o BD_LP) se conserva porque las plantillas las usan
	// mediante fórmulas y nombres definidos.
	removed := []string{in.Sheets.General, in.Sheets.P0, in.Sheets.M0}
	if in.Sheets.Configuracion != "" {
		removed = append(removed, in.Sheets.Configuracion)
	}
	var auxiliary []string
	for _, s := range f.GetSheetList() {
		if !contains(removed, s) {
			auxiliary = append(auxiliary, s)
		}
	}

	// Cada inserción de filas hace que excelize revise las fórmulas de todo el
	// libro, así que las P_ (que insertan filas) se arman antes que las M_, con
	// el libro aún pequeño. Ninguna hoja hace referencia a las M_ antes de que
	// existan, así que el resultado es el mismo. Las filas que el Resumen quita
	// se borran al final, en finalize.
	built := map[string]bool{}
	for _, g := range in.Groups {
		built[g.Project.Key] = true
	}
	resumen, err := b.buildResumen(built)
	if err != nil {
		return nil, fmt.Errorf("hoja Resumen: %w", err)
	}
	planillas := make([]string, len(in.Groups))
	for i, g := range in.Groups {
		if planillas[i], err = b.buildPlanilla(g); err != nil {
			return nil, fmt.Errorf("hoja P_%s: %w", g.Project.Key, err)
		}
	}
	var generated []string
	for i, g := range in.Groups {
		m, err := b.buildMetrado(g, planillas[i])
		if err != nil {
			return nil, fmt.Errorf("hoja M_%s: %w", g.Project.Key, err)
		}
		generated = append(generated, planillas[i], m)
	}
	if err := b.removeSheets(removed); err != nil {
		return nil, err
	}

	if err := moveToFront(f, append([]string{resumen}, generated...)); err != nil {
		return nil, err
	}
	index, _ := f.GetSheetIndex(resumen)
	f.SetActiveSheet(index)
	// Las fórmulas nuevas no tienen resultado guardado: Excel recalcula al abrir.
	fullCalc := true
	if err := f.SetCalcProps(&excelize.CalcPropsOptions{FullCalcOnLoad: &fullCalc}); err != nil {
		return nil, err
	}
	buf, err := f.WriteToBuffer()
	if err != nil {
		return nil, err
	}
	created := map[string]bool{resumen: true}
	for _, s := range generated {
		created[s] = true
	}
	return finalize(buf.Bytes(), in.BD, created, removed, map[string]int{resumen: in.Config.FirstRowMontaje - 2})
}

// moveToFront deja `order` al inicio del libro (el resto, después). MoveSheet de
// excelize no actualiza el índice de hoja de los nombres locales (áreas de
// impresión, etc.), así que se quitan antes y se vuelven a crear después.
func moveToFront(f *excelize.File, order []string) error {
	var local []excelize.DefinedName
	for _, dn := range f.GetDefinedName() {
		if dn.Scope == "" || dn.Scope == "Workbook" {
			continue
		}
		local = append(local, dn)
		if err := f.DeleteDefinedName(&excelize.DefinedName{Name: dn.Name, Scope: dn.Scope}); err != nil {
			return err
		}
	}
	for i := len(order) - 1; i >= 0; i-- {
		before := f.GetSheetList()[0]
		if i < len(order)-1 {
			before = order[i+1]
		}
		if err := f.MoveSheet(order[i], before); err != nil {
			return err
		}
	}
	for _, dn := range local {
		// Algunas BD traen nombres locales heredados que excelize no acepta
		// (p. ej. de vistas personalizadas); no los usa ninguna fórmula.
		_ = f.SetDefinedName(&excelize.DefinedName{Name: dn.Name, RefersTo: dn.RefersTo, Comment: dn.Comment, Scope: dn.Scope})
	}
	return nil
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}
