package metrado

import (
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"
)

func build(t *testing.T, bd, planilla []byte) *excelize.File {
	t.Helper()
	report, in := Analyze(bd, planilla)
	if !report.OK {
		t.Fatal(report.Errors)
	}
	out, err := Generate(in)
	if err != nil {
		t.Fatal(err)
	}
	return openBook(t, out)
}

func expectCells(t *testing.T, f *excelize.File, sheet string, want map[string]string) {
	t.Helper()
	for cell, value := range want {
		if got := show(t, f, sheet, cell); got != value {
			t.Errorf("%s!%s = %q, se esperaba %q", sheet, cell, got, value)
		}
	}
}

func TestGenerate(t *testing.T) {
	f := build(t, makeBD(t, bdOptions{}), makePlanilla(t, planillaOptions{}))
	defer f.Close()

	t.Run("orden de hojas", func(t *testing.T) {
		want := []string{"Resumen", "P_1", "M_1", "P_2", "M_2"}
		if got := f.GetSheetList(); !reflect.DeepEqual(got, want) {
			t.Fatalf("hojas = %v", got)
		}
		if f.GetActiveSheetIndex() != 0 {
			t.Fatal("el Resumen debe ser la hoja activa")
		}
	})

	t.Run("planilla con filas insertadas", func(t *testing.T) {
		expectCells(t, f, "P_1", map[string]string{
			"C3": "Loc A", "C4": "C-01", "C5": "SER", "C6": "Oxapampa", "C8": "3",
			// Filas 3, 4 y 6 de PLANILLA, pegadas desde B14 (una columna a la derecha de A14).
			"B14": "E1", "C14": "2", "D14": "20",
			"B15": "E2", "C15": "3", "D15": "30",
			"B16": "E4", "C16": "5", "D16": "50",
			// Se insertaron 3 filas en la fila 15: el total baja y su rango se expande.
			"B19": "TOTAL", "C19": "=SUM(C14:C18)",
		})
		if h, _ := f.GetRowHeight("P_1", 19); h != 30 {
			t.Errorf("alto de la fila del total = %v", h)
		}
		merges, _ := f.GetMergeCells("P_1")
		var refs []string
		for _, m := range merges {
			refs = append(refs, m.GetStartAxis()+":"+m.GetEndAxis())
		}
		sort.Strings(refs)
		if !reflect.DeepEqual(refs, []string{"A1:D1", "A20:B20"}) {
			t.Errorf("combinadas = %v", refs)
		}
		// Las filas insertadas heredan el formato de la fila superior.
		above, _ := f.GetCellStyle("P_1", "A14")
		inserted, _ := f.GetCellStyle("P_1", "A15")
		if above == 0 || inserted != above {
			t.Errorf("estilo A15 = %d, A14 = %d", inserted, above)
		}
		cf, _ := f.GetConditionalFormats("P_1")
		if _, ok := cf["C14:C18"]; !ok {
			t.Errorf("formato condicional = %v", cf)
		}
		var area string
		for _, dn := range f.GetDefinedName() {
			if dn.Name == "_xlnm.Print_Area" && dn.Scope == "P_1" {
				area = dn.RefersTo
			}
		}
		if area != "'P_1'!$A$1:$D$20" {
			t.Errorf("área de impresión = %q", area)
		}
	})

	t.Run("metrado transpuesto y extendido", func(t *testing.T) {
		expectCells(t, f, "M_2", map[string]string{
			"A10": "2", "C10": "Loc B",
			"I1": "=P_2!B14", "J3": "=P_2!D15",
			"K1": "=P_2!B16", // size + 1 columnas, como el TRANSPOSE original
			"L1": "", "J12": "=J$2*12", "K12": "",
		})
		i12, _ := f.GetCellStyle("M_2", "I12")
		j12, _ := f.GetCellStyle("M_2", "J12")
		if i12 != j12 {
			t.Errorf("estilo J12 = %d, I12 = %d", j12, i12)
		}
	})

	t.Run("resumen sin la zona superior", func(t *testing.T) {
		expectCells(t, f, "Resumen", map[string]string{
			"I5": "Loc A", "J5": "Loc B", "K5": "Loc C",
			"I6": "1", "J6": "2", "K6": "3",
			// Al borrar filas, excelize reescribe las fórmulas con comillas y sin el
			// "+" inicial; es equivalente a "=+M_1!G10".
			"I7": "='M_1'!G10", "I12": "='M_1'!G15", "J12": "='M_2'!G15",
			"K7":  "", // la localidad 3 no tiene filas
			"G12": "=SUM(I12:Z12)",
		})
		g12, _ := f.GetCellStyle("Resumen", "G12")
		i12, _ := f.GetCellStyle("Resumen", "I12")
		if g12 != i12 {
			t.Errorf("estilo I12 = %d, G12 = %d", i12, g12)
		}
	})
}

