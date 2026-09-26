package app

import (
	"context"
	"fmt"
	"math/rand/v2"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/ids"
)

// F3-15: ocho teléfonos contra un nodo sin internet (la nube apunta a una dirección
// inválida). Cada uno toma mesas al azar, envía y repite cada envío como si se hubiera
// cortado la red; quien no consigue la mesa guarda el pedido en su cola y lo reintenta
// después con la misma clave, como la app. Al final: 0 platos duplicados, 0 colisiones
// de mesa y una sola orden abierta por mesa.
func TestOchoTelefonosSinInternet(t *testing.T) {
	if testing.Short() {
		t.Skip("prueba de carga")
	}
	s := nuevoSalon(t)
	gente := []struct {
		id  ids.ID
		pin string
	}{{s.carlos, pinCarlos}, {s.ana, pinAna}, {s.luis, pinLuis}}
	mesas := []ids.ID{s.mesa1, s.mesa2, s.mesa3}
	const telefonos, rondas = 8, 30

	tels := make([]*telefono, telefonos)
	for i := range tels {
		tels[i] = s.emparejar(t, fmt.Sprintf("Teléfono %d", i+1))
		if st, out := tels[i].entrar(gente[i%len(gente)].id, gente[i%len(gente)].pin); st != 200 {
			t.Fatalf("PIN en el teléfono %d: %d %v", i+1, st, out)
		}
	}

	type envio struct {
		clave string
		mesa  ids.ID
	}
	var (
		editando   [3]atomic.Int32 // teléfonos con la mesa bloqueada a la vez (debe ser ≤ 1)
		colisiones atomic.Int32
		aceptadas  sync.Map // clave → número de comanda
		mu         sync.Mutex
		colas      = make([][]envio, telefonos)
	)
	aceptar := func(p *telefono, e envio) bool {
		st, out := p.enviar(e.clave, e.mesa, plato(s.hamburguesa, "1", s.terMedio))
		if st != 200 {
			if st != 409 {
				t.Errorf("enviar %s: %d %v", e.clave, st, out)
			}
			return false
		}
		n := int(out["comandaNumero"].(float64))
		if previo, ok := aceptadas.LoadOrStore(e.clave, n); ok && previo.(int) != n {
			t.Errorf("la clave %s dio las comandas #%d y #%d", e.clave, previo, n)
		}
		return true
	}

	var wg sync.WaitGroup
	for i, p := range tels {
		wg.Go(func() {
			for r := range rondas {
				m := rand.IntN(len(mesas))
				e := envio{clave: fmt.Sprintf("tel%d-ronda%02d", i, r), mesa: mesas[m]}
				st, _ := p.req("POST", "/v1/mesas/"+mesas[m].String()+"/bloqueo", nil)
				if st != 200 {
					mu.Lock()
					colas[i] = append(colas[i], e) // «Pendiente de envío»
					mu.Unlock()
					continue
				}
				if editando[m].Add(1) > 1 {
					colisiones.Add(1)
				}
				editando[m].Add(-1) // antes de enviar: el nodo libera la mesa al aceptar
				if !aceptar(p, e) {
					mu.Lock()
					colas[i] = append(colas[i], e)
					mu.Unlock()
					continue
				}
				// Se cortó la red antes de la respuesta: la app repite la misma clave.
				if rand.IntN(3) == 0 {
					aceptar(p, e)
				}
			}
		})
	}
	wg.Wait()

	// Vuelve la calma: cada teléfono vacía su cola (la mesa ya no está bloqueada).
	for i, p := range tels {
		for _, e := range colas[i] {
			if !aceptar(p, e) {
				t.Errorf("el pendiente %s no entró al vaciar la cola", e.clave)
			}
		}
	}

	claves := 0
	aceptadas.Range(func(_, _ any) bool { claves++; return true })
	if claves != telefonos*rondas {
		t.Fatalf("aceptadas %d de %d envíos", claves, telefonos*rondas)
	}
	if colisiones.Load() != 0 {
		t.Fatalf("%d colisiones de mesa", colisiones.Load())
	}

	db := s.a.Store.Read()
	ctx := context.Background()
	var comandas, distintas, lineas, abiertas, mesasAbiertas int
	if err := db.QueryRowContext(ctx, `SELECT count(*), count(DISTINCT idempotency_key) FROM comandas WHERE orden_id IS NOT NULL`).Scan(&comandas, &distintas); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM orden_lineas`).Scan(&lineas); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx, `SELECT count(*), count(DISTINCT mesa_id) FROM ordenes WHERE estado IN ('ABIERTA','PRECUENTA')`).Scan(&abiertas, &mesasAbiertas); err != nil {
		t.Fatal(err)
	}
	if comandas != telefonos*rondas || distintas != comandas {
		t.Errorf("comandas %d (distintas %d), quiero %d sin repetir", comandas, distintas, telefonos*rondas)
	}
	if lineas != telefonos*rondas {
		t.Errorf("platos %d, quiero %d: hay duplicados o perdidos", lineas, telefonos*rondas)
	}
	if abiertas != mesasAbiertas || abiertas > len(mesas) {
		t.Errorf("órdenes abiertas %d en %d mesas: dos órdenes en una mesa", abiertas, mesasAbiertas)
	}
	t.Logf("%d teléfonos × %d rondas: %d comandas, %d platos, %d órdenes abiertas, 0 colisiones", telefonos, rondas, comandas, lineas, abiertas)
}
