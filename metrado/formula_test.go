package metrado

import "testing"

func TestTranslateFormula(t *testing.T) {
	cases := []struct {
		formula      string
		dRows, dCols int
		want         string
	}{
		{"IF(AND($H60=I$29),HLOOKUP(I$2,LP_ARMADOS,$B60,FALSE),0)", 0, 2, "IF(AND($H60=K$29),HLOOKUP(K$2,LP_ARMADOS,$B60,FALSE),0)"},
		{"+M_8!G52", 3, 0, "+M_8!G55"},
		{"SUM($I60:$FZ60)", 1, 0, "SUM($I61:$FZ61)"},
		{`IF(A2="B2",LOG10(A2),F_COND)`, 1, 1, `IF(B3="B2",LOG10(B3),F_COND)`},
		{"'P_A 1'!B14+DATOS!J47", 0, 1, "'P_A 1'!C14+DATOS!K47"},
		{"A1-1", -1, 0, "#REF!-1"},
		{"SUM(3:5)+SUM(A:B)", 1, 1, "SUM(4:6)+SUM(B:C)"},
	}
	for _, c := range cases {
		if got := TranslateFormula(c.formula, c.dRows, c.dCols); got != c.want {
			t.Errorf("TranslateFormula(%q, %d, %d) = %q, se esperaba %q", c.formula, c.dRows, c.dCols, got, c.want)
		}
	}
}

func TestQuoteSheet(t *testing.T) {
	for name, want := range map[string]string{"M_8": "M_8", "M_A 1": "'M_A 1'", "AB12": "'AB12'", "O'Neil": "'O''Neil'"} {
		if got := QuoteSheet(name); got != want {
			t.Errorf("QuoteSheet(%q) = %q, se esperaba %q", name, got, want)
		}
	}
}
