// Command design-tokens genera los tokens del sistema de diseño (F0-12) para web y Flutter
// desde packages/design/{tokens,icons}.json, y verifica el contraste WCAG AA.
//
//	go run ./tools/design-tokens          # escribe los archivos generados
//	go run ./tools/design-tokens -check   # falla si están desactualizados (CI)
package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

func main() {
	root := flag.String("root", ".", "raíz del monorepo")
	check := flag.Bool("check", false, "no escribe: falla si los archivos generados no coinciden")
	flag.Parse()
	if err := run(*root, *check); err != nil {
		fmt.Fprintln(os.Stderr, "✗", err)
		os.Exit(1)
	}
}

func outputs(t *Tokens) map[string]string {
	return map[string]string{
		"packages/ts/ui/src/tokens.css":                  genCSS(t),
		"packages/ts/ui/src/tailwind.css":                genTailwind(t),
		"packages/ts/ui/src/tokens.ts":                   genTS(t),
		"packages/dart/restpos_ui/lib/src/tokens.g.dart": genDart(t),
	}
}

func run(root string, check bool) error {
	t, err := Load(filepath.Join(root, "packages/design/tokens.json"), filepath.Join(root, "packages/design/icons.json"))
	if err != nil {
		return err
	}
	if fails := CheckContrast(t); len(fails) > 0 {
		for _, f := range fails {
			fmt.Fprintln(os.Stderr, "  contraste insuficiente:", f)
		}
		return fmt.Errorf("%d combinaciones no cumplen WCAG AA", len(fails))
	}
	stale := 0
	for rel, content := range outputs(t) {
		path := filepath.Join(root, rel)
		current, _ := os.ReadFile(path) //nolint:gosec // ruta fija del repo
		if bytes.Equal(current, []byte(content)) {
			continue
		}
		if check {
			fmt.Fprintln(os.Stderr, "  desactualizado:", rel)
			stale++
			continue
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			return err
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			return err
		}
		fmt.Println("  escrito:", rel)
	}
	if stale > 0 {
		return fmt.Errorf("%d archivos generados no coinciden con packages/design: ejecuta make tokens", stale)
	}
	fmt.Println("✓ tokens al día y contraste AA verificado en claro y oscuro")
	return nil
}
