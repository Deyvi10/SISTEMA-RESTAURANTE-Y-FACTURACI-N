package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
)

// Themed es un valor con variante clara y oscura.
type Themed struct {
	Light string `json:"light"`
	Dark  string `json:"dark"`
	Uso   string `json:"uso,omitempty"`
}

type MesaState struct {
	Bg    Themed `json:"bg"`
	Fg    Themed `json:"fg"`
	Icono string `json:"icono"`
}

type TextStyle struct {
	Size     float64 `json:"size"`
	Line     float64 `json:"line"`
	Weight   int     `json:"weight"`
	Tracking float64 `json:"tracking"`
	Uso      string  `json:"uso,omitempty"`
}

type Icon struct {
	Lucide    string `json:"lucide"`
	SF        string `json:"sf"`
	Cupertino string `json:"cupertino"`
	Tint      string `json:"tint"`
}

// Tokens es tokens.json + icons.json ya interpretados.
type Tokens struct {
	Color    map[string]Themed
	Tint     map[string]Themed
	Mesa     map[string]MesaState
	Pairs    []Pair
	Fonts    map[string]string
	Scale    map[string]TextStyle
	Space    map[string]int
	Radius   map[string]int
	Size     map[string]int
	Motion   map[string]json.RawMessage
	Blur     int
	Saturate float64
	Shadow   map[string]string
	Icons    map[string]Icon
}

// Pair es un par texto/fondo con su contraste mínimo.
type Pair struct {
	Fg, Bg string
	Min    float64
}

// Load lee y valida los dos archivos fuente.
func Load(tokensPath, iconsPath string) (*Tokens, error) {
	var raw struct {
		Color     map[string]json.RawMessage `json:"color"`
		Tint      map[string]json.RawMessage `json:"tint"`
		Mesa      map[string]json.RawMessage `json:"mesa"`
		Contraste struct {
			Pares [][3]json.RawMessage `json:"pares"`
		} `json:"contraste"`
		Font struct {
			Family map[string]string          `json:"family"`
			Scale  map[string]json.RawMessage `json:"scale"`
		} `json:"font"`
		Space    map[string]int             `json:"space"`
		Radius   map[string]int             `json:"radius"`
		Size     map[string]int             `json:"size"`
		Motion   map[string]json.RawMessage `json:"motion"`
		Material struct {
			Blur     int     `json:"blur"`
			Saturate float64 `json:"saturate"`
		} `json:"material"`
		Shadow map[string]string `json:"shadow"`
	}
	if err := readJSON(tokensPath, &raw); err != nil {
		return nil, err
	}
	t := &Tokens{
		Color: map[string]Themed{}, Tint: map[string]Themed{}, Mesa: map[string]MesaState{},
		Fonts: raw.Font.Family, Scale: map[string]TextStyle{}, Space: raw.Space, Radius: raw.Radius,
		Size: raw.Size, Motion: raw.Motion, Blur: raw.Material.Blur, Saturate: raw.Material.Saturate, Shadow: raw.Shadow,
	}
	if err := decodeSection(raw.Color, t.Color); err != nil {
		return nil, fmt.Errorf("color: %w", err)
	}
	if err := decodeSection(raw.Tint, t.Tint); err != nil {
		return nil, fmt.Errorf("tint: %w", err)
	}
	if err := decodeSection(raw.Mesa, t.Mesa); err != nil {
		return nil, fmt.Errorf("mesa: %w", err)
	}
	if err := decodeSection(raw.Font.Scale, t.Scale); err != nil {
		return nil, fmt.Errorf("font.scale: %w", err)
	}
	delete(t.Motion, "$comentario")
	for i, p := range raw.Contraste.Pares {
		var pr Pair
		if json.Unmarshal(p[0], &pr.Fg) != nil || json.Unmarshal(p[1], &pr.Bg) != nil || json.Unmarshal(p[2], &pr.Min) != nil {
			return nil, fmt.Errorf("contraste.pares[%d] mal formado", i)
		}
		t.Pairs = append(t.Pairs, pr)
	}

	var icons struct {
		Iconos map[string]Icon `json:"iconos"`
	}
	if err := readJSON(iconsPath, &icons); err != nil {
		return nil, err
	}
	t.Icons = icons.Iconos
	return t, t.validate()
}

func (t *Tokens) validate() error {
	check := func(where string, v Themed) error {
		for _, c := range []string{v.Light, v.Dark} {
			if _, err := ParseColor(c); err != nil {
				return fmt.Errorf("%s: %w", where, err)
			}
		}
		return nil
	}
	for _, k := range sortedKeys(t.Color) {
		if err := check("color."+k, t.Color[k]); err != nil {
			return err
		}
	}
	for _, k := range sortedKeys(t.Tint) {
		if err := check("tint."+k, t.Tint[k]); err != nil {
			return err
		}
	}
	for _, k := range sortedKeys(t.Mesa) {
		m := t.Mesa[k]
		if err := check("mesa."+k+".bg", m.Bg); err != nil {
			return err
		}
		if err := check("mesa."+k+".fg", m.Fg); err != nil {
			return err
		}
		if _, ok := t.Icons[m.Icono]; !ok {
			return fmt.Errorf("mesa.%s usa el icono %q que no existe en icons.json", k, m.Icono)
		}
	}
	for _, k := range sortedKeys(t.Icons) {
		ic := t.Icons[k]
		if ic.Lucide == "" || ic.SF == "" || ic.Cupertino == "" {
			return fmt.Errorf("icono %s: faltan nombres para alguna plataforma", k)
		}
		if _, ok := t.Tint[ic.Tint]; !ok {
			return fmt.Errorf("icono %s: tint %q no existe", k, ic.Tint)
		}
	}
	return nil
}

func decodeSection[T any](in map[string]json.RawMessage, out map[string]T) error {
	for k, v := range in {
		if strings.HasPrefix(k, "$") {
			continue
		}
		var x T
		if err := json.Unmarshal(v, &x); err != nil {
			return fmt.Errorf("%s: %w", k, err)
		}
		out[k] = x
	}
	return nil
}

func readJSON(path string, v any) error {
	b, err := os.ReadFile(path) //nolint:gosec // ruta de la línea de comandos del desarrollador
	if err != nil {
		return err
	}
	if err := json.Unmarshal(b, v); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	return nil
}

func sortedKeys[T any](m map[string]T) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return lessNatural(keys[i], keys[j]) })
	return keys
}

// lessNatural ordena "2" antes que "10" en la escala de espaciado.
func lessNatural(a, b string) bool {
	if len(a) != len(b) && isDigits(a) && isDigits(b) {
		return len(a) < len(b)
	}
	return a < b
}

func isDigits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return s != ""
}

// kebab convierte labelSecondary → label-secondary.
func kebab(s string) string {
	var b strings.Builder
	for i, r := range s {
		if r >= 'A' && r <= 'Z' {
			if i > 0 {
				b.WriteByte('-')
			}
			r += 'a' - 'A'
		}
		b.WriteRune(r)
	}
	return b.String()
}
