package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/edge-node/internal/store"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/money"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/sri"
)

// conFacturacion activa la facturación en pruebas con el punto 001-002 de la caja.
func (c *cajaF4) conFacturacion(t *testing.T) {
	t.Helper()
	c.conPunto(t, "{}")
	if err := c.a.Store.Write(context.Background(), func(tx *store.Tx) error {
		if _, err := tx.Exec(`INSERT INTO configuracion_fiscal (tenant_id, ambiente, ruc, razon_social, nombre_comercial, direccion_matriz,
			obligado_contabilidad, regimen, facturacion_activa) VALUES ('t', 1, '1710034065001', 'ANDRADE JOSÉ LUIS', 'Don Pepe', 'Guayaquil', 0, 'RIMPE_EMPRENDEDOR', 1)`); err != nil {
			return err
		}
		_, err := tx.Exec(`INSERT INTO parametros_globales (clave, valor, descripcion) VALUES ('ruc_proveedor_sistema', '1790011674001', 'x')`)
		return err
	}); err != nil {
		t.Fatal(err)
	}
}

type comprobanteT struct {
	serie, clave, xml, hash, total string
	secuencial                     int64
}

func (c *cajaF4) comprobantes(t *testing.T) []comprobanteT {
	t.Helper()
	rows, err := c.a.Store.Read().Query(`SELECT serie, secuencial, clave_acceso, xml, hash, importe_total FROM comprobantes ORDER BY secuencial`)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rows.Close() }()
	var out []comprobanteT
	for rows.Next() {
		var x comprobanteT
		if err := rows.Scan(&x.serie, &x.secuencial, &x.clave, &x.xml, &x.hash, &x.total); err != nil {
			t.Fatal(err)
		}
		out = append(out, x)
	}
	return out
}

// validarXSD valida contra el XSD oficial embebido (xmllint o lxml; sin ninguno, no se valida).
func validarXSD(t *testing.T, doc string) {
	t.Helper()
	dir := t.TempDir()
	for _, n := range []string{"factura_V1.1.0.xsd", "xmldsig-core-schema.xsd"} {
		b, _ := sri.XSD.ReadFile("xsd/" + n)
		_ = os.WriteFile(filepath.Join(dir, n), b, 0o600)
	}
	f := filepath.Join(dir, "f.xml")
	_ = os.WriteFile(f, []byte(doc), 0o600)
	var cmd *exec.Cmd
	if x, err := exec.LookPath("xmllint"); err == nil {
		cmd = exec.Command(x, "--noout", "--nonet", "--schema", filepath.Join(dir, "factura_V1.1.0.xsd"), f)
	} else if py, err := exec.LookPath("python3"); err == nil && exec.Command(py, "-c", "import lxml").Run() == nil {
		cmd = exec.Command(py, "-c", `import sys
from lxml import etree
p = etree.XMLParser(no_network=True, load_dtd=False)
s = etree.XMLSchema(etree.parse(sys.argv[1], p))
sys.exit(0 if s.validate(etree.parse(sys.argv[2], p)) else print(s.error_log) or 1)`, filepath.Join(dir, "factura_V1.1.0.xsd"), f)
	} else {
		return
	}
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("el XSD oficial rechaza la factura: %s\n%s", out, doc)
	}
}

