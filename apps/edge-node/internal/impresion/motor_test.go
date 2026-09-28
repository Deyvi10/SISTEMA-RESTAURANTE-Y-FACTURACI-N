package impresion

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net"
	"path/filepath"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/edge-node/internal/impresion/termica"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/edge-node/internal/store"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/escpos"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/ids"
)

type entorno struct {
	m        *Motor
	st       *store.Store
	mu       sync.Mutex
	estados  []Estado
	impresos []Trabajo
}

func nuevoMotor(t *testing.T) *entorno {
	t.Helper()
	st, err := store.Open(context.Background(), filepath.Join(t.TempDir(), "n.db"))
	if err != nil {
		t.Fatal(err)
	}
	e := &entorno{st: st}
	e.m = &Motor{DB: st, T: TCP{}, Log: slog.New(slog.NewTextHandler(io.Discard, nil)), Now: time.Now, Sondeo: 100 * time.Millisecond, Reintento: 50 * time.Millisecond,
		AlCambiarEstado: func(x Estado) { e.mu.Lock(); e.estados = append(e.estados, x); e.mu.Unlock() },
		AlImprimir:      func(x Trabajo) { e.mu.Lock(); e.impresos = append(e.impresos, x); e.mu.Unlock() },
	}
	ctx, cancel := context.WithCancel(context.Background())
	e.m.Iniciar(ctx)
	t.Cleanup(func() { cancel(); e.m.Esperar(); _ = st.Close() })
	return e
}

func (e *entorno) encolar(t *testing.T, imp Impresora, texto string) ids.ID {
	t.Helper()
	id := ids.New()
	if err := e.st.Write(context.Background(), func(tx *store.Tx) error {
		return Encolar(context.Background(), tx, Trabajo{ID: id, ImpresoraID: imp.ID, Tipo: "COMANDA"}, nil, []byte(texto), nil, time.Now())
	}); err != nil {
		t.Fatal(err)
	}
	e.m.Despertar(imp.ID)
	return id
}

func (e *entorno) ultimoEstado(id ids.ID) string {
	return e.m.Estado(id).Estado
}

