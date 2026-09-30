package app

import (
	"context"
	"errors"
	"sort"
	"sync"
	"testing"

	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/edge-node/internal/store"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/ids"
)

// conPunto da a la caja de prueba un punto de emisión 001-002 cuyo dueño es este nodo.
func (c *cajaF4) conPunto(t *testing.T, ultimosNube string) ids.ID {
	t.Helper()
	punto := ids.New()
	if err := c.a.Store.Write(context.Background(), func(tx *store.Tx) error {
		_, err := tx.Exec(`INSERT INTO puntos_emision (id, tenant_id, local_id, codigo_establecimiento, codigo_punto, nodo_id, ultimos_secuenciales)
			VALUES (?, 't', 'l', '001', '002', (SELECT nodo_id FROM nodo WHERE id = 1), ?)`, punto.String(), ultimosNube)
		if err != nil {
			return err
		}
		_, err = tx.Exec(`UPDATE cajas SET punto_emision_id = ? WHERE id = ?`, punto.String(), c.caja1.String())
		return err
	}); err != nil {
		t.Fatal(err)
	}
	return punto
}

var errCobroFallido = errors.New("cobro que falla después de tomar el número")

// QA-03: 10 000 cobros concurrentes, el 30 % falla después de tomar su número: los números
// confirmados son exactamente 1…N, sin huecos ni duplicados.
func TestQA03SecuencialesSinHuecosNiDuplicados(t *testing.T) {
	if testing.Short() {
		t.Skip("prueba de carga")
	}
	c := nuevaCaja(t, nocheDel25)
	c.conPunto(t, "{}")
	const cobros = 10_000
	var (
		mu          sync.Mutex
		confirmados []int64
		wg          sync.WaitGroup
	)
	sem := make(chan struct{}, 64)
	for i := range cobros {
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			var n int64
			err := c.a.Store.Write(context.Background(), func(tx *store.Tx) error {
				p, err := puntoDeCaja(context.Background(), tx, c.caja1)
				if err != nil {
					return err
				}
				if n, err = siguienteSecuencial(context.Background(), tx, p.ID, "01", 1); err != nil {
					return err
				}
				if i%10 < 3 {
					return errCobroFallido
				}
				return nil
			})
			switch {
			case err == nil:
				mu.Lock()
				confirmados = append(confirmados, n)
				mu.Unlock()
			case !errors.Is(err, errCobroFallido):
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	sort.Slice(confirmados, func(i, j int) bool { return confirmados[i] < confirmados[j] })
	if len(confirmados) != cobros*7/10 {
		t.Fatalf("confirmados %d de %d", len(confirmados), cobros*7/10)
	}
	for i, n := range confirmados {
		if n != int64(i+1) {
			t.Fatalf("hueco o duplicado: en la posición %d está el %d", i, n)
		}
	}
	var ultimo int64
	_ = c.a.Store.Read().QueryRow(`SELECT ultimo FROM secuenciales WHERE tipo_comprobante = '01' AND ambiente = 1`).Scan(&ultimo)
	if ultimo != int64(len(confirmados)) {
		t.Fatalf("el contador quedó en %d y se confirmaron %d", ultimo, len(confirmados))
	}
}

func TestSecuencialesPorAmbienteYTraspaso(t *testing.T) {
	c := nuevaCaja(t, nocheDel25)
	// La nube conoce hasta el 500 de facturas en pruebas (lo numeró el nodo anterior).
	punto := c.conPunto(t, `{"01-1": 500}`)
	tomar := func(tipo string, amb int) int64 {
		t.Helper()
		var n int64
		if err := c.a.Store.Write(context.Background(), func(tx *store.Tx) error {
			var err error
			n, err = siguienteSecuencial(context.Background(), tx, punto, tipo, amb)
			return err
		}); err != nil {
			t.Fatal(err)
		}
		return n
	}
	if n := tomar("01", 1); n != 501 {
		t.Fatalf("tras un traspaso sigue desde lo que conoce la nube: %d", n)
	}
	if n := tomar("01", 2); n != 1 {
		t.Fatalf("producción numera aparte de pruebas: %d", n)
	}
	if n := tomar("04", 1); n != 1 {
		t.Fatalf("las notas de crédito numeran aparte: %d", n)
	}
	// Si el nodo ya iba más adelante que la nube, manda el nodo.
	_ = c.a.Store.Write(context.Background(), func(tx *store.Tx) error {
		_, err := tx.Exec(`UPDATE secuenciales SET ultimo = 600 WHERE tipo_comprobante = '01' AND ambiente = 1`)
		return err
	})
	if n := tomar("01", 1); n != 601 {
		t.Fatalf("el contador local más alto manda: %d", n)
	}
}

func TestPuntoDeOtroNodoNoSeNumera(t *testing.T) {
	c := nuevaCaja(t, nocheDel25)
	c.conPunto(t, "{}")
	_ = c.a.Store.Write(context.Background(), func(tx *store.Tx) error {
		_, err := tx.Exec(`UPDATE puntos_emision SET nodo_id = ?`, ids.New().String())
		return err
	})
	err := c.a.Store.Write(context.Background(), func(tx *store.Tx) error {
		_, err := puntoDeCaja(context.Background(), tx, c.caja1)
		return err
	})
	if err == nil {
		t.Fatal("un punto de otro nodo no se numera aquí")
	}
	// Sin punto asignado tampoco.
	_ = c.a.Store.Write(context.Background(), func(tx *store.Tx) error {
		_, err := tx.Exec(`UPDATE cajas SET punto_emision_id = NULL`)
		return err
	})
	err = c.a.Store.Write(context.Background(), func(tx *store.Tx) error {
		_, err := puntoDeCaja(context.Background(), tx, c.caja1)
		return err
	})
	if !errors.Is(err, errSinPunto) {
		t.Fatalf("caja sin punto: %v", err)
	}
}
