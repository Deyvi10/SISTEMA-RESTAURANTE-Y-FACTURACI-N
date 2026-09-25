package main

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

const header = "GENERADO por tools/design-tokens desde packages/design/*.json. No editar: ejecuta `make tokens`."

func fmtNum(f float64) string { return strconv.FormatFloat(f, 'f', -1, 64) }

// motionValue devuelve la duración en ms o la curva de easing.
func motionValue(raw json.RawMessage) (ms int, ease string) {
	if err := json.Unmarshal(raw, &ms); err == nil {
		return ms, ""
	}
	_ = json.Unmarshal(raw, &ease)
	return 0, ease
}

// ---------- CSS ----------

func genCSS(t *Tokens) string {
	var b strings.Builder
	fmt.Fprintf(&b, "/* %s */\n\n:root {\n  color-scheme: light;\n", header)
	colorVars(&b, t, func(v Themed) string { return v.Light })
	fmt.Fprintf(&b, "\n  --rp-font-text: %s;\n  --rp-font-display: %s;\n  --rp-font-mono: %s;\n", t.Fonts["text"], t.Fonts["display"], t.Fonts["mono"])
	for _, k := range sortedKeys(t.Scale) {
		s := t.Scale[k]
		n := kebab(k)
		fmt.Fprintf(&b, "  --rp-text-%s-size: %spx;\n  --rp-text-%s-line: %spx;\n  --rp-text-%s-weight: %d;\n  --rp-text-%s-tracking: %sem;\n",
			n, fmtNum(s.Size), n, fmtNum(s.Line), n, s.Weight, n, fmtNum(s.Tracking))
	}
	b.WriteString("\n")
	for _, k := range sortedKeys(t.Space) {
		fmt.Fprintf(&b, "  --rp-space-%s: %dpx;\n", k, t.Space[k])
	}
	for _, k := range sortedKeys(t.Radius) {
		fmt.Fprintf(&b, "  --rp-radius-%s: %dpx;\n", kebab(k), t.Radius[k])
	}
	for _, k := range sortedKeys(t.Size) {
		fmt.Fprintf(&b, "  --rp-size-%s: %dpx;\n", kebab(k), t.Size[k])
	}
	for _, k := range sortedKeys(t.Motion) {
		ms, ease := motionValue(t.Motion[k])
		if ease != "" {
			fmt.Fprintf(&b, "  --rp-%s: %s;\n", kebab(k), ease)
		} else {
			fmt.Fprintf(&b, "  --rp-duration-%s: %dms;\n", kebab(k), ms)
		}
	}
	fmt.Fprintf(&b, "  --rp-blur: %dpx;\n  --rp-saturate: %s;\n", t.Blur, fmtNum(t.Saturate))
	for _, k := range sortedKeys(t.Shadow) {
		fmt.Fprintf(&b, "  --rp-shadow-%s: %s;\n", kebab(k), t.Shadow[k])
	}
	b.WriteString("}\n\n")

	dark := func(indent string) {
		fmt.Fprintf(&b, "%scolor-scheme: dark;\n", indent)
		var inner strings.Builder
		colorVars(&inner, t, func(v Themed) string { return v.Dark })
		for _, line := range strings.Split(strings.TrimRight(inner.String(), "\n"), "\n") {
			b.WriteString(indent[:len(indent)-2] + line + "\n")
		}
		// Sin sombras en modo oscuro: la profundidad la dan las superficies más claras.
		fmt.Fprintf(&b, "%s--rp-shadow-card: none;\n", indent)
	}
	b.WriteString("/* Oscuro por preferencia del sistema, salvo que se elija claro explícitamente. */\n")
	b.WriteString("@media (prefers-color-scheme: dark) {\n  :root:not([data-theme=\"light\"]) {\n")
	dark("    ")
	b.WriteString("  }\n}\n\n/* Oscuro elegido explícitamente. */\n:root[data-theme=\"dark\"] {\n")
	dark("  ")
	b.WriteString("}\n")
	return b.String()
}

func colorVars(b *strings.Builder, t *Tokens, pick func(Themed) string) {
	for _, k := range sortedKeys(t.Color) {
		fmt.Fprintf(b, "  --rp-color-%s: %s;\n", kebab(k), pick(t.Color[k]))
	}
	for _, k := range sortedKeys(t.Tint) {
		fmt.Fprintf(b, "  --rp-tint-%s: %s;\n", kebab(k), pick(t.Tint[k]))
	}
	for _, k := range sortedKeys(t.Mesa) {
		m := t.Mesa[k]
		fmt.Fprintf(b, "  --rp-mesa-%s-bg: %s;\n  --rp-mesa-%s-fg: %s;\n", kebab(k), pick(m.Bg), kebab(k), pick(m.Fg))
	}
}

// ---------- Tailwind v4 (@theme) ----------

