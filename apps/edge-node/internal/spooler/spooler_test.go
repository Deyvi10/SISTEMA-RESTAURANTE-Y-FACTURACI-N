package spooler

import "testing"

func TestEstadoDe(t *testing.T) {
	if st, _ := EstadoDe(statusPaperOut, 0); !st.PaperOut || st.OK() {
		t.Fatal("sin papel")
	}
	if st, _ := EstadoDe(statusDoorOpen, 0); !st.CoverOpen {
		t.Fatal("tapa abierta")
	}
	// Windows suele marcar «offline» a las USB en reposo: no debe frenar el envío.
	if st, _ := EstadoDe(statusOffline, attrWorkOffline); !st.OK() {
		t.Fatal("offline no debe bloquear")
	}
	for s, want := range map[uint32]string{0: "OK", statusPaperOut: "SIN_PAPEL", statusDoorOpen: "TAPA_ABIERTA", statusPaperJam: "ERROR", statusOffline: "SIN_CONEXION"} {
		if got := EtiquetaEstado(s, 0); got != want {
			t.Errorf("EtiquetaEstado(%#x) = %s, quiero %s", s, got, want)
		}
	}
}

func TestVirtualYAncho(t *testing.T) {
	for _, c := range []struct {
		nombre, puerto, driver string
		virtual                bool
	}{
		{"Microsoft Print to PDF", "PORTPROMPT:", "Microsoft Print To PDF", true},
		{"Microsoft XPS Document Writer", "PORTPROMPT:", "Microsoft XPS Document Writer v4", true},
		{"Fax", "SHRFAX:", "Microsoft Shared Fax Driver", true},
		{"OneNote (Desktop)", "nul:", "Send to Microsoft OneNote 16 Driver", true},
		{"EPSON TM-T20III Receipt", "ESDPRT001", "EPSON TM-T20III Receipt", false},
		{"POS-58", "USB001", "Generic / Text Only", false},
		{"Cocina", "IP_192.168.1.50", "Generic / Text Only", false},
	} {
		if Virtual(c.nombre, c.puerto, c.driver) != c.virtual {
			t.Errorf("Virtual(%q) != %v", c.nombre, c.virtual)
		}
	}
	for n, want := range map[string]int{"POS-58": 58, "XP-58IIH": 58, "Impresora 58mm": 58, "EPSON TM-T20III": 80, "POS-80C": 80, "XP-580": 80} {
		if got := AnchoSugerido(n, ""); got != want {
			t.Errorf("AnchoSugerido(%q) = %d", n, got)
		}
	}
	for p, want := range map[string]string{"IP_192.168.1.50": "192.168.1.50", "192.168.1.60_1": "192.168.1.60", "USB001": "", "WSD-abc": ""} {
		if got := IPDePuerto(p); got != want {
			t.Errorf("IPDePuerto(%q) = %q", p, got)
		}
	}
}
