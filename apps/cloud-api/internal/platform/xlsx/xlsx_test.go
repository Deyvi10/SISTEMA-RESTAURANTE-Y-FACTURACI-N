package xlsx

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestColumna(t *testing.T) {
	for i, quiero := range map[int]string{0: "A", 25: "Z", 26: "AA", 27: "AB", 701: "ZZ", 702: "AAA"} {
		if got := columna(i); got != quiero {
			t.Errorf("%d: %s", i, got)
		}
	}
}

// Lo lee otra implementación (openpyxl) si está instalada; si no, al menos es un zip válido.
func TestLibroLoLeeOpenpyxl(t *testing.T) {
	b, err := Libro("Ventas <2026>", []string{"Fecha", "Total"}, [][]Celda{{T("2026-09-30 & noche"), N("18.47")}, {T("2026-10-01"), N("-3.00")}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Libro("x", []string{"a"}, [][]Celda{{N("1e5")}}); err == nil {
		t.Fatal("un número raro se rechaza")
	}
	if exec.Command("python3", "-c", "import openpyxl").Run() != nil {
		t.Skip("sin openpyxl")
	}
	f := filepath.Join(t.TempDir(), "v.xlsx")
	_ = os.WriteFile(f, b, 0o600)
	out, err := exec.Command("python3", "-c", `import sys, openpyxl
ws = openpyxl.load_workbook(sys.argv[1]).active
print(ws.title)
for r in ws.iter_rows(values_only=True): print(r)
print(round(sum(r[1] for r in ws.iter_rows(min_row=2, values_only=True)), 2))`, f).CombinedOutput()
	if err != nil {
		t.Fatalf("openpyxl: %s", out)
	}
	s := string(out)
	for _, quiero := range []string{"Ventas <2026>", "('Fecha', 'Total')", "('2026-09-30 & noche', 18.47)", "15.47"} {
		if !strings.Contains(s, quiero) {
			t.Errorf("falta %q en:\n%s", quiero, s)
		}
	}
}
