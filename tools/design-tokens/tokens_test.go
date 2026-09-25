package main

import (
	"math"
	"strings"
	"testing"
)

func load(t *testing.T) *Tokens {
	t.Helper()
	tk, err := Load("../../packages/design/tokens.json", "../../packages/design/icons.json")
	if err != nil {
		t.Fatal(err)
	}
	return tk
}

// RNF-31: todo par de texto declarado cumple WCAG AA en claro y oscuro.
func TestContrastAA(t *testing.T) {
	for _, f := range CheckContrast(load(t)) {
		t.Error(f)
	}
}

func TestContrastMath(t *testing.T) {
	w, _ := ParseColor("#FFFFFF")
	k, _ := ParseColor("#000000")
	if c := Contrast(w, k); math.Abs(c-21) > 0.01 {
		t.Fatalf("blanco/negro = %.2f", c)
	}
	blue, _ := ParseColor("#007AFF")
	if c := Contrast(blue, w); c > 4.1 || c < 3.9 {
		t.Fatalf("#007AFF sobre blanco = %.2f; iOS no cumple AA y por eso existe accent", c)
	}
	half, _ := ParseColor("rgba(0,0,0,0.5)")
	if got := half.Over(w); got.R != 128 {
		t.Fatalf("composición: %+v", got)
	}
	for _, bad := range []string{"#FFF", "rgb(1,2,3)", "rgba(1,2,300,1)", "rgba(1,2,3,2)"} {
		if _, err := ParseColor(bad); err == nil {
			t.Errorf("%q debería rechazarse", bad)
		}
	}
}

func TestGeneratedOutputsAreConsistent(t *testing.T) {
	tk := load(t)
	css := genCSS(tk)
	for _, want := range []string{"--rp-color-accent: #0066CC;", "--rp-color-accent: #409CFF;", `:root[data-theme="dark"]`, "prefers-color-scheme: dark", "--rp-mesa-libre-bg:"} {
		if !strings.Contains(css, want) {
			t.Errorf("CSS sin %q", want)
		}
	}
	// Cada color declarado aparece en claro Y en oscuro en el CSS.
	for k := range tk.Color {
		if n := strings.Count(css, "--rp-color-"+kebab(k)+":"); n != 3 {
			t.Errorf("--rp-color-%s aparece %d veces (esperado: claro + 2 bloques oscuros)", kebab(k), n)
		}
	}
	dart := genDart(tk)
	if !strings.Contains(dart, "static const IconData salon = CupertinoIcons.square_grid_2x2_fill;") || !strings.Contains(dart, "accent: Color(0xFF0066CC)") {
		t.Error("Dart generado incompleto")
	}
	if ts := genTS(tk); !strings.Contains(ts, `cocina: "flame"`) || !strings.Contains(ts, "export type IconName") {
		t.Error("TS generado incompleto")
	}
}

func TestKebabAndOrder(t *testing.T) {
	if kebab("labelSecondary") != "label-secondary" || kebab("largeTitle") != "large-title" {
		t.Fatal("kebab")
	}
	got := sortedKeys(map[string]int{"10": 0, "2": 0, "12": 0, "1": 0})
	if strings.Join(got, ",") != "1,2,10,12" {
		t.Fatalf("orden natural: %v", got)
	}
}
