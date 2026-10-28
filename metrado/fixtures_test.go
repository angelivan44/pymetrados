package metrado

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/xuri/excelize/v2"
)

const (
	firstRowMontaje = 10
	mLastRow        = 20
)

type bdOptions struct {
	general    [][]any
	config     map[string]any
	withTables bool
}

func toBytes(t *testing.T, f *excelize.File) []byte {
	t.Helper()
	buf, err := f.WriteToBuffer()
	if err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func makeBD(t *testing.T, opts bdOptions) []byte {
	t.Helper()
	f := excelize.NewFile()
	defer f.Close()
	must(t, f.SetSheetName("Sheet1", "General"))
	general := opts.general
	if general == nil {
		general = [][]any{
			{1, "Loc A", "C-01", "SER", "Oxapampa"},
			{2, "Loc B", "C-02", "SER", "Pozuzo"},
			{3, "Loc C", "C-03", "SER", "Villa Rica"},
		}
	}
	must(t, f.SetSheetRow("General", "A1", &[]any{"Id", "Nombre", "Código", "Sistema", "Ubicación"}))
	for i, row := range general {
		r := row
		must(t, f.SetSheetRow("General", fmt.Sprintf("A%d", i+2), &r))
	}

	_, _ = f.NewSheet("Configuracion")
	config := map[string]any{
		"B2": "C3", "B3": "C4", "B4": "C5", "B5": "C6", "B6": "A14",
		"B7": "C8", "B8": firstRowMontaje, "B9": "G10", "B10": "C1",
	}
	for k, v := range opts.config {
		config[k] = v
	}
	for addr, v := range config {
		must(t, f.SetCellValue("Configuracion", addr, v))
	}

	_, _ = f.NewSheet("P_0")
	must(t, f.SetCellValue("P_0", "A1", "PLANILLA DE METRADO"))
	must(t, f.MergeCell("P_0", "A1", "D1"))
	must(t, f.SetSheetRow("P_0", "A13", &[]any{"N°", "Elemento", "Cantidad", "Largo"}))
	yellow, _ := f.NewStyle(&excelize.Style{Fill: excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"FFFF00"}}})
	must(t, f.SetCellStyle("P_0", "A14", "A14", yellow))
	must(t, f.SetCellValue("P_0", "B16", "TOTAL"))
	must(t, f.SetCellFormula("P_0", "C16", "SUM(C14:C15)"))
	must(t, f.MergeCell("P_0", "A17", "B17"))
	must(t, f.SetRowHeight("P_0", 16, 30))
	bold, _ := f.NewConditionalStyle(&excelize.Style{Font: &excelize.Font{Bold: true}})
	must(t, f.SetConditionalFormat("P_0", "C14:C15", []excelize.ConditionalFormatOptions{
		{Type: "formula", Criteria: "C14>C$16", Format: &bold},
	}))
	must(t, f.SetDefinedName(&excelize.DefinedName{Name: "_xlnm.Print_Area", RefersTo: "P_0!$A$1:$D$17", Scope: "P_0"}))

	_, _ = f.NewSheet("M_0")
	boldStyle, _ := f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true}})
	must(t, f.SetCellValue("M_0", "A9", "ITEM"))
	for r := firstRowMontaje; r <= mLastRow; r++ {
		must(t, f.SetCellValue("M_0", fmt.Sprintf("B%d", r), fmt.Sprintf("Partida %d", r)))
		must(t, f.SetCellFormula("M_0", fmt.Sprintf("G%d", r), fmt.Sprintf("SUM(I%d:Z%d)", r, r)))
		must(t, f.SetCellStyle("M_0", fmt.Sprintf("G%d", r), fmt.Sprintf("G%d", r), boldStyle))
		must(t, f.SetCellFormula("M_0", fmt.Sprintf("I%d", r), fmt.Sprintf("I$2*%d", r)))
	}

	if opts.withTables {
		// Tabla auxiliar usada por las plantillas, como DATOS/BD_LP en las BD reales.
		_, _ = f.NewSheet("DATOS")
		must(t, f.MoveSheet("DATOS", "General"))
		must(t, f.SetCellValue("DATOS", "A1", 1.5))
		must(t, f.SetDefinedName(&excelize.DefinedName{Name: "FACTOR", RefersTo: "DATOS!$A$1"}))
		must(t, f.SetCellFormula("M_0", "H10", "FACTOR*I10"))
	}
	return toBytes(t, f)
}

type planillaOptions struct {
	ids     []any
	rows    [][]any
	headers []any
}

func makePlanilla(t *testing.T, opts planillaOptions) []byte {
	t.Helper()
	f := excelize.NewFile()
	defer f.Close()
	ids := opts.ids
	if ids == nil {
		ids = []any{2, 1, 1, 2, 1}
	}
	rows := opts.rows
	if rows == nil {
		for i := range ids {
			rows = append(rows, []any{fmt.Sprintf("E%d", i), i + 1, 10 * (i + 1)})
		}
	}
	headers := opts.headers
	if headers == nil {
		headers = []any{"Elemento", "Cantidad", "Largo"}
	}
	must(t, f.SetSheetName("Sheet1", "DB"))
	must(t, f.SetCellValue("DB", "A1", "Id"))
	for i, id := range ids {
		if id != nil {
			must(t, f.SetCellValue("DB", fmt.Sprintf("A%d", i+2), id))
		}
	}
	_, _ = f.NewSheet("PLANILLA")
	must(t, f.SetSheetRow("PLANILLA", "A1", &headers))
	for i, row := range rows {
		r := row
		must(t, f.SetSheetRow("PLANILLA", fmt.Sprintf("A%d", i+2), &r))
	}
	return toBytes(t, f)
}

func openBook(t *testing.T, data []byte) *excelize.File {
	t.Helper()
	f, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	return f
}

// show devuelve "=fórmula" o el valor de la celda, para comparar fácilmente.
func show(t *testing.T, f *excelize.File, sheet, cell string) string {
	t.Helper()
	formula, err := f.GetCellFormula(sheet, cell)
	if err != nil {
		t.Fatal(err)
	}
	if formula != "" {
		return "=" + formula
	}
	v, err := f.GetCellValue(sheet, cell)
	if err != nil {
		t.Fatal(err)
	}
	return v
}
