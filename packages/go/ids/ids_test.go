package ids

import (
	"bytes"
	"testing"
)

func TestNewIsV7AndOrdered(t *testing.T) {
	prev := New()
	seen := map[ID]bool{prev: true}
	for i := 0; i < 10_000; i++ {
		id := New()
		if id.Version() != 7 {
			t.Fatalf("versión %d", id.Version())
		}
		if seen[id] {
			t.Fatalf("colisión: %s", id)
		}
		seen[id] = true
		// Mismo proceso: la biblioteca garantiza monotonía dentro del mismo milisegundo.
		if bytes.Compare(prev[:], id[:]) >= 0 {
			t.Fatalf("no ordenado: %s >= %s", prev, id)
		}
		prev = id
	}
}

func TestParse(t *testing.T) {
	id := New()
	got, err := Parse(id.String())
	if err != nil || got != id {
		t.Fatalf("Parse(%s) = %s, %v", id, got, err)
	}
	if _, err := Parse("f47ac10b-58cc-4372-a567-0e02b2c3d479"); err == nil {
		t.Error("un UUID v4 debería rechazarse")
	}
	if _, err := Parse("no-es-uuid"); err == nil {
		t.Error("texto inválido debería rechazarse")
	}
}
