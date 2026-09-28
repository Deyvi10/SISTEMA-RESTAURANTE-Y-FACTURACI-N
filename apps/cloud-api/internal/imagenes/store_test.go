package imagenes

import (
	"context"
	"errors"
	"io"
	"os"
	"testing"

	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/ids"
)

// probarStore es el contrato que cumple todo Store: lo que se guarda se lee igual
// y lo que no existe responde ErrNoExiste / false.
func probarStore(t *testing.T, s Store) {
	t.Helper()
	ctx := context.Background()
	key := "prueba/" + ids.New().String() + ".webp"

	if ok, err := s.Exists(ctx, key); err != nil || ok {
		t.Fatalf("Exists antes de subir = %v, %v; quiero false, nil", ok, err)
	}
	if _, err := s.Get(ctx, key); !errors.Is(err, ErrNoExiste) {
		t.Fatalf("Get de algo inexistente = %v; quiero ErrNoExiste", err)
	}
	datos := []byte("RIFF\x00\x00\x00\x00WEBPVP8 contenido")
	if err := s.Put(ctx, key, datos, "image/webp"); err != nil {
		t.Fatal(err)
	}
	if ok, err := s.Exists(ctx, key); err != nil || !ok {
		t.Fatalf("Exists después de subir = %v, %v; quiero true, nil", ok, err)
	}
	r, err := s.Get(ctx, key)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	got, _ := io.ReadAll(r)
	if string(got) != string(datos) {
		t.Fatalf("Get devolvió %q; quiero %q", got, datos)
	}
}

func TestMemoryStore(t *testing.T) { probarStore(t, &Memory{}) }

func TestAzureBlobStore(t *testing.T) {
	conn := os.Getenv("TEST_AZURE_STORAGE")
	if conn == "" {
		t.Skip("TEST_AZURE_STORAGE no definido (levanta Azurite con `make dev`)")
	}
	s, err := NewAzureBlob(context.Background(), conn, "pruebas")
	if err != nil {
		t.Skipf("Azurite no disponible: %v", err)
	}
	probarStore(t, s)
}
