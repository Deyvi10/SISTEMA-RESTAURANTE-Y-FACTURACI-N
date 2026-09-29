package app

import (
	"context"
	"fmt"
	"math/rand/v2"
	"regexp"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/edge-node/internal/store"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/ids"
)

// percentil de una muestra de duraciones (p en 0..100).
func percentil(xs []time.Duration, p int) time.Duration {
	if len(xs) == 0 {
		return 0
	}
	s := slices.Clone(xs)
	slices.Sort(s)
	i := (len(s)*p + 99) / 100
	return s[max(0, min(len(s)-1, i-1))]
}

// Restaurante lleno: 20 teléfonos de meseros, 40 mesas, cada mesa recibe 5 rondas de
// platos (cocina y bar, con observaciones) mientras la caja cobra las mesas que terminan.
// Un tercio de los envíos se repite con la misma clave (la señal se cortó antes de la
// respuesta). Exige: ningún error, ninguna comanda duplicada, cada comanda impresa una
// sola vez en su estación con su nota, y tiempos de respuesta de una caja en hora pico.
func TestRestauranteLleno(t *testing.T) {
	if testing.Short() {
		t.Skip("prueba de carga")
	}
	c := nuevaCaja(t, nocheDel25)
	const telefonos, mesasPorTelefono, rondas = 20, 2, 5
	ctx := context.Background()

	// 40 mesas en el mismo salón.
	var zona string
	if err := c.a.Store.Read().QueryRow(`SELECT zona_id FROM mesas LIMIT 1`).Scan(&zona); err != nil {
		t.Fatal(err)
	}
	mesas := make([]ids.ID, telefonos*mesasPorTelefono)
	if err := c.a.Store.Write(ctx, func(tx *store.Tx) error {
		for i := range mesas {
			mesas[i] = ids.New()
			if _, err := tx.Exec(`INSERT INTO mesas (id, tenant_id, local_id, zona_id, nombre, capacidad) VALUES (?, 't', 'l', ?, ?, 4)`, mesas[i].String(), zona, fmt.Sprintf("Salón %d", i+1)); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if st, out := c.pos.req("POST", "/v1/turnos", map[string]any{"cajaId": c.caja1, "fondoInicial": "50"}); st != 200 {
		t.Fatalf("turno: %d %v", st, out)
	}

	gente := []struct {
		id  ids.ID
		pin string
	}{{c.carlos, pinCarlos}, {c.ana, pinAna}}
	tels := make([]*telefono, telefonos)
	for i := range tels {
		tels[i] = c.emparejar(t, fmt.Sprintf("Teléfono %d", i+1))
		if st, out := tels[i].entrar(gente[i%2].id, gente[i%2].pin); st != 200 {
			t.Fatalf("PIN en el teléfono %d: %d %v", i+1, st, out)
		}
	}

	var (
		mu        sync.Mutex
		latEnvio  []time.Duration
		latCobro  []time.Duration
		porClave  = map[string]float64{} // clave → número de comanda
		notas     = map[string]bool{}    // nota esperada en la barra
		errores   []string
		porCobrar = make(chan string, len(mesas))
	)
	fallo := func(f string, a ...any) {
		mu.Lock()
		errores = append(errores, fmt.Sprintf(f, a...))
		mu.Unlock()
	}
	enviar := func(p *telefono, clave string, mesa ids.ID, nota string) (string, bool) {
		inicio := time.Now()
		cerveza := plato(c.cerveza, "2")
		cerveza["nota"] = nota
		st, out := p.enviar(clave, mesa, plato(c.ceviche, "1"), cerveza)
		d := time.Since(inicio)
		if st != 200 {
			fallo("%s: %d %v", clave, st, out)
			return "", false
		}
		mu.Lock()
		defer mu.Unlock()
		latEnvio = append(latEnvio, d)
		n := out["comandaNumero"].(float64)
		if previo, ok := porClave[clave]; ok && previo != n {
			errores = append(errores, fmt.Sprintf("la clave %s dio las comandas #%v y #%v", clave, previo, n))
		}
		porClave[clave] = n
		return out["orden"].(map[string]any)["id"].(string), true
	}

	// La caja cobra en efectivo exacto cada mesa que termina, mientras siguen llegando pedidos.
	// Cada mesa pasa del límite de consumidor final: se cobra con el comprador identificado.
	comprador := map[string]any{"identificacion": "1710034065", "razonSocial": "Cliente de Prueba", "email": "cliente@example.com"}
	var cajaWG sync.WaitGroup
	cobradas := 0
	cajaWG.Go(func() {
		for orden := range porCobrar {
			inicio := time.Now()
			st, _, raw := c.cobrarA(orden, "cobro-"+orden, comprador)
			d := time.Since(inicio)
			if st != 200 {
				fallo("cobrar %s: %d %v", orden, st, raw)
				continue
			}
			mu.Lock()
			latCobro = append(latCobro, d)
			cobradas++
			mu.Unlock()
		}
	})

	inicio := time.Now()
	var wg sync.WaitGroup
	for i, p := range tels {
		wg.Go(func() {
			for k := range mesasPorTelefono {
				mesa := mesas[i*mesasPorTelefono+k]
				var orden string
				for r := range rondas {
					if st, out := p.req("POST", "/v1/mesas/"+mesa.String()+"/bloqueo", nil); st != 200 {
						fallo("bloqueo tel %d: %d %v", i, st, out)
						return
					}
					clave := fmt.Sprintf("t%02d-m%d-r%d", i, k, r)
					nota := fmt.Sprintf("sin hielo %s", clave)
					mu.Lock()
					notas[nota] = true
					mu.Unlock()
					o, ok := enviar(p, clave, mesa, nota)
					if !ok {
						return
					}
					orden = o
					if rand.IntN(3) == 0 { // la señal se cortó: la app repite con la misma clave
						enviar(p, clave, mesa, nota)
					}
				}
				porCobrar <- orden
			}
		})
	}
	wg.Wait()
	close(porCobrar)
	cajaWG.Wait()
	duracion := time.Since(inicio)
	if len(errores) > 0 {
		t.Fatalf("%d errores; el primero: %s", len(errores), errores[0])
	}
	envios := telefonos * mesasPorTelefono * rondas
	if len(porClave) != envios || cobradas != len(mesas) {
		t.Fatalf("aceptados %d de %d envíos; cobradas %d de %d mesas", len(porClave), envios, cobradas, len(mesas))
	}

	// Cada envío dio una comanda en cocina y otra en la barra, impresas una sola vez.
	numero := regexp.MustCompile(`Comanda #(\d+)`)
	contar := func(textos []string) map[string]int {
		n := map[string]int{}
		for _, x := range textos {
			if m := numero.FindStringSubmatch(x); m != nil {
				n[m[1]]++
			}
		}
		return n
	}
	var cocina, bar map[string]int
	limite := time.Now().Add(30 * time.Second)
	for {
		cocina, bar = contar(c.cocina.Textos()), contar(c.bar.Textos())
		if (len(cocina) >= envios && len(bar) >= envios) || time.Now().After(limite) {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	for nombre, impresas := range map[string]map[string]int{"cocina": cocina, "barra": bar} {
		if len(impresas) != envios {
			t.Fatalf("%s: %d comandas impresas de %d", nombre, len(impresas), envios)
		}
		for n, veces := range impresas {
			if veces != 1 {
				t.Fatalf("%s: la comanda #%s salió %d veces", nombre, n, veces)
			}
		}
	}
	todoBar := strings.Join(c.bar.Textos(), "\n")
	for nota := range notas {
		if !strings.Contains(todoBar, nota) {
			t.Fatalf("la observación %q no salió en la barra", nota)
		}
	}

	p50, p95, p99 := percentil(latEnvio, 50), percentil(latEnvio, 95), percentil(latEnvio, 99)
	t.Logf("%d teléfonos, %d envíos (+%d repetidos) y %d cobros en %s", telefonos, envios, len(latEnvio)-envios, cobradas, duracion.Round(time.Millisecond))
	t.Logf("envío de comanda: p50 %s · p95 %s · p99 %s · máx %s", p50, p95, p99, percentil(latEnvio, 100))
	t.Logf("cobro: p50 %s · p95 %s · máx %s", percentil(latCobro, 50), percentil(latCobro, 95), percentil(latCobro, 100))
	// Objetivo de hora pico: p95 del envío bajo 300 ms (medido: ~70 ms con fsync en cada
	// confirmación). Con -race solo se exige la corrección.
	if !conRace && p95 > 300*time.Millisecond {
		t.Errorf("p95 del envío %s supera 300 ms", p95)
	}
}
