// Package clock hace inyectable el tiempo (docs/11 §2.3).
//
// El dominio nunca llama a time.Now directamente: recibe un Clock. Así las pruebas
// controlan vencimientos como el bloqueo de mesa (45 s), las reservas de cupo (10 min)
// o los reintentos fiscales sin esperar ni depender del reloj real.
package clock

import (
	"sync"
	"time"
)

// Clock entrega la hora actual.
type Clock interface {
	Now() time.Time
}

// Real usa el reloj del sistema, en UTC (la zona del local solo se usa al presentar).
type Real struct{}

func (Real) Now() time.Time { return time.Now().UTC() }

// Fake es un reloj manual para pruebas. Es seguro para uso concurrente.
type Fake struct {
	mu  sync.Mutex
	now time.Time
}

// NewFake crea un reloj detenido en t.
func NewFake(t time.Time) *Fake { return &Fake{now: t.UTC()} }

func (f *Fake) Now() time.Time {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.now
}

// Advance adelanta el reloj.
func (f *Fake) Advance(d time.Duration) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.now = f.now.Add(d)
}

// Set fija el reloj en t.
func (f *Fake) Set(t time.Time) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.now = t.UTC()
}

// Guayaquil es la zona horaria de operación (RNF-32). Se carga una vez; si el sistema
// no tiene la base de zonas horarias, se usa el desfase fijo UTC-5 (Ecuador no usa horario de verano).
var Guayaquil = func() *time.Location {
	if loc, err := time.LoadLocation("America/Guayaquil"); err == nil {
		return loc
	}
	return time.FixedZone("ECT", -5*60*60)
}()
