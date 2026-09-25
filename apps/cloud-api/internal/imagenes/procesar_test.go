package imagenes

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/jpeg"
	"testing"

	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/platform/apperr"
)

func jpegCon(w, h, orientacion int) []byte {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.Set(x, y, color.RGBA{200, uint8(x % 255), 40, 255}) //nolint:gosec // prueba
		}
	}
	var b bytes.Buffer
	_ = jpeg.Encode(&b, img, &jpeg.Options{Quality: 80})
	data := b.Bytes()
	if orientacion == 0 {
		return data
	}
	// APP1 con un TIFF mínimo: una entrada IFD 0x0112 (Orientation).
	tiff := []byte{'I', 'I', 42, 0, 8, 0, 0, 0, 1, 0}
	e := make([]byte, 12)
	binary.LittleEndian.PutUint16(e[0:], 0x0112)
	binary.LittleEndian.PutUint16(e[2:], 3)
	binary.LittleEndian.PutUint32(e[4:], 1)
	binary.LittleEndian.PutUint16(e[8:], uint16(orientacion)) //nolint:gosec // prueba
	tiff = append(tiff, e...)
	tiff = append(tiff, 0, 0, 0, 0)
	seg := append([]byte("Exif\x00\x00"), tiff...)
	app1 := []byte{0xFF, 0xE1, 0, 0}
	binary.BigEndian.PutUint16(app1[2:], uint16(len(seg)+2)) //nolint:gosec // prueba
	app1 = append(app1, seg...)
	return append(append([]byte{0xFF, 0xD8}, app1...), data[2:]...)
}

func tam(t *testing.T, webpData []byte) (int, int) {
	t.Helper()
	cfg, format, err := image.DecodeConfig(bytes.NewReader(webpData))
	if err != nil || format != "webp" {
		t.Fatalf("salida no es webp: %v %s", err, format)
	}
	return cfg.Width, cfg.Height
}

func TestProcesarTamanos(t *testing.T) {
	out, err := Procesar(jpegCon(1600, 1000, 0))
	if err != nil {
		t.Fatal(err)
	}
	if w, h := tam(t, out["sm"]); w != 160 || h != 160 {
		t.Errorf("sm = %dx%d", w, h)
	}
	if w, h := tam(t, out["md"]); w != 480 || h != 480 {
		t.Errorf("md = %dx%d", w, h)
	}
	if w, h := tam(t, out["lg"]); w != 1200 || h != 750 {
		t.Errorf("lg = %dx%d (debe conservar la proporción)", w, h)
	}
	for k, v := range out {
		if bytes.Contains(v, []byte("Exif")) {
			t.Errorf("%s conserva metadatos EXIF", k)
		}
	}
}

// Una foto de celular en vertical llega como 1600×1000 con Orientation=6: debe quedar derecha.
func TestOrientacionEXIF(t *testing.T) {
	data := jpegCon(1600, 1000, 6)
	if o := orientacionEXIF(data); o != 6 {
		t.Fatalf("orientación leída = %d", o)
	}
	out, err := Procesar(data)
	if err != nil {
		t.Fatal(err)
	}
	if w, h := tam(t, out["lg"]); w != 750 || h != 1200 {
		t.Fatalf("lg = %dx%d, se esperaba 750x1200 (rotada)", w, h)
	}
	if o := orientacionEXIF([]byte("basura")); o != 1 {
		t.Fatal("datos inválidos deben dar orientación normal")
	}
}

func TestRechazos(t *testing.T) {
	casos := map[string][]byte{
		"IMAGEN_INVALIDA": []byte("<svg xmlns='http://www.w3.org/2000/svg'></svg>"),
		"IMAGEN_PEQUENA":  jpegCon(120, 120, 0),
		"IMAGEN_GRANDE":   make([]byte, MaxBytes+1),
	}
	for code, data := range casos {
		_, err := Procesar(data)
		if e, ok := apperr.As(err); !ok || e.Code != code {
			t.Errorf("%s: obtuvo %v", code, err)
		}
	}
}

func TestClaveValida(t *testing.T) {
	a := "0192a000-0000-7000-8000-00000000000a"
	if !ClaveValida(mustID(a), "t/"+a+"/0192a000-0000-7000-8000-0000000000ff") {
		t.Error("foto propia rechazada")
	}
	if ClaveValida(mustID(a), "t/0192a000-0000-7000-8000-00000000000b/0192a000-0000-7000-8000-0000000000ff") {
		t.Error("foto de otro tenant aceptada")
	}
	if !ClaveValida(mustID(a), "biblioteca/ceviche-camaron") || ClaveValida(mustID(a), "biblioteca/../t/x") {
		t.Error("galería mal validada")
	}
}

// La galería embebida tiene manifiesto y foto para cada entrada, todas con licencia libre.
func TestGaleriaEmbebida(t *testing.T) {
	fotos, err := manifiesto()
	if err != nil || len(fotos) < 20 {
		t.Fatalf("galería: %d fotos, %v", len(fotos), err)
	}
	for _, f := range fotos {
		if f.Licencia != "CC0" && f.Licencia != "PDM" {
			t.Errorf("%s con licencia %q: solo se aceptan CC0 o dominio público", f.Slug, f.Licencia)
		}
		if _, err := bibliotecaFS.ReadFile("biblioteca/" + f.Slug + ".jpg"); err != nil {
			t.Errorf("%s sin archivo: %v", f.Slug, err)
		}
	}
	s := &Service{Store: &Memory{}, PublicURL: "http://x"}
	n, err := s.SembrarGaleria(t.Context())
	if err != nil || n != len(fotos) {
		t.Fatalf("sembrar: %d, %v", n, err)
	}
	if n, _ := s.SembrarGaleria(t.Context()); n != 0 {
		t.Fatal("sembrar dos veces no debe volver a subir")
	}
}

func mustID(s string) [16]byte {
	var id [16]byte
	h := []byte{}
	for _, c := range s {
		if c != '-' {
			h = append(h, byte(c))
		}
	}
	for i := range 16 {
		var v byte
		for _, c := range h[i*2 : i*2+2] {
			v <<= 4
			switch {
			case c >= '0' && c <= '9':
				v |= c - '0'
			default:
				v |= c - 'a' + 10
			}
		}
		id[i] = v
	}
	return id
}