// F5-05: con la facturación activa, el cobro emite la factura con su clave de acceso, guarda el
// XML sin firmar con su hash, imprime el RIDE y deja el comprobante para la nube.
func TestCobroEmiteLaFactura(t *testing.T) {
	c := nuevaCaja(t, nocheDel25)
	tel := c.emparejar(t, "Teléfono")
	tel.entrar(c.carlos, pinCarlos)
	c.pos.req("POST", "/v1/turnos", map[string]any{"cajaId": c.caja1, "fondoInicial": "20"})
	// Antes de activar la facturación ya hubo documentos internos: la factura numera aparte.
	previa, _ := c.ordenEnMesa(t, tel, "interno-previo", c.mesa3, plato(c.cerveza, "1"))
	if st, out, raw := c.cobrar(previa, c.efectivo.String(), "", "cobro-interno-previo"); st != 200 || out.Documento.Tipo != "INTERNO" {
		t.Fatalf("documento interno previo: %d %v", st, raw)
	}
	c.conFacturacion(t)
	orden, total := c.ordenEnMesa(t, tel, "factura-0001", c.mesa1, plato(c.ceviche, "1"), plato(c.cerveza, "2"))
	st, out, raw := c.cobrar(orden, c.efectivo.String(), "", "cobro-fact-1")
	if st != 200 {
		t.Fatalf("cobro: %d %v", st, raw)
	}
	d := out.Documento
	if d.Tipo != "FACTURA" || d.Codigo != "001-002-000000001" || d.Ambiente != 1 {
		t.Fatalf("documento: %+v", d)
	}
	clave, err := sri.ParseClaveAcceso(d.ClaveAcceso)
	if err != nil || clave.Serie() != "001002" || clave.Secuencial() != "000000001" || clave.RUC() != "1710034065001" || clave.FechaEmision() != "25092026" {
		t.Fatalf("clave %s: %v", d.ClaveAcceso, err)
	}
	cs := c.comprobantes(t)
	if len(cs) != 1 || cs[0].clave != d.ClaveAcceso || cs[0].total != total.String() {
		t.Fatalf("comprobantes: %+v", cs)
	}
	suma := sha256.Sum256([]byte(cs[0].xml))
	if cs[0].hash != hex.EncodeToString(suma[:]) {
		t.Fatal("el hash no corresponde al XML")
	}
	for _, quiero := range []string{"<claveAcceso>" + d.ClaveAcceso + "</claveAcceso>", "<importeTotal>" + total.String() + "</importeTotal>",
		`<campoAdicional nombre="RUC Proveedor">1790011674001</campoAdicional>`, "<contribuyenteRimpe>CONTRIBUYENTE RÉGIMEN RIMPE</contribuyenteRimpe>",
		"<tipoIdentificacionComprador>07</tipoIdentificacionComprador>"} {
		if !strings.Contains(cs[0].xml, quiero) {
			t.Errorf("el XML no trae %s", quiero)
		}
	}
	validarXSD(t, cs[0].xml)

	// El RIDE sale en la impresora (la caja de prueba cae en la cocina).
	esperar(t, func() bool {
		return slices.ContainsFunc(c.cocina.Textos(), func(s string) bool { return strings.Contains(s, "No. 001-002-000000001") })
	})
	for _, txt := range c.cocina.Textos() {
		if strings.Contains(txt, "No. 001-002-000000001") {
			for _, quiero := range []string{"FACTURA", "AMBIENTE DE PRUEBAS - SIN VALIDEZ TRIBUTARIA", "pendiente de autorización del SRI", "[QR " + d.ClaveAcceso + "]", "RUC Proveedor: 1790011674001"} {
				if !strings.Contains(txt, quiero) {
					t.Errorf("el RIDE no trae %q", quiero)
				}
			}
		}
	}
	// El comprobante espera en el outbox para la nube, con su XML y hash.
	var payload string
	if err := c.a.Store.Read().QueryRow(`SELECT payload FROM outbox WHERE tipo = ?`, EventoComprobanteEmitido).Scan(&payload); err != nil {
		t.Fatal(err)
	}
	var ev map[string]any
	_ = json.Unmarshal([]byte(payload), &ev)
	if ev["claveAcceso"] != d.ClaveAcceso || ev["hash"] != cs[0].hash || ev["xml"] != cs[0].xml {
		t.Fatalf("evento: %v", ev["claveAcceso"])
	}

	// La siguiente venta toma el secuencial siguiente.
	orden2, _ := c.ordenEnMesa(t, tel, "factura-0002", c.mesa2, plato(c.cerveza, "1"))
	if st, out, raw := c.cobrar(orden2, c.efectivo.String(), "", "cobro-fact-2"); st != 200 || out.Documento.Codigo != "001-002-000000002" {
		t.Fatalf("segunda: %d %v", st, raw)
	}
}