func genTailwind(t *Tokens) string {
	var b strings.Builder
	fmt.Fprintf(&b, "/* %s\n   Uso: @import \"tailwindcss\"; @import \"@restpos/ui/tokens.css\"; @import \"@restpos/ui/tailwind.css\"; */\n\n@theme inline {\n", header)
	for _, k := range sortedKeys(t.Color) {
		fmt.Fprintf(&b, "  --color-%s: var(--rp-color-%s);\n", kebab(k), kebab(k))
	}
	for _, k := range sortedKeys(t.Tint) {
		fmt.Fprintf(&b, "  --color-tint-%s: var(--rp-tint-%s);\n", kebab(k), kebab(k))
	}
	for _, k := range sortedKeys(t.Mesa) {
		fmt.Fprintf(&b, "  --color-mesa-%s: var(--rp-mesa-%s-bg);\n  --color-on-mesa-%s: var(--rp-mesa-%s-fg);\n", kebab(k), kebab(k), kebab(k), kebab(k))
	}
	b.WriteString("  --font-sans: var(--rp-font-text);\n  --font-display: var(--rp-font-display);\n  --font-mono: var(--rp-font-mono);\n")
	for _, k := range sortedKeys(t.Scale) {
		n := kebab(k)
		fmt.Fprintf(&b, "  --text-%s: var(--rp-text-%s-size);\n  --text-%s--line-height: var(--rp-text-%s-line);\n  --text-%s--letter-spacing: var(--rp-text-%s-tracking);\n  --text-%s--font-weight: var(--rp-text-%s-weight);\n",
			n, n, n, n, n, n, n, n)
	}
	for _, k := range sortedKeys(t.Radius) {
		fmt.Fprintf(&b, "  --radius-%s: var(--rp-radius-%s);\n", kebab(k), kebab(k))
	}
	for _, k := range sortedKeys(t.Shadow) {
		fmt.Fprintf(&b, "  --shadow-%s: var(--rp-shadow-%s);\n", kebab(k), kebab(k))
	}
	b.WriteString("  --ease-standard: var(--rp-ease-standard);\n  --ease-spring: var(--rp-ease-spring);\n}\n")
	return b.String()
}

// ---------- TypeScript ----------

func genTS(t *Tokens) string {
	var b strings.Builder
	fmt.Fprintf(&b, "// %s\n\n", header)
	b.WriteString("/** Referencias a variables CSS: úsalas en estilos para respetar el tema claro/oscuro. */\nexport const color = {\n")
	for _, k := range sortedKeys(t.Color) {
		fmt.Fprintf(&b, "  %s: \"var(--rp-color-%s)\",\n", k, kebab(k))
	}
	b.WriteString("} as const;\n\nexport const tint = {\n")
	for _, k := range sortedKeys(t.Tint) {
		fmt.Fprintf(&b, "  %s: \"var(--rp-tint-%s)\",\n", k, kebab(k))
	}
	b.WriteString("} as const;\n\n/** Estados de mesa (RF-03-02): fondo, texto e icono semántico. */\nexport const mesa = {\n")
	for _, k := range sortedKeys(t.Mesa) {
		fmt.Fprintf(&b, "  %s: { bg: \"var(--rp-mesa-%s-bg)\", fg: \"var(--rp-mesa-%s-fg)\", icon: %q },\n", k, kebab(k), kebab(k), t.Mesa[k].Icono)
	}
	b.WriteString("} as const;\n\nexport type EstadoMesa = keyof typeof mesa;\nexport type Tint = keyof typeof tint;\n\n")

	b.WriteString("/** Valores crudos por tema, para canvas, gráficos o correos donde no hay variables CSS. */\nexport const raw = {\n  color: {\n")
	for _, k := range sortedKeys(t.Color) {
		fmt.Fprintf(&b, "    %s: { light: %q, dark: %q },\n", k, t.Color[k].Light, t.Color[k].Dark)
	}
	b.WriteString("  },\n  tint: {\n")
	for _, k := range sortedKeys(t.Tint) {
		fmt.Fprintf(&b, "    %s: { light: %q, dark: %q },\n", k, t.Tint[k].Light, t.Tint[k].Dark)
	}
	b.WriteString("  },\n} as const;\n\n")

	b.WriteString("export const text = {\n")
	for _, k := range sortedKeys(t.Scale) {
		s := t.Scale[k]
		fmt.Fprintf(&b, "  %s: { size: %s, line: %s, weight: %d, tracking: %s },\n", k, fmtNum(s.Size), fmtNum(s.Line), s.Weight, fmtNum(s.Tracking))
	}
	b.WriteString("} as const;\n\nexport const space = {")
	for _, k := range sortedKeys(t.Space) {
		fmt.Fprintf(&b, " %q: %d,", k, t.Space[k])
	}
	b.WriteString(" } as const;\nexport const radius = {")
	for _, k := range sortedKeys(t.Radius) {
		fmt.Fprintf(&b, " %s: %d,", k, t.Radius[k])
	}
	b.WriteString(" } as const;\nexport const size = {")
	for _, k := range sortedKeys(t.Size) {
		fmt.Fprintf(&b, " %s: %d,", k, t.Size[k])
	}
	b.WriteString(" } as const;\n\n")

	b.WriteString("/** Iconos semánticos → nombre de Lucide (web). Nunca uses un nombre de Lucide directo en una pantalla. */\nexport const icons = {\n")
	for _, k := range sortedKeys(t.Icons) {
		fmt.Fprintf(&b, "  %s: %q,\n", k, t.Icons[k].Lucide)
	}
	b.WriteString("} as const;\n\n/** Color de fondo del mosaico de cada icono (estilo icono de app de iOS). */\nexport const iconTint: Record<IconName, Tint> = {\n")
	for _, k := range sortedKeys(t.Icons) {
		fmt.Fprintf(&b, "  %s: %q,\n", k, t.Icons[k].Tint)
	}
	b.WriteString("};\n\nexport type IconName = keyof typeof icons;\n")
	return b.String()
}

