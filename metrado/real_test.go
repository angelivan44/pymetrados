package metrado

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestArchivosReales corre con los ejemplos de la release v1.0 (DM_LP.xlsx y
// Planilla LP.xlsx) si METRADO_REAL_DIR apunta a la carpeta que los contiene.
// Deja el resultado en $METRADO_REAL_DIR/Metrado_go.xlsx para revisarlo.
func TestArchivosReales(t *testing.T) {
	dir := os.Getenv("METRADO_REAL_DIR")
	if dir == "" {
		t.Skip("define METRADO_REAL_DIR para probar con archivos reales")
	}
	bd, err := os.ReadFile(filepath.Join(dir, "DM_LP.xlsx"))
	must(t, err)
	planilla, err := os.ReadFile(filepath.Join(dir, "Planilla LP.xlsx"))
	must(t, err)

	start := time.Now()
	report, in := Analyze(bd, planilla)
	t.Logf("validación: %v", time.Since(start))
	if !report.OK {
		t.Fatal(report.Errors)
	}
	if s := report.Summary; s.Localidades != 6 || s.Filas != 122 || s.Columnas != 33 {
		t.Fatalf("resumen = %+v", s)
	}
	if len(report.Warnings) != 1 {
		t.Fatalf("advertencias = %v", messages(report.Warnings))
	}

	start = time.Now()
	out, err := Generate(in)
	must(t, err)
	t.Logf("generación: %v", time.Since(start))
	must(t, os.WriteFile(filepath.Join(dir, "Metrado_go.xlsx"), out, 0o644))

	f := openBook(t, out)
	defer f.Close()
	want := []string{"Resumen", "P_8", "M_8", "P_9", "M_9", "P_10", "M_10", "P_11", "M_11", "P_12", "M_12", "P_15", "M_15", "DATOS", "BD_LP", "Heaths"}
	if got := f.GetSheetList(); len(got) != len(want) {
		t.Fatalf("hojas = %v", got)
	}
}