// Si la factura no se puede emitir, el cobro entero se deshace y el número no se consume.
func TestSinPuntoNoSeCobraNiSeConsumeNumero(t *testing.T) {
	c := nuevaCaja(t, nocheDel25)
	c.conFacturacion(t)
	tel := c.emparejar(t, "Teléfono")
	tel.entrar(c.carlos, pinCarlos)
	c.pos.req("POST", "/v1/turnos", map[string]any{"cajaId": c.caja1, "fondoInicial": "20"})
	orden, _ := c.ordenEnMesa(t, tel, "sin-punto", c.mesa1, plato(c.cerveza, "1"))
	_ = c.a.Store.Write(context.Background(), func(tx *store.Tx) error {
		_, err := tx.Exec(`UPDATE cajas SET punto_emision_id = NULL`)
		return err
	})
	if st, _, raw := c.cobrar(orden, c.efectivo.String(), "", "cobro-sin-punto"); st != 409 || raw["code"] != "CAJA_SIN_PUNTO_EMISION" {
		t.Fatalf("sin punto: %d %v", st, raw)
	}
	var docs, pagos, secs int
	var estado string
	_ = c.a.Store.Read().QueryRow(`SELECT (SELECT count(*) FROM documentos_venta), (SELECT count(*) FROM pagos), (SELECT count(*) FROM secuenciales), (SELECT estado FROM ordenes WHERE id = ?)`, orden).
		Scan(&docs, &pagos, &secs, &estado)
	if docs != 0 || pagos != 0 || secs != 0 || estado == "CERRADA" {
		t.Fatalf("quedó algo del cobro fallido: docs %d pagos %d secuenciales %d estado %s", docs, pagos, secs, estado)
	}
}

// Cuenta dividida: una factura por cuenta, cada una con sus propios totales.
func TestFacturaPorCuentaDividida(t *testing.T) {
	c := nuevaCaja(t, nocheDel25)
	c.conFacturacion(t)
	tel := c.emparejar(t, "Teléfono")
	tel.entrar(c.carlos, pinCarlos)
	c.pos.req("POST", "/v1/turnos", map[string]any{"cajaId": c.caja1, "fondoInicial": "20"})
	orden, total := c.ordenEnMesa(t, tel, "factura-division", c.mesa1, plato(c.ceviche, "1"))
	// Ceviche entre 3 partes iguales.
	var linea string
	_ = c.a.Store.Read().QueryRow(`SELECT id FROM orden_lineas WHERE orden_id = ?`, orden).Scan(&linea)
	parte := []AsignacionIn{{LineaID: mustID(linea), Peso: 1}}
	st, d, raw := c.dividir(orden, parte, parte, parte)
	if st != 200 {
		t.Fatalf("dividir: %d %v", st, raw)
	}
	var suma money.Money
	for i, cu := range d.Cuentas {
		st, out, raw := c.cobrarCuenta(orden, cu.ID.String(), "cuenta-"+cu.ID.String())
		if st != 200 {
			t.Fatalf("cuenta %d: %d %v", i+1, st, raw)
		}
		suma = suma.Add(money.MustParse(out.Documento.Totales.Total))
	}
	cs := c.comprobantes(t)
	if len(cs) != 3 || !suma.Equal(total) {
		t.Fatalf("%d facturas que suman %s de %s", len(cs), suma, total)
	}
	for _, x := range cs {
		if !strings.Contains(x.xml, "1/3 Ceviche") {
			t.Fatalf("cada factura lleva su parte del plato: %s", x.xml)
		}
		validarXSD(t, x.xml)
	}
}
