package clock

import (
	"testing"
	"time"
)

func TestFake(t *testing.T) {
	start := time.Date(2026, 9, 24, 21, 15, 0, 0, time.UTC)
	c := NewFake(start)
	c.Advance(45 * time.Second)
	if got := c.Now().Sub(start); got != 45*time.Second {
		t.Fatalf("avance = %s", got)
	}
	var _ Clock = c
	var _ Clock = Real{}
}

func TestGuayaquilOffset(t *testing.T) {
	// 02:00 UTC del 25 es 21:00 del 24 en Ecuador: define la fecha de emisión.
	utc := time.Date(2026, 9, 25, 2, 0, 0, 0, time.UTC)
	local := utc.In(Guayaquil)
	if local.Day() != 24 || local.Hour() != 21 {
		t.Fatalf("hora local = %s", local)
	}
}
