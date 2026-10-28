package metrado

import (
	"reflect"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"
)

func messages(issues []Issue) []string {
	var out []string
	for _, i := range issues {
		out = append(out, i.Message)
	}
	return out
}

func anyContains(list []string, parts ...string) bool {
	for _, s := range list {
		ok := true
		for _, p := range parts {
			ok = ok && strings.Contains(s, p)
		}
		if ok {
			return true
		}
	}
	return false
}

func TestAgrupaFilasPorID(t *testing.T) {
	report, in := Analyze(makeBD(t, bdOptions{}), makePlanilla(t, planillaOptions{}))
	if !report.OK {
		t.Fatal(report.Errors)
	}
	var got [][]any
	for _, g := range in.Groups {
		got = append(got, []any{g.Project.Key, g.SourceRows})
	}
	want := [][]any{{"1", []int{3, 4, 6}}, {"2", []int{2, 5}}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("grupos = %v, se esperaba %v", got, want)
	}
	if report.Summary.Filas != 5 {
		t.Fatalf("filas = %d", report.Summary.Filas)
	}
	warnings := strings.Join(messages(report.Warnings), " ")
	if !strings.Contains(warnings, "agrupadas") || !strings.Contains(warnings, "Resumen sin valores") {
		t.Fatalf("advertencias: %s", warnings)
	}
}

func TestIDsNumericosEquivalentes(t *testing.T) {
	report, in := Analyze(makeBD(t, bdOptions{}), makePlanilla(t, planillaOptions{
		ids: []any{1.0, "2", " 2 "}, rows: [][]any{{"a"}, {"b"}, {"c"}},
	}))
	if !report.OK {
		t.Fatal(report.Errors)
	}
	if len(in.Groups) != 2 || len(in.Groups[0].Rows) != 1 || len(in.Groups[1].Rows) != 2 {
		t.Fatalf("grupos inesperados: %+v", in.Groups)
	}
}

func TestIDDesconocido(t *testing.T) {
	report, _ := Analyze(makeBD(t, bdOptions{}), makePlanilla(t, planillaOptions{
		ids: []any{1, 12, 12}, rows: [][]any{{"a"}, {"b"}, {"c"}},
	}))
	errs := messages(report.Errors)
	if report.OK || len(errs) != 1 || !anyContains(errs, "'12'", "filas 3, 4") {
		t.Fatalf("errores: %v", errs)
	}
}

func TestFilaSinIDEntreFilasConID(t *testing.T) {
	report, _ := Analyze(makeBD(t, bdOptions{}), makePlanilla(t, planillaOptions{
		ids: []any{1, nil, 2}, rows: [][]any{{"a"}, {"b"}, {"c"}},
	}))
	if !anyContains(messages(report.Errors), "sin ID", "filas 3)") {
		t.Fatalf("errores: %v", messages(report.Errors))
	}
}

func TestIgnoraFilasDeRellenoAlFinal(t *testing.T) {
	report, in := Analyze(makeBD(t, bdOptions{}), makePlanilla(t, planillaOptions{
		ids: []any{1, 2, nil, nil}, rows: [][]any{{"a"}, {"b"}, {"MAD", 95}, {0, 0}},
	}))
	if !report.OK {
		t.Fatal(report.Errors)
	}
	if len(in.Groups) != 2 {
		t.Fatalf("grupos: %d", len(in.Groups))
	}
}

func TestConfiguracionInvalidaIndicaCelda(t *testing.T) {
	report, in := Analyze(makeBD(t, bdOptions{config: map[string]any{"B6": nil, "B8": "veinte"}}), makePlanilla(t, planillaOptions{}))
	if in != nil {
		t.Fatal("no debería poder generarse")
	}
	var where []string
	for _, e := range report.Errors {
		where = append(where, e.Where)
	}
	want := []string{"BD › Configuracion!B6", "BD › Configuracion!B8"}
	if !reflect.DeepEqual(where, want) {
		t.Fatalf("where = %v", where)
	}
}

func TestFaltaHoja(t *testing.T) {
	planilla := makePlanilla(t, planillaOptions{})
	report, in := Analyze(planilla, planilla)
	if in != nil || !anyContains(messages(report.Errors), "Falta la hoja 'General'") {
		t.Fatalf("errores: %v", messages(report.Errors))
	}
}

func TestAceptaHojaConTilde(t *testing.T) {
	f := openBook(t, makeBD(t, bdOptions{}))
	must(t, f.SetSheetName("Configuracion", "Configuración"))
	report, in := Analyze(toBytes(t, f), makePlanilla(t, planillaOptions{}))
	if !report.OK || in.Sheets.Configuracion != "Configuración" {
		t.Fatalf("ok=%v hojas=%+v", report.OK, in)
	}
}

func TestRechazaXLS(t *testing.T) {
	xls := append([]byte{0xd0, 0xcf, 0x11, 0xe0}, make([]byte, 100)...)
	report, _ := Analyze(xls, makePlanilla(t, planillaOptions{}))
	if !anyContains(messages(report.Errors), ".xls") {
		t.Fatalf("errores: %v", messages(report.Errors))
	}
}

func TestIDsRepetidosEnGeneral(t *testing.T) {
	general := [][]any{{1, "A", "", "", ""}, {1.0, "B", "", "", ""}}
	report, _ := Analyze(makeBD(t, bdOptions{general: general}), makePlanilla(t, planillaOptions{ids: []any{1}, rows: [][]any{{"a"}}}))
	if !anyContains(messages(report.Errors), "repetido") {
		t.Fatalf("errores: %v", messages(report.Errors))
	}
}

func TestM0SinFilasSuficientes(t *testing.T) {
	report, _ := Analyze(makeBD(t, bdOptions{config: map[string]any{"B8": 4}}), makePlanilla(t, planillaOptions{}))
	if !anyContains(messages(report.Errors), "solo reserva") {
		t.Fatalf("errores: %v", messages(report.Errors))
	}
}

func TestLeeResultadoDeFormulasIncluidoCero(t *testing.T) {
	// Excel guarda el resultado de cada fórmula; aquí se simula escribiendo el XML.
	f := excelize.NewFile()
	must(t, f.SetSheetName("Sheet1", "DB"))
	must(t, f.SetCellValue("DB", "A1", "Id"))
	must(t, f.SetCellValue("DB", "A2", 1))
	_, _ = f.NewSheet("PLANILLA")
	must(t, f.SetCellValue("PLANILLA", "A1", "Cantidad"))
	must(t, f.SetCellValue("PLANILLA", "A2", 0))
	must(t, f.SetCellFormula("PLANILLA", "A2", "DB!A2-1"))
	data := toBytes(t, f)
	report, in := Analyze(makeBD(t, bdOptions{}), data)
	if !report.OK {
		t.Fatal(report.Errors)
	}
	// Lo importante es que el 0 no se pierda (ExcelJS lo descartaba).
	if v := in.Groups[0].Rows[0][0]; valueText(v) != "0" {
		t.Fatalf("valor = %#v", v)
	}
}
