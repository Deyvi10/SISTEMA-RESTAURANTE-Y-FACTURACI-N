// Command contracts-gen genera los tipos de los eventos WebSocket (F2-06) para Go, TypeScript
// y Dart desde contracts/events/*.schema.json, para que el nodo, la caja, el KDS y la app de
// meseros nunca se desalineen (docs/03 §6).
//
//	go run ./tools/contracts-gen          # escribe los archivos generados
//	go run ./tools/contracts-gen -check   # falla si están desactualizados (CI)
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"go/format"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Schema es el subconjunto de JSON Schema que usan los contratos.
type Schema struct {
	Title       string             `json:"title"`
	Description string             `json:"description"`
	Version     int                `json:"x-event-version"`
	Type        string             `json:"type"`
	Format      string             `json:"format"`
	Decimal     bool               `json:"x-decimal"`
	Enum        []string           `json:"enum"`
	Items       *Schema            `json:"items"`
	Properties  map[string]*Schema `json:"properties"`
	Required    []string           `json:"required"`
}

type Evento struct {
	Nombre string // table.locked
	Tipo   string // TableLocked
	S      *Schema
}

func main() {
	root := flag.String("root", ".", "raíz del monorepo")
	check := flag.Bool("check", false, "no escribe: falla si los archivos generados no coinciden")
	flag.Parse()
	if err := run(*root, *check); err != nil {
		fmt.Fprintln(os.Stderr, "✗", err)
		os.Exit(1)
	}
}

func cargar(root string) ([]Evento, error) {
	files, err := filepath.Glob(filepath.Join(root, "contracts/events/*.schema.json"))
	if err != nil {
		return nil, err
	}
	var out []Evento
	for _, f := range files {
		if strings.HasPrefix(filepath.Base(f), "_") {
			continue // el sobre se escribe a mano en cada lenguaje
		}
		b, err := os.ReadFile(f) //nolint:gosec // archivos del repo
		if err != nil {
			return nil, err
		}
		var s Schema
		if err := json.Unmarshal(b, &s); err != nil {
			return nil, fmt.Errorf("%s: %w", f, err)
		}
		if s.Title+".schema.json" != filepath.Base(f) {
			return nil, fmt.Errorf("%s: el title debe ser el nombre del evento", f)
		}
		if s.Version < 1 {
			return nil, fmt.Errorf("%s: falta x-event-version", f)
		}
		for _, r := range s.Required {
			if s.Properties[r] == nil {
				return nil, fmt.Errorf("%s: required %q no está en properties", f, r)
			}
		}
		out = append(out, Evento{Nombre: s.Title, Tipo: pascal(s.Title), S: &s})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Nombre < out[j].Nombre })
	if len(out) == 0 {
		return nil, fmt.Errorf("no hay esquemas en contracts/events")
	}
	return out, nil
}

// pascal: "table.lock.request" / "by_user_id" → "TableLockRequest" / "ByUserID".
func pascal(s string) string {
	var b strings.Builder
	for _, p := range strings.FieldsFunc(s, func(r rune) bool { return r == '.' || r == '_' }) {
		switch p {
		case "id":
			b.WriteString("ID")
		case "ids":
			b.WriteString("IDs")
		default:
			b.WriteString(strings.ToUpper(p[:1]) + p[1:])
		}
	}
	return b.String()
}

func camel(s string) string {
	p := pascal(s)
	if strings.HasPrefix(p, "ID") {
		return "id" + p[2:]
	}
	return strings.ToLower(p[:1]) + p[1:]
}

