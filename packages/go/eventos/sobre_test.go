package eventos

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/ids"
)

func TestSobreIdaYVuelta(t *testing.T) {
	mesa, mesero := ids.New(), ids.New()
	exp := time.Date(2026, 9, 25, 21, 15, 48, 0, time.UTC)
	s, err := Nuevo(TableLocked{TableID: mesa, ByUserID: mesero, ByName: "Carlos", ExpiresAt: exp}, exp)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(s)
	var vuelta Sobre
	if err := json.Unmarshal(raw, &vuelta); err != nil {
		t.Fatal(err)
	}
	if vuelta.V != 1 || vuelta.Type != "table.locked" || vuelta.ID.Version() != 7 {
		t.Fatalf("sobre: %+v", vuelta)
	}
	got, err := Abrir[TableLocked](vuelta)
	if err != nil || got.TableID != mesa || got.ByName != "Carlos" || !got.ExpiresAt.Equal(exp) {
		t.Fatalf("Abrir = %+v, %v", got, err)
	}
	// Los campos opcionales no viajan vacíos.
	if strings.Contains(string(vuelta.Data), "device_id") {
		t.Fatalf("un opcional nil viajó: %s", vuelta.Data)
	}
	if _, err := Abrir[TableUnlocked](vuelta); err == nil {
		t.Fatal("Abrir aceptó un tipo distinto")
	}
}
