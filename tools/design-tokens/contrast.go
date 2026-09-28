package main

import (
	"fmt"
	"strings"
)

// CheckContrast verifica en ambos temas los pares declarados y los estados de mesa.
// Un fondo «fill@surface» significa el color fill compuesto sobre surface.
func CheckContrast(t *Tokens) []string {
	var fails []string
	for _, theme := range []string{"light", "dark"} {
		pick := func(v Themed) string {
			if theme == "dark" {
				return v.Dark
			}
			return v.Light
		}
		resolve := func(name string, over RGBA) (RGBA, error) {
			v, ok := t.Color[name]
			if !ok {
				return RGBA{}, fmt.Errorf("color %q no existe", name)
			}
			c, err := ParseColor(pick(v))
			if err != nil {
				return RGBA{}, err
			}
			return c.Over(over), nil
		}
		opaque := func(expr string) (RGBA, error) {
			name, base, layered := strings.Cut(expr, "@")
			if !layered {
				return resolve(name, RGBA{255, 255, 255, 1})
			}
			bg, err := resolve(base, RGBA{255, 255, 255, 1})
			if err != nil {
				return RGBA{}, err
			}
			return resolve(name, bg)
		}
		for _, p := range t.Pairs {
			bg, err := opaque(p.Bg)
			if err != nil {
				fails = append(fails, err.Error())
				continue
			}
			fg, err := resolve(p.Fg, bg)
			if err != nil {
				fails = append(fails, err.Error())
				continue
			}
			if c := Contrast(fg, bg); c < p.Min {
				fails = append(fails, fmt.Sprintf("[%s] %s sobre %s = %.2f (mínimo %.1f)", theme, p.Fg, p.Bg, c, p.Min))
			}
		}
		for _, k := range sortedKeys(t.Mesa) {
			m := t.Mesa[k]
			bg, _ := ParseColor(pick(m.Bg))
			fg, _ := ParseColor(pick(m.Fg))
			if c := Contrast(fg.Over(bg), bg); c < 4.5 {
				fails = append(fails, fmt.Sprintf("[%s] mesa %s: texto sobre fondo = %.2f (mínimo 4.5)", theme, k, c))
			}
		}
	}
	return fails
}