func esperar(t *testing.T, cond func() bool) {
	t.Helper()
	for range 300 {
		if cond() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("la condición no se cumplió en 3 s")
}

func imp(x *termica.Termica, nombre string) Impresora {
	host, port, _ := net.SplitHostPort(x.Addr())
	p, _ := strconv.Atoi(port)
	return Impresora{ID: ids.New(), Nombre: nombre, Host: host, Puerto: p, Ancho: escpos.Paper80}
}

func TestImprimeEnOrdenYEnParalelo(t *testing.T) {
	e := nuevoMotor(t)
	cocina, bar := termica.Nueva(t), termica.Nueva(t)
	ic, ib := imp(cocina, "Cocina"), imp(bar, "Bar")
	e.m.Sincronizar([]Impresora{ic, ib})
	for i := range 5 {
		e.encolar(t, ic, "COCINA-"+strconv.Itoa(i))
		e.encolar(t, ib, "BAR-"+strconv.Itoa(i))
	}
	esperar(t, func() bool { return len(cocina.Trabajos()) == 5 && len(bar.Trabajos()) == 5 })
	for i, j := range cocina.Trabajos() {
		if string(j) != "COCINA-"+strconv.Itoa(i) {
			t.Fatalf("orden en cocina: %q en la posición %d", j, i)
		}
	}
	if e.ultimoEstado(ic.ID) != EstadoOK {
		t.Fatalf("estado = %s", e.ultimoEstado(ic.ID))
	}
	// La impresora recibe los bytes un instante antes de que el motor marque el trabajo.
	esperar(t, func() bool {
		var pendientes int
		_ = e.st.Read().QueryRow(`SELECT count(*) FROM trabajos_impresion WHERE estado = 'PENDIENTE'`).Scan(&pendientes)
		return pendientes == 0
	})
}

// RF-02-04.3 y DoD de F2: al desconectar una impresora el trabajo se conserva y se imprime
// al volver, sin frenar a la otra.
func TestImpresoraCaidaNoPierdeNiBloquea(t *testing.T) {
	e := nuevoMotor(t)
	cocina, bar := termica.Nueva(t), termica.Nueva(t)
	ic, ib := imp(cocina, "Cocina"), imp(bar, "Bar")
	e.m.Sincronizar([]Impresora{ic, ib})
	cocina.Apagar()
	e.encolar(t, ic, "PLATO")
	e.encolar(t, ib, "CERVEZA")
	esperar(t, func() bool { return len(bar.Trabajos()) == 1 })
	esperar(t, func() bool { return e.ultimoEstado(ic.ID) == EstadoSinConexion })
	if len(cocina.Trabajos()) != 0 {
		t.Fatal("imprimió estando apagada")
	}
	cocina.Encender()
	esperar(t, func() bool { return len(cocina.Trabajos()) == 1 })
	esperar(t, func() bool { return e.ultimoEstado(ic.ID) == EstadoOK })
	e.mu.Lock()
	defer e.mu.Unlock()
	var vistos []string
	for _, x := range e.estados {
		if x.ID == ic.ID {
			vistos = append(vistos, x.Estado)
		}
	}
	if len(vistos) < 2 || vistos[len(vistos)-1] != EstadoOK {
		t.Fatalf("avisos de estado: %v", vistos)
	}
}

func TestSinPapelEsperaYReanuda(t *testing.T) {
	e := nuevoMotor(t)
	x := termica.Nueva(t)
	imp := imp(x, "Caja")
	x.Fijar(escpos.Status{PaperOut: true})
	e.m.Sincronizar([]Impresora{imp})
	e.encolar(t, imp, "PRECUENTA")
	esperar(t, func() bool { return e.ultimoEstado(imp.ID) == EstadoSinPapel })
	time.Sleep(250 * time.Millisecond)
	if len(x.Trabajos()) != 0 {
		t.Fatal("envió el trabajo sin papel: se habría perdido en el buffer")
	}
	if m := e.m.Estado(imp.ID).Motivo; m == "" {
		t.Fatal("sin motivo para el cajero")
	}
	x.Fijar(escpos.Status{PaperNearEnd: true})
	esperar(t, func() bool { return len(x.Trabajos()) == 1 })
	if e.ultimoEstado(imp.ID) != EstadoPocoPapel {
		t.Fatalf("estado = %s", e.ultimoEstado(imp.ID))
	}
	esperar(t, func() bool {
		estados := e.m.Estados(context.Background())
		return len(estados) == 1 && estados[0].Cola == 0
	})
}

func TestImpresoraGenericaSinEstado(t *testing.T) {
	e := nuevoMotor(t)
	x := termica.Nueva(t)
	x.SinEstado()
	imp := imp(x, "Genérica")
	e.m.Sincronizar([]Impresora{imp})
	e.encolar(t, imp, "HOLA")
	esperar(t, func() bool { return len(x.Trabajos()) == 1 && bytes.Equal(x.Trabajos()[0], []byte("HOLA")) })
}

// Quitar una impresora de la configuración detiene su proceso; su cola queda guardada.
func TestSincronizarQuitaImpresora(t *testing.T) {
	e := nuevoMotor(t)
	x := termica.Nueva(t)
	imp := imp(x, "Vieja")
	e.m.Sincronizar([]Impresora{imp})
	e.m.Sincronizar(nil)
	e.encolar(t, imp, "X")
	time.Sleep(200 * time.Millisecond)
	if len(x.Trabajos()) != 0 {
		t.Fatal("imprimió una impresora retirada")
	}
	if got := e.m.Estado(imp.ID).Estado; got != EstadoDesconocido {
		t.Fatalf("estado = %s", got)
	}
}
