package app

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/edge-node/internal/store"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/ids"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/money"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/secreto"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/sri"
)

// validarXSDNC valida contra el XSD oficial de la nota de crédito 1.1.0.
func validarXSDNC(t *testing.T, doc string) {
	t.Helper()
	dir := t.TempDir()
	for _, n := range []string{"NotaCredito_V1.1.0.xsd", "xmldsig-core-schema.xsd"} {
		b, _ := sri.XSD.ReadFile("xsd/" + n)
		_ = os.WriteFile(filepath.Join(dir, n), b, 0o600)
	}
	f := filepath.Join(dir, "nc.xml")
	_ = os.WriteFile(f, []byte(doc), 0o600)
	esquema := filepath.Join(dir, "NotaCredito_V1.1.0.xsd")
	var cmd *exec.Cmd
	if x, err := exec.LookPath("xmllint"); err == nil {
		cmd = exec.Command(x, "--noout", "--nonet", "--schema", esquema, f)
	} else if py, err := exec.LookPath("python3"); err == nil && exec.Command(py, "-c", "import lxml").Run() == nil {
		cmd = exec.Command(py, "-c", `import sys
from lxml import etree
p = etree.XMLParser(no_network=True, load_dtd=False)
s = etree.XMLSchema(etree.parse(sys.argv[1], p))
sys.exit(0 if s.validate(etree.parse(sys.argv[2], p)) else print(s.error_log) or 1)`, esquema, f)
	} else {
		return
	}
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("el XSD oficial rechaza la nota de crédito: %s\n%s", out, doc)
	}
}