func TestComillasEnNombresDeHoja(t *testing.T) {
	f := build(t,
		makeBD(t, bdOptions{general: [][]any{{"A 1", "Loc", "c", "s", "u"}}}),
		makePlanilla(t, planillaOptions{ids: []any{"A 1"}, rows: [][]any{{"x", 1, 2}}}),
	)
	defer f.Close()
	expectCells(t, f, "M_A 1", map[string]string{"I1": "='P_A 1'!B14"})
	expectCells(t, f, "Resumen", map[string]string{"I7": "='M_A 1'!G10"})
}

func TestConservaHojasAuxiliaresYNombres(t *testing.T) {
	f := build(t, makeBD(t, bdOptions{withTables: true}), makePlanilla(t, planillaOptions{}))
	defer f.Close()
	want := []string{"Resumen", "P_1", "M_1", "P_2", "M_2", "DATOS"}
	if got := f.GetSheetList(); !reflect.DeepEqual(got, want) {
		t.Fatalf("hojas = %v", got)
	}
	found := false
	for _, dn := range f.GetDefinedName() {
		found = found || dn.Name == "FACTOR" && dn.RefersTo == "DATOS!$A$1"
	}
	if !found {
		t.Fatalf("nombres = %+v", f.GetDefinedName())
	}
	expectCells(t, f, "M_1", map[string]string{"H10": "=FACTOR*I10"})
}

// Problemas que hacían que Excel pidiera reparar el archivo con la BD real.
func TestSalidaQueExcelAbreSinReparar(t *testing.T) {
	f := openBook(t, makeBD(t, bdOptions{}))
	// Fórmula compartida en la plantilla (como las columnas B y G de M_0 reales).
	shared, ref := excelize.STCellFormulaTypeShared, "B10:B20"
	must(t, f.SetCellFormula("M_0", "B10", "A10&\"x\"", excelize.FormulaOpts{Type: &shared, Ref: &ref}))
	// Nombre heredado con comillas necesarias (excelize las quitaba al mover filas).
	_, _ = f.NewSheet("MI-HOJA")
	must(t, f.SetDefinedName(&excelize.DefinedName{Name: "HEREDADO", RefersTo: "'MI-HOJA'!$A$1:$B$2"}))
	bd := toBytes(t, f)

	report, in := Analyze(bd, makePlanilla(t, planillaOptions{}))
	if !report.OK {
		t.Fatal(report.Errors)
	}
	out, err := Generate(in)
	must(t, err)
	pkg, err := readPackage(out)
	must(t, err)

	if _, ok := pkg.files["xl/calcChain.xml"]; ok {
		t.Error("calcChain.xml debería quitarse")
	}
	_, parts := pkg.sheets()
	for _, sheet := range []string{"Resumen", "M_1", "M_2"} {
		if strings.Contains(string(pkg.files[parts[sheet]]), `t="shared"`) {
			t.Errorf("%s conserva fórmulas compartidas", sheet)
		}
	}
	if workbook := unescapeXML(string(pkg.files["xl/workbook.xml"])); !strings.Contains(workbook, `name="HEREDADO">'MI-HOJA'!$A$1:$B$2<`) {
		t.Errorf("el nombre heredado cambió: %s", definedNameElement.FindAllString(string(pkg.files["xl/workbook.xml"]), -1))
	}
	book := openBook(t, out)
	defer book.Close()
	expectCells(t, book, "M_1", map[string]string{"B10": `=A10&"x"`, "B15": `=A15&"x"`})
}
