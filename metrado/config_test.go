package metrado

import (
	"reflect"
	"testing"
)

// Lo que en la hoja Configuracion de makeBD es A14, C3… expresado como en la página.
var configPagina = map[string]string{
	"nombre": "C3", "codigo": "C4", "sistema": "C5", "ubicacion": "C6",
	"primeraFila": "B14", "cantidad": "C8", "filaMetrado": "10", "celdaResumen": "G10", "ultimaColumna": "C",
}

func origins(fields []CampoConfig) map[string]string {
	out := map[string]string{}
	for _, f := range fields {
		out[f.Clave] = f.Origen + ":" + f.Valor
	}
	return out
}

func TestSinConfiguracionDetectaYPideElResto(t *testing.T) {
	report, in := Analyze(makeBD(t, bdOptions{sinConfig: true}), makePlanilla(t, planillaOptions{}))
	if in != nil || report.OK {
		t.Fatal("no debería poder generarse sin elegir las celdas")
	}
	got := origins(report.Config)
	if got["cantidad"] != "detectado:C8" || got["filaMetrado"] != "detectado:7" || got["ultimaColumna"] != "detectado:C" || got["nombre"] != ":" {
		t.Fatalf("config = %v", got)
	}
	errs := messages(report.Errors)
	if len(errs) != 1 || !anyContains(errs, "«Nombre de la localidad»", "«Celda que va al Resumen»") {
		t.Fatalf("errores: %v", errs)
	}
	if report.Vistas["P_0"] == nil || report.Vistas["M_0"] == nil || report.Vistas["PLANILLA"] == nil || report.Firma == "" {
		t.Fatalf("faltan vistas o firma: %v %q", report.Vistas, report.Firma)
	}
	if report.Vistas["P_0"].Celdas["A1"] != "PLANILLA DE METRADO" {
		t.Fatalf("vista P_0: %v", report.Vistas["P_0"].Celdas)
	}
}

func TestConfiguracionElegidaEnLaPaginaEquivaleALaHoja(t *testing.T) {
	planilla := makePlanilla(t, planillaOptions{})
	_, conHoja := Analyze(makeBD(t, bdOptions{}), planilla)
	report, in := AnalyzeWith(makeBD(t, bdOptions{sinConfig: true}), planilla, Opciones{Configuracion: configPagina})
	if !report.OK {
		t.Fatal(report.Errors)
	}
	if !reflect.DeepEqual(in.Config, conHoja.Config) {
		t.Fatalf("config = %+v, se esperaba %+v", in.Config, conHoja.Config)
	}
	if _, err := Generate(in); err != nil {
		t.Fatal(err)
	}
}

func TestPaginaMandaSobreLaHoja(t *testing.T) {
	report, in := AnalyzeWith(makeBD(t, bdOptions{config: map[string]any{"B6": "nada"}}), makePlanilla(t, planillaOptions{}),
		Opciones{Configuracion: map[string]string{"primeraFila": "B14", "nombre": "D3"}})
	if !report.OK {
		t.Fatal(report.Errors)
	}
	if in.Config.FirstRowCell != "A14" || in.Config.NameCell != "D3" {
		t.Fatalf("config = %+v", in.Config)
	}
	if got := origins(report.Config); got["nombre"] != "usuario:D3" || got["codigo"] != "configuracion:C4" {
		t.Fatalf("orígenes = %v", got)
	}
}

func TestValorInvalidoDesdeLaPagina(t *testing.T) {
	report, _ := AnalyzeWith(makeBD(t, bdOptions{}), makePlanilla(t, planillaOptions{}),
		Opciones{Configuracion: map[string]string{"primeraFila": "A14"}})
	if report.OK || !anyContains(messages(report.Errors), "«Primera celda de datos»") {
		t.Fatalf("errores: %v", messages(report.Errors))
	}
}

func TestAvisaSiLaHojaContradiceLasFormulas(t *testing.T) {
	bd := openBook(t, makeBD(t, bdOptions{}))
	must(t, bd.SetCellFormula("P_0", "D16", "SUM(OFFSET($D$15,0,0,$C$8))"))
	report, _ := Analyze(toBytes(t, bd), makePlanilla(t, planillaOptions{}))
	if !anyContains(messages(report.Warnings), "Configuracion!B6", "fila 14", "fila 15") {
		t.Fatalf("advertencias: %v", messages(report.Warnings))
	}
}

func TestSoloUnArchivo(t *testing.T) {
	report, in := Analyze(nil, makePlanilla(t, planillaOptions{}))
	if in != nil || report.OK || len(report.Errors) != 0 {
		t.Fatalf("ok=%v errores=%v", report.OK, messages(report.Errors))
	}
	var texts []string
	for _, c := range report.Checks {
		texts = append(texts, c.Archivo+": "+c.Texto)
	}
	if !anyContains(texts, "planilla: DB: 5 filas con ID", "2 localidades") {
		t.Fatalf("checks: %v", texts)
	}

	report, _ = Analyze(makeBD(t, bdOptions{}), nil)
	if len(report.Errors) != 0 || len(report.Config) != len(campos) {
		t.Fatalf("errores=%v config=%d", messages(report.Errors), len(report.Config))
	}
}

func TestEscribeLaConfiguracionEnLaBD(t *testing.T) {
	bd := makeBD(t, bdOptions{sinConfig: true, withTables: true})
	planilla := makePlanilla(t, planillaOptions{})
	report, _ := AnalyzeWith(bd, planilla, Opciones{Configuracion: configPagina})
	out, err := WriteConfig(bd, report.Config)
	must(t, err)

	report, in := Analyze(out, planilla)
	if !report.OK {
		t.Fatal(report.Errors)
	}
	for k, v := range origins(report.Config) {
		if v != "configuracion:"+configPagina[k] {
			t.Fatalf("%s = %s", k, v)
		}
	}
	f := openBook(t, out)
	defer f.Close()
	if got := show(t, f, "Configuracion", "B6"); got != "A14" {
		t.Fatalf("Configuracion!B6 = %s", got)
	}
	if _, err := Generate(in); err != nil {
		t.Fatal(err)
	}
}