func props(s *Schema) []string {
	var ks []string
	for k := range s.Properties {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	return ks
}

func requerido(s *Schema, k string) bool {
	for _, r := range s.Required {
		if r == k {
			return true
		}
	}
	return false
}

func run(root string, check bool) error {
	evs, err := cargar(root)
	if err != nil {
		return err
	}
	goSrc, err := format.Source([]byte(genGo(evs)))
	if err != nil {
		return fmt.Errorf("código Go generado inválido: %w", err)
	}
	salidas := map[string]string{
		"packages/go/eventos/eventos.gen.go":                     string(goSrc),
		"packages/ts/contracts/src/eventos.gen.ts":               genTS(evs),
		"packages/dart/restpos_contracts/lib/src/eventos.g.dart": genDart(evs),
	}
	stale := 0
	for rel, content := range salidas {
		path := filepath.Join(root, rel)
		actual, _ := os.ReadFile(path) //nolint:gosec // ruta fija del repo
		if bytes.Equal(actual, []byte(content)) {
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
		return fmt.Errorf("%d archivos no coinciden con contracts/events: ejecuta make contracts", stale)
	}
	fmt.Printf("✓ %d eventos: tipos Go, TS y Dart al día\n", len(evs))
	return nil
}

const aviso = "GENERADO por tools/contracts-gen desde contracts/events. No editar: ejecuta `make contracts`."

// ---------- Go ----------

func goTipo(s *Schema, req bool) string {
	var t string
	switch {
	case s.Type == "array":
		return "[]" + goTipo(s.Items, true)
	case s.Format == "uuid":
		t = "ids.ID"
	case s.Format == "date-time":
		t = "time.Time"
	case s.Type == "integer":
		t = "int64"
	case s.Type == "boolean":
		t = "bool"
	default:
		t = "string"
	}
	if !req {
		return "*" + t
	}
	return t
}

func genGo(evs []Evento) string {
	var b strings.Builder
	fmt.Fprintf(&b, "// %s\n\npackage eventos\n\nimport (\n\t\"time\"\n\n\t\"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/ids\"\n)\n\n", aviso)
	b.WriteString("// Nombres de los eventos.\nconst (\n")
	for _, e := range evs {
		fmt.Fprintf(&b, "\tTipo%s = %q\n", e.Tipo, e.Nombre)
	}
	b.WriteString(")\n\n// Versiones vigentes de cada evento (campo v del sobre).\nvar Versiones = map[string]int{\n")
	for _, e := range evs {
		fmt.Fprintf(&b, "\tTipo%s: %d,\n", e.Tipo, e.S.Version)
	}
	b.WriteString("}\n")
	for _, e := range evs {
		fmt.Fprintf(&b, "\n// %s: %s\ntype %s struct {\n", e.Nombre, e.S.Description, e.Tipo)
		for _, k := range props(e.S) {
			p := e.S.Properties[k]
			req := requerido(e.S, k)
			tag := k
			if !req {
				tag += ",omitempty"
			}
			com := ""
			if p.Description != "" {
				com = " // " + p.Description
			} else if len(p.Enum) > 0 {
				com = " // " + strings.Join(p.Enum, ", ")
			}
			fmt.Fprintf(&b, "\t%s %s `json:%q`%s\n", pascal(k), goTipo(p, req), tag, com)
		}
		fmt.Fprintf(&b, "}\n\n// Tipo devuelve el nombre del evento.\nfunc (%s) Tipo() string { return Tipo%s }\n", e.Tipo, e.Tipo)
	}
	return b.String()
}

// ---------- TypeScript ----------

func tsTipo(s *Schema) string {
	switch {
	case s.Type == "array":
		return tsTipo(s.Items) + "[]"
	case len(s.Enum) > 0:
		q := make([]string, len(s.Enum))
		for i, v := range s.Enum {
			q[i] = fmt.Sprintf("%q", v)
		}
		return strings.Join(q, " | ")
	case s.Type == "integer":
		return "number"
	case s.Type == "boolean":
		return "boolean"
	default:
		return "string"
	}
}

func genTS(evs []Evento) string {
	var b strings.Builder
	fmt.Fprintf(&b, "// %s\n\n/** Sobre de todo mensaje WebSocket (docs/03 §6). */\nexport interface Sobre<T extends string = string, D = unknown> {\n  v: number;\n  id: string;\n  type: T;\n  ts: string;\n  data: D;\n}\n", aviso)
	for _, e := range evs {
		fmt.Fprintf(&b, "\n/** %s: %s */\nexport interface %s {\n", e.Nombre, e.S.Description, e.Tipo)
		for _, k := range props(e.S) {
			p := e.S.Properties[k]
			opt := "?"
			if requerido(e.S, k) {
				opt = ""
			}
			if p.Description != "" {
				fmt.Fprintf(&b, "  /** %s */\n", p.Description)
			}
			fmt.Fprintf(&b, "  %s%s: %s;\n", k, opt, tsTipo(p))
		}
		b.WriteString("}\n")
	}
	b.WriteString("\n/** Todos los eventos, discriminados por `type`. */\nexport type Evento =\n")
	for i, e := range evs {
		sep := ""
		if i == len(evs)-1 {
			sep = ";"
		}
		fmt.Fprintf(&b, "  | Sobre<%q, %s>%s\n", e.Nombre, e.Tipo, sep)
	}
	b.WriteString("\nexport const VERSIONES: Record<Evento[\"type\"], number> = {\n")
	for _, e := range evs {
		fmt.Fprintf(&b, "  %q: %d,\n", e.Nombre, e.S.Version)
	}
	b.WriteString("};\n")
	return b.String()
}

// ---------- Dart ----------

func dartTipo(s *Schema) string {
	switch {
	case s.Type == "array":
		return "List<" + dartTipo(s.Items) + ">"
	case s.Format == "date-time":
		return "DateTime"
	case s.Type == "integer":
		return "int"
	case s.Type == "boolean":
		return "bool"
	default:
		return "String"
	}
}

func dartDesde(s *Schema, expr string) string {
	switch {
	case s.Type == "array":
		return fmt.Sprintf("(%s as List).map((e) => %s).toList()", expr, dartDesde(s.Items, "e"))
	case s.Format == "date-time":
		return fmt.Sprintf("DateTime.parse(%s as String)", expr)
	default:
		return fmt.Sprintf("%s as %s", expr, dartTipo(s))
	}
}

func dartHacia(s *Schema, expr string) string {
	if s.Format == "date-time" {
		return expr + ".toUtc().toIso8601String()"
	}
	return expr
}

func genDart(evs []Evento) string {
	var b strings.Builder
	fmt.Fprintf(&b, "// %s\n// ignore_for_file: public_member_api_docs, sort_constructors_first\n\n", aviso)
	b.WriteString("/// Versiones vigentes de cada evento.\nconst versionesEventos = <String, int>{\n")
	for _, e := range evs {
		fmt.Fprintf(&b, "  '%s': %d,\n", e.Nombre, e.S.Version)
	}
	b.WriteString("};\n")
	for _, e := range evs {
		fmt.Fprintf(&b, "\n/// %s: %s\nclass %s {\n  static const tipo = '%s';\n", e.Nombre, e.S.Description, e.Tipo, e.Nombre)
		var params, desde, hacia []string
		for _, k := range props(e.S) {
			p, req, f := e.S.Properties[k], requerido(e.S, k), camel(k)
			t := dartTipo(p)
			if req {
				fmt.Fprintf(&b, "  final %s %s;\n", t, f)
				params = append(params, "required this."+f)
				desde = append(desde, fmt.Sprintf("%s: %s", f, dartDesde(p, fmt.Sprintf("j['%s']", k))))
				hacia = append(hacia, fmt.Sprintf("'%s': %s", k, dartHacia(p, f)))
			} else {
				fmt.Fprintf(&b, "  final %s? %s;\n", t, f)
				params = append(params, "this."+f)
				desde = append(desde, fmt.Sprintf("%s: j['%s'] == null ? null : %s", f, k, dartDesde(p, fmt.Sprintf("j['%s']", k))))
				hacia = append(hacia, fmt.Sprintf("if (%s != null) '%s': %s", f, k, dartHacia(p, f+"!")))
			}
		}
		fmt.Fprintf(&b, "\n  const %s({%s});\n\n", e.Tipo, strings.Join(params, ", "))
		fmt.Fprintf(&b, "  factory %s.fromJson(Map<String, dynamic> j) => %s(\n", e.Tipo, e.Tipo)
		for _, d := range desde {
			fmt.Fprintf(&b, "        %s,\n", d)
		}
		b.WriteString("      );\n\n  Map<String, dynamic> toJson() => {\n")
		for _, h := range hacia {
			fmt.Fprintf(&b, "        %s,\n", h)
		}
		b.WriteString("      };\n}\n")
	}
	return b.String()
}