// F5-13: nota de crédito parcial y luego total sobre una factura a consumidor final, con
// devolución en efectivo que el Cierre Z descuenta de lo cobrado.
func TestNotaDeCreditoParcialYTotal(t *testing.T) {
	c := nuevaCaja(t, nocheDel25)
	tel := c.emparejar(t, "Teléfono")
	tel.entrar(c.carlos, pinCarlos)
	c.pos.req("POST", "/v1/turnos", map[string]any{"cajaId": c.caja1, "fondoInicial": "20"})
	c.conFacturacion(t)
	orden, total := c.ordenEnMesa(t, tel, "factura-nc", c.mesa1, plato(c.ceviche, "1"), plato(c.cerveza, "2"))
	st, cobro, raw := c.cobrar(orden, c.efectivo.String(), "", "cobro-nc")
	if st != 200 {
		t.Fatalf("cobro: %d %v", st, raw)
	}
	factura := c.comprobantes(t)[0]
	var facturaID string
	_ = c.a.Store.Read().QueryRow(`SELECT id FROM comprobantes WHERE tipo = '01'`).Scan(&facturaID)

	// Un administrador (el cajero Luis no tiene EMITIR_NC por defecto).
	sofia := ids.New()
	var pepper []byte
	_ = c.a.Store.Read().QueryRow(`SELECT pin_pepper FROM nodo WHERE id = 1`).Scan(&pepper)
	h, _ := secreto.HashPIN(pepper, sofia, "5173")
	if err := c.a.Store.Write(context.Background(), func(tx *store.Tx) error {
		_, err := tx.Exec(`INSERT INTO usuarios (id, tenant_id, nombre_mostrar, rol, pin_hash, activo) VALUES (?, 't', 'Sofía A.', 'ADMIN', ?, 1)`, sofia.String(), h)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	nc := func(body map[string]any) (int, map[string]any) {
		body["cajaId"] = c.caja1
		return c.pos.req("POST", "/v1/caja/comprobantes/"+facturaID+"/nota-credito", body)
	}
	parcial := map[string]any{"lineas": []map[string]any{{"indice": 1, "cantidad": "1"}}, "motivo": "Una cerveza vino caliente", "idempotencyKey": "nc-parcial-1",
		"devolucion": map[string]any{"metodoId": c.efectivo}}

	// Sin el permiso pide supervisor; con su PIN, sigue.
	if st, out := nc(parcial); st != 403 || out["code"] != "REQUIERE_SUPERVISOR" {
		t.Fatalf("sin permiso: %d %v", st, out)
	}
	autorizar := func() string {
		_, au := c.pos.req("POST", "/v1/autorizaciones", map[string]any{"usuarioId": sofia, "pin": "5173", "accion": "EMITIR_NC", "referencia": facturaID})
		return au["token"].(string)
	}
	parcial["autorizacion"] = autorizar()
	// La factura fue a consumidor final: la NC exige identificar al cliente.
	if st, out := nc(parcial); st != 422 || out["code"] != "REQUIERE_CLIENTE" {
		t.Fatalf("consumidor final: %d %v", st, out)
	}
	parcial["autorizacion"] = autorizar()
	parcial["comprador"] = map[string]any{"identificacion": "1710034065", "razonSocial": "María Pérez", "email": "maria@example.com"}
	st, out := nc(parcial)
	if st != 200 || out["numero"] != "001-002-000000001" || out["tipo"] != "04" || out["revierteTodo"] != false || out["autorizadoPor"] != "Sofía A." || out["devuelto"] == nil {
		t.Fatalf("parcial: %d %v", st, out)
	}
	valorParcial := money.MustParse(out["valor"].(string))
	// El mismo pedido repetido (mala señal) no emite otra nota.
	parcial["autorizacion"] = autorizar()
	if st, again := nc(parcial); st != 200 || again["id"] != out["id"] {
		t.Fatalf("idempotencia: %d %v", st, again)
	}

	// Detalle: lo que queda por devolver.
	_, det := c.pos.req("GET", "/v1/caja/comprobantes/"+facturaID, nil)
	lineas := det["lineas"].([]any)
	if lineas[1].(map[string]any)["disponible"] != "1" || len(det["notasCredito"].([]any)) != 1 || det["consumidorFinal"] != true {
		t.Fatalf("detalle: %v", det)
	}
	// Pasarse de lo que queda: no.
	if st, out := c.pos.req("POST", "/v1/caja/comprobantes/"+facturaID+"/nota-credito", map[string]any{"cajaId": c.caja1, "lineas": []map[string]any{{"indice": 1, "cantidad": "2"}},
		"motivo": "otra vez", "idempotencyKey": "nc-pasarse", "autorizacion": autorizar(), "comprador": parcial["comprador"]}); st != 409 || out["code"] != "NC_INVALIDA" {
		t.Fatalf("pasarse: %d %v", st, out)
	}
	// El resto de la factura: la nota total.
	st, out = c.pos.req("POST", "/v1/caja/comprobantes/"+facturaID+"/nota-credito", map[string]any{"cajaId": c.caja1, "todo": true, "motivo": "Mesa cancelada",
		"idempotencyKey": "nc-total-1", "autorizacion": autorizar(), "comprador": parcial["comprador"]})
	if st != 200 || out["numero"] != "001-002-000000002" || out["revierteTodo"] != true {
		t.Fatalf("total: %d %v", st, out)
	}
	valorTotal := money.MustParse(out["valor"].(string))
	// Entre las dos notas: la factura sin la propina (la NC no lleva propina).
	sinPropina := total.Sub(money.MustParse(cobro.Documento.Totales.Propina))
	if !valorParcial.Add(valorTotal).Equal(sinPropina) {
		t.Fatalf("notas %s + %s, factura sin propina %s", valorParcial, valorTotal, sinPropina)
	}
	if st, out := c.pos.req("POST", "/v1/caja/comprobantes/"+facturaID+"/nota-credito", map[string]any{"cajaId": c.caja1, "todo": true, "motivo": "de nuevo",
		"idempotencyKey": "nc-nada-1", "autorizacion": autorizar(), "comprador": parcial["comprador"]}); st != 409 {
		t.Fatalf("ya revertida: %d %v", st, out)
	}

	// Las NC tienen su propia numeración (tipo 04), validan contra el XSD y viajan a la nube.
	var xmls []string
	rows, _ := c.a.Store.Read().Query(`SELECT xml FROM comprobantes WHERE tipo = '04' ORDER BY secuencial`)
	for rows.Next() {
		var x string
		_ = rows.Scan(&x)
		xmls = append(xmls, x)
	}
	_ = rows.Close()
	if len(xmls) != 2 {
		t.Fatalf("notas: %d", len(xmls))
	}
	for _, x := range xmls {
		for _, quiero := range []string{"<numDocModificado>" + factura.serie[:3] + "-" + factura.serie[3:] + "-000000001</numDocModificado>",
			"<tipoIdentificacionComprador>05</tipoIdentificacionComprador>", "<codDocModificado>01</codDocModificado>"} {
			if !strings.Contains(x, quiero) {
				t.Errorf("la NC no trae %s", quiero)
			}
		}
		validarXSDNC(t, x)
	}
	var payloads []string
	rows, _ = c.a.Store.Read().Query(`SELECT payload FROM outbox WHERE tipo = ? ORDER BY rowid`, EventoComprobanteEmitido)
	for rows.Next() {
		var p string
		_ = rows.Scan(&p)
		payloads = append(payloads, p)
	}
	_ = rows.Close()
	var ev map[string]any
	_ = json.Unmarshal([]byte(payloads[len(payloads)-1]), &ev)
	if ev["tipo"] != "04" || ev["sustentoClave"] != factura.clave || ev["total"] != true {
		t.Fatalf("evento NC: %v", ev["tipo"])
	}

	// Cierre Z: lo devuelto en efectivo sale de lo cobrado en efectivo.
	var turno string
	_ = c.a.Store.Read().QueryRow(`SELECT id FROM turnos_caja WHERE estado = 'ABIERTO'`).Scan(&turno)
	tid, _ := ids.Parse(turno)
	porMetodo, err := sumarPorMetodo(context.Background(), c.a.Store.Read(), tid)
	if err != nil {
		t.Fatal(err)
	}
	if !porMetodo[c.efectivo].Equal(total.Sub(valorParcial)) {
		t.Fatalf("efectivo neto %s; cobrado %s menos devuelto %s", porMetodo[c.efectivo], total, valorParcial)
	}

	// Auditoría y ticket.
	var auditadas int
	_ = c.a.Store.Read().QueryRow(`SELECT count(*) FROM auditoria WHERE accion = 'NOTA_CREDITO_EMITIDA'`).Scan(&auditadas)
	if auditadas != 2 {
		t.Fatalf("auditoría: %d", auditadas)
	}
	esperar(t, func() bool {
		return slices.ContainsFunc(c.cocina.Textos(), func(s string) bool {
			return strings.Contains(s, "NOTA DE CRÉDITO") && strings.Contains(s, "001-002-000000002")
		})
	})
	// En la lista de la caja: la factura y sus dos notas; se busca por cliente o número.
	var todos, deMaria, porNumero []ComprobanteCaja
	if st, err := c.pos.reqLista("GET", "/v1/caja/comprobantes", &todos); st != 200 || err != nil || len(todos) != 3 {
		t.Fatalf("lista: %d %v %v", st, err, todos)
	}
	_, _ = c.pos.reqLista("GET", "/v1/caja/comprobantes?q=Mar%C3%ADa", &deMaria)
	_, _ = c.pos.reqLista("GET", "/v1/caja/comprobantes?q=001-002-000000002", &porNumero)
	if len(deMaria) != 2 || len(porNumero) != 1 || porNumero[0].Tipo != "04" || porNumero[0].Sustento == nil || *porNumero[0].Sustento != "001-002-000000001" || porNumero[0].Estado != "EMITIDO" {
		t.Fatalf("búsqueda: %v %v", deMaria, porNumero)
	}
}

// Un mesero sin permiso ni supervisor no puede, y lo que no es factura no se revierte.
func TestNotaDeCreditoReglas(t *testing.T) {
	c := nuevaCaja(t, nocheDel25)
	c.pos.req("POST", "/v1/turnos", map[string]any{"cajaId": c.caja1, "fondoInicial": "20"})
	if st, out := c.pos.req("POST", "/v1/caja/comprobantes/"+ids.New().String()+"/nota-credito", map[string]any{"cajaId": c.caja1, "todo": true,
		"motivo": "x", "idempotencyKey": "nc-reglas-1"}); st != 422 {
		t.Fatalf("motivo corto: %d %v", st, out)
	}
	if st, out := c.pos.req("POST", "/v1/caja/comprobantes/"+ids.New().String()+"/nota-credito", map[string]any{"cajaId": c.caja1,
		"motivo": "sin líneas", "idempotencyKey": "nc-reglas-2"}); st != 422 {
		t.Fatalf("sin líneas: %d %v", st, out)
	}
	if st, out := c.pos.req("POST", "/v1/caja/comprobantes/"+ids.New().String()+"/nota-credito", map[string]any{"cajaId": c.caja1, "todo": true,
		"motivo": "devolver inventario", "devolverInventario": true, "idempotencyKey": "nc-reglas-3"}); st != 403 || out["code"] != "SOLO_ADMIN" {
		t.Fatalf("inventario sin ser admin: %d %v", st, out)
	}
}
