package app

import (
	"context"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/edge-node/internal/store"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/ids"
)

// F5-18: copia local del XML autorizado, verificada, reimpresión con la leyenda de autorizado y
// purga a los 90 días.
func TestCopiaLocalDeAutorizados(t *testing.T) {
	c := nuevaCaja(t, nocheDel25)
	tel := c.emparejar(t, "Teléfono")
	tel.entrar(c.carlos, pinCarlos)
	c.pos.req("POST", "/v1/turnos", map[string]any{"cajaId": c.caja1, "fondoInicial": "20"})
	c.conFacturacion(t)
	orden, _ := c.ordenEnMesa(t, tel, "copia-local", c.mesa1, plato(c.ceviche, "1"))
	if st, _, raw := c.cobrar(orden, c.efectivo.String(), "", "cobro-copia"); st != 200 {
		t.Fatalf("cobro: %d %v", st, raw)
	}
	var id, clave, doc string
	_ = c.a.Store.Read().QueryRow(`SELECT id, clave_acceso, xml FROM comprobantes`).Scan(&id, &clave, &doc)
	cid, _ := ids.Parse(id)
	ctx := context.Background()
	if _, err := c.a.XMLAutorizadoLocal(ctx, cid); err == nil {
		t.Fatal("sin autorizar no hay copia")
	}
	// La nube avisó que está autorizado y entrega el XML autorizado.
	autorizado := `<?xml version="1.0" encoding="UTF-8"?><autorizacion><estado>AUTORIZADO</estado><numeroAutorizacion>` + clave +
		`</numeroAutorizacion><comprobante><![CDATA[` + doc + `]]></comprobante></autorizacion>`
	fecha := time.Date(2026, 9, 26, 3, 5, 0, 0, time.UTC)
	if err := c.a.Store.Write(ctx, func(tx *store.Tx) error {
		_, err := tx.Exec(`INSERT INTO estados_comprobante (id, clave_acceso, estado, numero_autorizacion, fecha_autorizacion) VALUES (?, ?, 'AUTORIZADO', ?, ?)`,
			id, clave, clave, fecha.Format(time.RFC3339))
		return err
	}); err != nil {
		t.Fatal(err)
	}
	otro := autorizadoNube{ID: cid, NumeroAutorizacion: strings.Repeat("9", 49), FechaAutorizacion: fecha, XML: "<otro/>"}
	if err := c.a.guardarAutorizados(ctx, []autorizadoNube{otro}, c.reloj.Now()); err != nil {
		t.Fatal(err)
	}
	if _, err := c.a.XMLAutorizadoLocal(ctx, cid); err == nil {
		t.Fatal("un XML que no es de este comprobante no se guarda")
	}
	bueno := autorizadoNube{ID: cid, NumeroAutorizacion: clave, FechaAutorizacion: fecha, XML: autorizado}
	if err := c.a.guardarAutorizados(ctx, []autorizadoNube{bueno}, c.reloj.Now()); err != nil {
		t.Fatal(err)
	}
	got, err := c.a.XMLAutorizadoLocal(ctx, cid)
	if err != nil || string(got) != autorizado {
		t.Fatalf("copia: %v", err)
	}
	var comprimido int
	_ = c.a.Store.Read().QueryRow(`SELECT length(xml_zst) FROM autorizados_locales`).Scan(&comprimido)
	if comprimido >= len(autorizado) {
		t.Fatalf("la copia va comprimida: %d de %d", comprimido, len(autorizado))
	}

	// Reimpresión: marca, leyenda de autorizado con su fecha (hora del local) y auditoría.
	if st, out := c.pos.req("POST", "/v1/caja/comprobantes/"+id+"/reimprimir", map[string]any{"cajaId": c.caja1}); st != 200 {
		t.Fatalf("reimprimir: %d %v", st, out)
	}
	esperar(t, func() bool {
		return slices.ContainsFunc(c.cocina.Textos(), func(s string) bool {
			return strings.Contains(s, "REIMPRESIÓN") && strings.Contains(s, "AUTORIZADO por el SRI el 25/09/2026 22:05") && strings.Contains(s, "No. 001-002-000000001")
		})
	})
	var auditado int
	_ = c.a.Store.Read().QueryRow(`SELECT count(*) FROM auditoria WHERE accion = 'COMPROBANTE_REIMPRESO'`).Scan(&auditado)
	if auditado != 1 {
		t.Fatalf("auditoría: %d", auditado)
	}

	// Búsqueda por fecha de emisión (hora del local).
	var delDia, otroDia []ComprobanteCaja
	_, _ = c.pos.reqLista("GET", "/v1/caja/comprobantes?fecha=2026-09-25", &delDia)
	_, _ = c.pos.reqLista("GET", "/v1/caja/comprobantes?fecha=2026-09-24", &otroDia)
	if len(delDia) != 1 || len(otroDia) != 0 {
		t.Fatalf("por fecha: %d y %d", len(delDia), len(otroDia))
	}

	// A los 90 días se purga la copia local (el original sigue en la nube).
	if n, err := c.a.purgarAutorizados(ctx, c.reloj.Now().Add(89*24*time.Hour)); err != nil || n != 0 {
		t.Fatalf("antes de 90 días: %d %v", n, err)
	}
	if n, err := c.a.purgarAutorizados(ctx, c.reloj.Now().Add(91*24*time.Hour)); err != nil || n != 1 {
		t.Fatalf("a los 91 días: %d %v", n, err)
	}
}
