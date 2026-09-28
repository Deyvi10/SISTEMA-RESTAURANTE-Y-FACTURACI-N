package auditoria

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/ids"
)

func cadena(n int) []Registro {
	t0 := time.Date(2026, 9, 25, 20, 0, 0, 0, time.FixedZone("ECT", -5*3600))
	tenant, u := ids.New(), ids.New()
	var rs []Registro
	anterior := ""
	for i := range n {
		r := Registro{ID: ids.New(), Seq: int64(i + 1), TenantID: tenant, UsuarioID: &u, Accion: "LINEA_ANULADA", Entidad: "orden", EntidadID: ids.New().String(),
			Monto: "3.50", Motivo: "Se enfrió", Detalle: Compactar(map[string]any{"lineas": []int{i}}), CreatedAt: t0.Add(time.Duration(i) * time.Minute)}
		r.Sellar(anterior)
		anterior = r.Hash
		rs = append(rs, r)
	}
	return rs
}

func TestCadenaIntegraYViajaComoJSON(t *testing.T) {
	rs := cadena(5)
	if i := Verificar(rs); i != -1 {
		t.Fatalf("cadena rota en %d", i)
	}
	// Ida y vuelta por JSON (el outbox): se sigue verificando igual.
	b, _ := json.Marshal(rs)
	var vuelta []Registro
	if err := json.Unmarshal(b, &vuelta); err != nil || Verificar(vuelta) != -1 {
		t.Fatalf("tras JSON: %v", err)
	}
}

func TestDetectaManipulacion(t *testing.T) {
	alterado := cadena(5)
	alterado[2].Monto = "0.50"
	if i := Verificar(alterado); i != 2 {
		t.Fatalf("monto alterado: %d", i)
	}
	borrado := cadena(5)
	borrado = append(borrado[:2], borrado[3:]...)
	if i := Verificar(borrado); i != 2 {
		t.Fatalf("registro borrado: %d", i)
	}
	cambiados := cadena(5)
	cambiados[1], cambiados[2] = cambiados[2], cambiados[1]
	if i := Verificar(cambiados); i != 1 {
		t.Fatalf("orden cambiado: %d", i)
	}
	// Un registro reescrito con su hash recalculado igual rompe el eslabón siguiente.
	reescrito := cadena(5)
	reescrito[2].Motivo = "Otro"
	reescrito[2].Sellar(reescrito[1].Hash)
	if i := Verificar(reescrito); i != 3 {
		t.Fatalf("reescrito: %d", i)
	}
}

func TestRegistrosAnterioresALaCadena(t *testing.T) {
	rs := append([]Registro{{ID: ids.New(), Accion: "VIEJO"}, {ID: ids.New(), Accion: "VIEJO"}}, cadena(3)...)
	if i := Verificar(rs); i != -1 {
		t.Fatalf("los registros previos no deben romperla: %d", i)
	}
	// Pero un registro sin hash DESPUÉS de empezar la cadena sí es una manipulación.
	rs = append(cadena(2), Registro{ID: ids.New(), Accion: "INSERTADO"})
	if i := Verificar(rs); i != 2 {
		t.Fatalf("sin hash dentro de la cadena: %d", i)
	}
}

func TestCompactar(t *testing.T) {
	if Compactar(nil) != nil || Compactar(map[string]any{}) != nil {
		t.Fatalf("vacío debe quedar sin valor: %q %q", Compactar(nil), Compactar(map[string]any{}))
	}
	if string(Compactar(map[string]any{"b": 1, "a": "<x>"})) != `{"a":"\u003cx\u003e","b":1}` {
		t.Fatalf("no quedó en la forma de encoding/json: %s", Compactar(map[string]any{"b": 1, "a": "<x>"}))
	}
}
