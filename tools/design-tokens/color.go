package main

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// RGBA es un color con canales 0..255 y alfa 0..1.
type RGBA struct {
	R, G, B uint8
	A       float64 // solo presentación: nunca interviene en dinero
}

// ParseColor acepta #RRGGBB y rgba(r,g,b,a).
func ParseColor(s string) (RGBA, error) {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "#") && len(s) == 7 {
		v, err := strconv.ParseUint(s[1:], 16, 32)
		if err != nil {
			return RGBA{}, fmt.Errorf("color %q: %w", s, err)
		}
		return RGBA{uint8(v >> 16), uint8(v >> 8), uint8(v), 1}, nil //nolint:gosec // G115: bytes de un valor de 24 bits
	}
	if strings.HasPrefix(s, "rgba(") && strings.HasSuffix(s, ")") {
		parts := strings.Split(s[5:len(s)-1], ",")
		if len(parts) != 4 {
			return RGBA{}, fmt.Errorf("color %q: rgba necesita 4 valores", s)
		}
		var ch [3]uint8
		for i := range 3 {
			v, err := strconv.Atoi(strings.TrimSpace(parts[i]))
			if err != nil || v < 0 || v > 255 {
				return RGBA{}, fmt.Errorf("color %q: canal inválido", s)
			}
			ch[i] = uint8(v) //nolint:gosec // G115: validado 0..255
		}
		a, err := strconv.ParseFloat(strings.TrimSpace(parts[3]), 64)
		if err != nil || a < 0 || a > 1 {
			return RGBA{}, fmt.Errorf("color %q: alfa inválido", s)
		}
		return RGBA{ch[0], ch[1], ch[2], a}, nil
	}
	return RGBA{}, fmt.Errorf("color %q: usa #RRGGBB o rgba(r,g,b,a)", s)
}

// Over compone c sobre un fondo opaco.
func (c RGBA) Over(bg RGBA) RGBA {
	mix := func(f, b uint8) uint8 {
		return uint8(math.Round(float64(f)*c.A + float64(b)*(1-c.A))) //nolint:gosec // G115: mezcla de dos valores 0..255
	}
	return RGBA{mix(c.R, bg.R), mix(c.G, bg.G), mix(c.B, bg.B), 1}
}

// Luminance es la luminancia relativa de WCAG 2.x.
func (c RGBA) Luminance() float64 {
	f := func(v uint8) float64 {
		s := float64(v) / 255
		if s <= 0.03928 {
			return s / 12.92
		}
		return math.Pow((s+0.055)/1.055, 2.4)
	}
	return 0.2126*f(c.R) + 0.7152*f(c.G) + 0.0722*f(c.B)
}

// Contrast devuelve la relación de contraste WCAG entre dos colores opacos.
func Contrast(a, b RGBA) float64 {
	la, lb := a.Luminance(), b.Luminance()
	if la < lb {
		la, lb = lb, la
	}
	return (la + 0.05) / (lb + 0.05)
}

// ARGB32 devuelve el entero 0xAARRGGBB que usa Flutter.
func (c RGBA) ARGB32() uint32 {
	a := uint32(math.Round(c.A * 255))
	return a<<24 | uint32(c.R)<<16 | uint32(c.G)<<8 | uint32(c.B)
}
