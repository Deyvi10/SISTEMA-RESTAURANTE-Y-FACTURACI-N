package server_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/certificados"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/sri/firma/firmaprueba"
)

const claveP12 = "Clave-De-Prueba-2026"

func p12De(t *testing.T, ruc string, desde, hasta time.Time) []byte {
	t.Helper()
	b, err := firmaprueba.P12(firmaprueba.Opciones{Titular: "RESTAURANTE DE PRUEBA", RUC: ruc, Desde: desde, Hasta: hasta}, claveP12)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// subirP12 envía el formulario como el backoffice (multipart: p12 + clave).
func (c *cliente) subirP12(data []byte, clave string, want int) (certificados.Info, map[string]any) {
	c.e.t.Helper()
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	fw, _ := mw.CreateFormFile("p12", "firma.p12")
	_, _ = fw.Write(data)
	_ = mw.WriteField("clave", clave)
	_ = mw.Close()
	req, _ := http.NewRequest("POST", c.e.srv.URL+"/v1/facturacion/certificado", &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+c.token)
	res, err := c.http.Do(req)
	if err != nil {
		c.e.t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	raw, _ := io.ReadAll(res.Body)
	if res.StatusCode != want {
		c.e.t.Fatalf("subir p12 = %d, quería %d: %s", res.StatusCode, want, raw)
	}
	var info certificados.Info
	var m map[string]any
	_ = json.Unmarshal(raw, &info)
	_ = json.Unmarshal(raw, &m)
	return info, m
}

// F5-07 / F5-06 pasos 1 y 2: se valida el .p12 y se guarda cifrado; nunca se devuelve.
func TestSubirCertificado(t *testing.T) {
	e := newEnv(t)
	c, r := e.restaurante("1790011674001", "p12@a.ec")
	ahora := time.Now()
	c.do("GET", "/v1/facturacion/certificado", nil, 204, nil)

	// Errores claros: contraseña, vigencia, RUC de otro, archivo que no es un .p12.
	bueno := p12De(t, "1790011674001", ahora.Add(-time.Hour), ahora.AddDate(0, 0, 45))
	_, m := c.subirP12(bueno, "otra", 422)
	if !strings.Contains(m["detail"].(string), "contraseña") {
		t.Fatalf("contraseña: %v", m)
	}
	c.subirP12(p12De(t, "1790011674001", ahora.AddDate(-2, 0, 0), ahora.AddDate(-1, 0, 0)), claveP12, 422)
	_, m = c.subirP12(p12De(t, "0992339411001", ahora.Add(-time.Hour), ahora.AddDate(1, 0, 0)), claveP12, 409)
	if m["code"] != "RUC_DISTINTO" || !strings.Contains(m["detail"].(string), "0992339411001") {
		t.Fatalf("RUC de otro: %v", m)
	}
	c.subirP12([]byte("no soy un p12"), claveP12, 422)
	c.subirP12(bueno, "", 422)

	info, m := c.subirP12(bueno, claveP12, 201)
	if info.Titular != "RESTAURANTE DE PRUEBA" || info.RUC == nil || *info.RUC != "1790011674001" || info.DiasRestantes != 44 || info.Aviso != nil {
		t.Fatalf("info: %+v", info)
	}
	for k := range m {
		if strings.Contains(strings.ToLower(k), "cifrad") || strings.Contains(strings.ToLower(k), "p12") || k == "clave" || k == "dek" {
			t.Fatalf("la respuesta expone %q", k)
		}
	}
	var got certificados.Info
	c.do("GET", "/v1/facturacion/certificado", nil, 200, &got)
	if got.ID != info.ID {
		t.Fatalf("activo: %+v", got)
	}

	// Lo guardado está cifrado: ni el .p12 ni la contraseña aparecen en claro en ninguna parte.
	ctx := context.Background()
	var p12c, passc []byte
	if err := e.tdb.Admin.QueryRow(ctx, `SELECT p12_cifrado, password_cifrada FROM certificados_firma WHERE id = $1`, info.ID).Scan(&p12c, &passc); err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(p12c, bueno[:64]) || bytes.Contains(passc, []byte(claveP12)) {
		t.Fatal("el certificado quedó en claro")
	}
	var fugas int
	_ = e.tdb.Admin.QueryRow(ctx, `SELECT count(*) FROM auditoria WHERE tenant_id = $1 AND (coalesce(antes::text, '') || coalesce(despues::text, '')) LIKE '%' || $2 || '%'`,
		r.TenantID, claveP12).Scan(&fugas)
	if fugas != 0 {
		t.Fatal("la contraseña apareció en la auditoría")
	}

	// Re-subir deja uno solo activo; el anterior queda (no se borra) y lo cifrado no cambia.
	nuevo, _ := c.subirP12(p12De(t, "1790011674001", ahora.Add(-time.Hour), ahora.AddDate(2, 0, 0)), claveP12, 201)
	var activos, total int
	_ = e.tdb.Admin.QueryRow(ctx, `SELECT count(*) FILTER (WHERE activo), count(*) FROM certificados_firma WHERE tenant_id = $1`, r.TenantID).Scan(&activos, &total)
	if activos != 1 || total != 2 || nuevo.ID == info.ID {
		t.Fatalf("activos %d de %d", activos, total)
	}
	if _, err := e.tdb.Admin.Exec(ctx, `UPDATE certificados_firma SET p12_cifrado = '\x00' WHERE id = $1`, info.ID); err == nil {
		t.Fatal("lo cifrado no se reescribe")
	}
	if _, err := e.tdb.Admin.Exec(ctx, `UPDATE certificados_firma SET activo = true WHERE id = $1`, info.ID); err == nil {
		t.Fatal("un certificado reemplazado no se reactiva")
	}
	if _, err := e.tdb.Admin.Exec(ctx, `DELETE FROM certificados_firma WHERE id = $1`, info.ID); err == nil {
		t.Fatal("un certificado no se borra")
	}
	var auditados int
	_ = e.tdb.Admin.QueryRow(ctx, `SELECT count(*) FROM auditoria WHERE tenant_id = $1 AND accion = 'CERTIFICADO_SUBIDO'`, r.TenantID).Scan(&auditados)
	if auditados != 2 {
		t.Fatalf("auditoría: %d", auditados)
	}

	// Otro restaurante no ve el certificado (RLS).
	otro, _ := e.restaurante("0992339411001", "p12@b.ec")
	otro.do("GET", "/v1/facturacion/certificado", nil, 204, nil)
}
