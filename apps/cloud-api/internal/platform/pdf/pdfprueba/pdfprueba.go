// Package pdfprueba ayuda a probar PDFs con herramientas independientes: poppler (pdftoppm,
// pdftotext) y zxing-cpp. Si no están instaladas, las pruebas lo informan y siguen (el CI las
// instala).
package pdfprueba

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Barras renderiza el PDF y lee los códigos con zxing-cpp (implementación
// independiente). Sin pdftoppm o sin zxing-cpp devuelve nil y la prueba lo informa.
func Barras(t *testing.T, doc []byte) []string {
	t.Helper()
	pdftoppm, err := exec.LookPath("pdftoppm")
	if err != nil || exec.Command("python3", "-c", "import zxingcpp, PIL").Run() != nil {
		t.Log("sin pdftoppm o zxing-cpp: no se decodifica el código de barras")
		return nil
	}
	dir := t.TempDir()
	f := filepath.Join(dir, "d.pdf")
	_ = os.WriteFile(f, doc, 0o600)
	if out, err := exec.Command(pdftoppm, "-r", "300", "-png", f, filepath.Join(dir, "p")).CombinedOutput(); err != nil {
		t.Fatalf("pdftoppm: %s", out)
	}
	imgs, _ := filepath.Glob(filepath.Join(dir, "p*.png"))
	args := append([]string{"-c", `import sys, zxingcpp
from PIL import Image
for f in sys.argv[1:]:
    for r in zxingcpp.read_barcodes(Image.open(f)):
        print(str(r.format) + "|" + r.text)`}, imgs...)
	out, err := exec.Command("python3", args...).CombinedOutput()
	if err != nil {
		t.Fatalf("zxing: %s", out)
	}
	return strings.Split(strings.TrimSpace(string(out)), "\n")
}

// Texto extrae el texto del PDF con pdftotext (orden de lectura). "" si no está instalado.
func Texto(t *testing.T, doc []byte) string {
	t.Helper()
	bin, err := exec.LookPath("pdftotext")
	if err != nil {
		t.Log("sin pdftotext: no se revisa el texto del PDF")
		return ""
	}
	dir := t.TempDir()
	f := filepath.Join(dir, "d.pdf")
	_ = os.WriteFile(f, doc, 0o600)
	out, err := exec.Command(bin, "-layout", f, "-").CombinedOutput()
	if err != nil {
		t.Fatalf("pdftotext rechaza el PDF: %s", out)
	}
	return string(out)
}
