// Package imagenes procesa y guarda las fotos del menú y los avatares (F1-13, docs/06 §6):
// valida el tipo real, corrige la orientación de las fotos de celular, elimina metadatos
// (EXIF puede traer la ubicación GPS) y recodifica a WebP en varios tamaños.
package imagenes

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/draw"
	_ "image/gif" // decodificadores registrados
	_ "image/jpeg"
	_ "image/png"
	"net/http"

	"github.com/gen2brain/webp"
	xdraw "golang.org/x/image/draw"
	_ "golang.org/x/image/webp"

	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/platform/apperr"
)

const (
	MaxBytes  = 10 << 20 // 10 MB: una foto de celular moderno
	maxPixels = 40_000_000
	minLado   = 200
)

// Tamaño de salida. Los cuadrados (sm, md) se recortan al centro para que la cuadrícula
// del menú y de la app se vea pareja; lg conserva la proporción.
type Tamano struct {
	Nombre   string
	Lado     int
	Cuadrado bool
	Calidad  int
}

var Tamanos = []Tamano{
	{"sm", 160, true, 78},   // miniaturas en listas y en la app (caché offline)
	{"md", 480, true, 80},   // tarjetas de producto
	{"lg", 1200, false, 82}, // menú QR y vista ampliada
}

var errTipo = apperr.New(apperr.Invalid, "IMAGEN_INVALIDA", "Sube una foto JPG, PNG o WebP.")

// Procesar valida la imagen y devuelve cada tamaño codificado en WebP.
func Procesar(data []byte) (map[string][]byte, error) {
	if len(data) > MaxBytes {
		return nil, apperr.New(apperr.Invalid, "IMAGEN_GRANDE", "La foto pesa más de 10 MB. Usa una más liviana.")
	}
	switch http.DetectContentType(data) {
	case "image/jpeg", "image/png", "image/webp", "image/gif":
	default:
		return nil, errTipo
	}
	// Leer solo la cabecera primero: evita «bombas» de descompresión.
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, errTipo
	}
	if cfg.Width*cfg.Height > maxPixels {
		return nil, apperr.New(apperr.Invalid, "IMAGEN_ENORME", "La foto tiene demasiados píxeles. Usa una de menos de 40 megapíxeles.")
	}
	if cfg.Width < minLado || cfg.Height < minLado {
		return nil, apperr.New(apperr.Invalid, "IMAGEN_PEQUENA", fmt.Sprintf("La foto es muy pequeña (%d×%d). Usa una de al menos %d píxeles por lado.", cfg.Width, cfg.Height, minLado))
	}
	src, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, errTipo
	}
	src = orientar(src, orientacionEXIF(data))

	out := make(map[string][]byte, len(Tamanos))
	for _, t := range Tamanos {
		img := redimensionar(src, t)
		var buf bytes.Buffer
		if err := webp.Encode(&buf, img, webp.Options{Quality: t.Calidad}); err != nil {
			return nil, fmt.Errorf("imagenes: webp %s: %w", t.Nombre, err)
		}
		out[t.Nombre] = buf.Bytes()
	}
	return out, nil
}

func redimensionar(src image.Image, t Tamano) image.Image {
	b := src.Bounds()
	if t.Cuadrado {
		lado := min(b.Dx(), b.Dy())
		x0, y0 := b.Min.X+(b.Dx()-lado)/2, b.Min.Y+(b.Dy()-lado)/2
		crop := image.Rect(x0, y0, x0+lado, y0+lado)
		dst := image.NewRGBA(image.Rect(0, 0, min(t.Lado, lado), min(t.Lado, lado)))
		xdraw.CatmullRom.Scale(dst, dst.Bounds(), src, crop, draw.Src, nil)
		return dst
	}
	w, h := b.Dx(), b.Dy()
	if max(w, h) > t.Lado {
		if w >= h {
			w, h = t.Lado, h*t.Lado/w
		} else {
			w, h = w*t.Lado/h, t.Lado
		}
	}
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	xdraw.CatmullRom.Scale(dst, dst.Bounds(), src, b, draw.Src, nil)
	return dst
}

// orientacionEXIF lee la etiqueta Orientation (0x0112) de un JPEG. 1 = normal.
func orientacionEXIF(data []byte) int {
	if len(data) < 4 || data[0] != 0xFF || data[1] != 0xD8 {
		return 1
	}
	i := 2
	for i+4 <= len(data) {
		if data[i] != 0xFF {
			return 1
		}
		marker, size := data[i+1], int(binary.BigEndian.Uint16(data[i+2:]))
		if marker == 0xDA || size < 2 || i+2+size > len(data) {
			return 1
		}
		seg := data[i+4 : i+2+size]
		if marker == 0xE1 && len(seg) > 14 && string(seg[:6]) == "Exif\x00\x00" {
			return leerOrientacion(seg[6:])
		}
		i += 2 + size
	}
	return 1
}

func leerOrientacion(tiff []byte) int {
	if len(tiff) < 8 {
		return 1
	}
	var bo binary.ByteOrder
	switch string(tiff[:2]) {
	case "II":
		bo = binary.LittleEndian
	case "MM":
		bo = binary.BigEndian
	default:
		return 1
	}
	off := int(bo.Uint32(tiff[4:]))
	if off+2 > len(tiff) {
		return 1
	}
	n := int(bo.Uint16(tiff[off:]))
	for k := range n {
		e := off + 2 + k*12
		if e+12 > len(tiff) {
			return 1
		}
		if bo.Uint16(tiff[e:]) == 0x0112 {
			if v := int(bo.Uint16(tiff[e+8:])); v >= 1 && v <= 8 {
				return v
			}
		}
	}
	return 1
}

// orientar aplica la transformación EXIF para que la foto quede derecha.
func orientar(src image.Image, o int) image.Image {
	if o <= 1 || o > 8 {
		return src
	}
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	dw, dh := w, h
	if o >= 5 {
		dw, dh = h, w
	}
	dst := image.NewRGBA(image.Rect(0, 0, dw, dh))
	for y := range h {
		for x := range w {
			var nx, ny int
			switch o {
			case 2:
				nx, ny = w-1-x, y
			case 3:
				nx, ny = w-1-x, h-1-y
			case 4:
				nx, ny = x, h-1-y
			case 5:
				nx, ny = y, x
			case 6:
				nx, ny = h-1-y, x
			case 7:
				nx, ny = h-1-y, w-1-x
			case 8:
				nx, ny = y, w-1-x
			}
			dst.Set(nx, ny, src.At(b.Min.X+x, b.Min.Y+y))
		}
	}
	return dst
}