// ---------- Dart / Flutter ----------

func dartColor(s string) string {
	c, _ := ParseColor(s) // validado en Load
	return fmt.Sprintf("Color(0x%08X)", c.ARGB32())
}

func dartIdent(k string) string {
	if isDigits(k) {
		return "s" + k
	}
	return k
}

func genDart(t *Tokens) string {
	var b strings.Builder
	fmt.Fprintf(&b, "// %s\n// ignore_for_file: public_member_api_docs\n\nimport 'package:flutter/cupertino.dart';\n\n", header)

	themedClass := func(name, doc string, fields map[string]Themed) {
		keys := sortedKeys(fields)
		fmt.Fprintf(&b, "/// %s\n@immutable\nclass %s {\n  const %s({\n", doc, name, name)
		for _, k := range keys {
			fmt.Fprintf(&b, "    required this.%s,\n", k)
		}
		b.WriteString("  });\n\n")
		for _, k := range keys {
			if u := fields[k].Uso; u != "" {
				fmt.Fprintf(&b, "  /// %s\n", u)
			}
			fmt.Fprintf(&b, "  final Color %s;\n", k)
		}
		for _, mode := range []string{"light", "dark"} {
			fmt.Fprintf(&b, "\n  static const %s = %s(\n", mode, name)
			for _, k := range keys {
				v := fields[k].Light
				if mode == "dark" {
					v = fields[k].Dark
				}
				fmt.Fprintf(&b, "    %s: %s,\n", k, dartColor(v))
			}
			b.WriteString("  );\n")
		}
		b.WriteString("}\n\n")
	}
	themedClass("RpColors", "Colores semánticos del sistema de diseño.", t.Color)
	themedClass("RpTints", "Colores de sistema de iOS para fondos, iconos y mosaicos.", t.Tint)

	mesa := map[string]Themed{}
	for k, m := range t.Mesa {
		mesa[k+"Bg"], mesa[k+"Fg"] = m.Bg, m.Fg
	}
	themedClass("RpMesaColors", "Colores de los estados de mesa (RF-03-02).", mesa)

	b.WriteString("/// Escala tipográfica de iOS. Usa la fuente del sistema (SF Pro en iOS, Roboto en Android).\nabstract final class RpText {\n")
	for _, k := range sortedKeys(t.Scale) {
		s := t.Scale[k]
		fmt.Fprintf(&b, "  static const %s = TextStyle(fontSize: %s, height: %s, fontWeight: FontWeight.w%d, letterSpacing: %s);\n",
			k, fmtNum(s.Size), strconv.FormatFloat(s.Line/s.Size, 'f', 4, 64), s.Weight, strconv.FormatFloat(s.Tracking*s.Size, 'f', 3, 64))
	}
	b.WriteString("}\n\nabstract final class RpSpace {\n")
	for _, k := range sortedKeys(t.Space) {
		fmt.Fprintf(&b, "  static const double %s = %d;\n", dartIdent(k), t.Space[k])
	}
	b.WriteString("}\n\nabstract final class RpRadius {\n")
	for _, k := range sortedKeys(t.Radius) {
		fmt.Fprintf(&b, "  static const double %s = %d;\n", k, t.Radius[k])
	}
	b.WriteString("}\n\nabstract final class RpSize {\n")
	for _, k := range sortedKeys(t.Size) {
		fmt.Fprintf(&b, "  static const double %s = %d;\n", k, t.Size[k])
	}
	b.WriteString("}\n\nabstract final class RpMotion {\n")
	for _, k := range sortedKeys(t.Motion) {
		if ms, ease := motionValue(t.Motion[k]); ease == "" {
			fmt.Fprintf(&b, "  static const %s = Duration(milliseconds: %d);\n", k, ms)
		} else {
			args := strings.TrimSuffix(strings.TrimPrefix(ease, "cubic-bezier("), ")")
			fmt.Fprintf(&b, "  static const %s = Cubic(%s);\n", k, args)
		}
	}
	b.WriteString("}\n\n/// Iconos semánticos → CupertinoIcons. Nunca uses CupertinoIcons directo en una pantalla.\nabstract final class RpIcons {\n")
	for _, k := range sortedKeys(t.Icons) {
		fmt.Fprintf(&b, "  static const IconData %s = CupertinoIcons.%s;\n", k, t.Icons[k].Cupertino)
	}
	b.WriteString("}\n")
	return b.String()
}
