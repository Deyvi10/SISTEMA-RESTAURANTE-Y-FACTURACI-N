package server_test

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/json"
	"image"
	"image/color"
	"image/jpeg"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/auth"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/catalogo"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/imagenes"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/personal"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/platform/mail"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/platform/testdb"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/salon"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/server"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/tenants"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/clock"
)

type env struct {
	t    *testing.T
	tdb  *testdb.DB
	srv  *httptest.Server
	mail *mail.Memory
	ten  *tenants.Service
}

func newEnv(t *testing.T) *env {
	t.Helper()
	tdb := testdb.New(t)
	m := &mail.Memory{}
	clk := clock.Real{}
	_, key, _ := ed25519.GenerateKey(nil)
	signer := auth.NewSigner(key, clk.Now)
	img := &imagenes.Service{Store: &imagenes.Memory{}}
	deps := server.Deps{
		DB: tdb.App, Signer: signer, Now: clk.Now, BackofficeURL: "http://bo.test",
		Auth:     &auth.Handlers{Svc: &auth.Service{DB: tdb.App, Signer: signer, Mail: m, Clock: clk, BackofficeURL: "http://bo.test"}},
		Salon:    &salon.Service{DB: tdb.App},
		Catalogo: &catalogo.Service{DB: tdb.App, ImagenURL: img.URL, Clock: clk},
		Personal: &personal.Service{DB: tdb.App, Mail: m, Pepper: []byte("pepper-de-pruebas-0123456789abcdef"), ImagenURL: img.URL},
		Imagenes: img,
	}
	srv := httptest.NewServer(server.Handler(deps, server.Routes(deps)))
	img.PublicURL = srv.URL
	t.Cleanup(srv.Close)
	return &env{t: t, tdb: tdb, srv: srv, mail: m, ten: &tenants.Service{DB: tdb.App, Mail: m, BackofficeURL: "http://bo.test"}}
}

// cliente simula el backoffice de un usuario con su cookie de refresh.
type cliente struct {
	e     *env
	http  *http.Client
	token string
}

func (e *env) restaurante(ruc, email string) (*cliente, tenants.Resultado) {
	e.t.Helper()
	r, err := e.ten.Crear(context.Background(), tenants.Alta{RUC: ruc, RazonSocial: "R " + ruc, NombreComercial: "Local " + ruc, NombreDueno: "Dueño", EmailDueno: email, Plan: "RESTAURANTE"})
	if err != nil {
		e.t.Fatal(err)
	}
	c := e.login(email, r.PasswordTemporal)
	c.do("POST", "/v1/auth/password/cambiar", map[string]string{"actual": r.PasswordTemporal, "nueva": "ClaveSegura2026"}, 200, nil)
	return c, r
}

func (e *env) login(usuario, password string) *cliente {
	e.t.Helper()
	jar, _ := cookiejar.New(nil)
	c := &cliente{e: e, http: &http.Client{Jar: jar}}
	var s struct{ AccessToken string }
	c.do("POST", "/v1/auth/login", map[string]string{"usuario": usuario, "password": password}, 200, &s)
	return c
}

func (c *cliente) do(method, path string, body any, want int, out any) {
	c.e.t.Helper()
	var rdr io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rdr = bytes.NewReader(b)
	}
	req, _ := http.NewRequest(method, c.e.srv.URL+path, rdr)
	req.Header.Set("Content-Type", "application/json")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	res, err := c.http.Do(req)
	if err != nil {
		c.e.t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	raw, _ := io.ReadAll(res.Body)
	if res.StatusCode != want {
		c.e.t.Fatalf("%s %s = %d, se esperaba %d: %s", method, path, res.StatusCode, want, raw)
	}
	// Guardar el access token de login, refresh y cambio de contraseña.
	var tok struct{ AccessToken string }
	if json.Unmarshal(raw, &tok) == nil && tok.AccessToken != "" {
		c.token = tok.AccessToken
	}
	if out != nil && len(raw) > 0 {
		if err := json.Unmarshal(raw, out); err != nil {
			c.e.t.Fatalf("respuesta ilegible: %v: %s", err, raw)
		}
	}
}

func fotoJPEG(w, h int) []byte {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.Set(x, y, color.RGBA{uint8(x * 255 / w), uint8(y * 255 / h), 90, 255}) //nolint:gosec // prueba
		}
	}
	var b bytes.Buffer
	_ = jpeg.Encode(&b, img, nil)
	return b.Bytes()
}

func (c *cliente) subirFoto(data []byte, want int) imagenes.Imagen {
	c.e.t.Helper()
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	fw, _ := mw.CreateFormFile("foto", "plato.jpg")
	_, _ = fw.Write(data)
	_ = mw.Close()
	req, _ := http.NewRequest("POST", c.e.srv.URL+"/v1/imagenes", &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+c.token)
	res, err := c.http.Do(req)
	if err != nil {
		c.e.t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	raw, _ := io.ReadAll(res.Body)
	if res.StatusCode != want {
		c.e.t.Fatalf("subir foto = %d: %s", res.StatusCode, raw)
	}
	var img imagenes.Imagen
	_ = json.Unmarshal(raw, &img)
	return img
}

// Flujo completo del dueño: exactamente lo que hará el backoffice (DoD de F1).
func TestFlujoDelDueno(t *testing.T) {
	e := newEnv(t)
	c, _ := e.restaurante("1790011674001", "dueno@a.ec")

	var resumen server.Resumen
	c.do("GET", "/v1/resumen", nil, 200, &resumen)
	if resumen.Progreso != 0 || resumen.Cuentas["categorias"] != 3 || resumen.Cuentas["mesas"] != 1 {
		t.Fatalf("resumen inicial: %+v", resumen)
	}

	var tarifas []catalogo.TarifaIVA
	c.do("GET", "/v1/tarifas-iva", nil, 200, &tarifas)
	if len(tarifas) == 0 || tarifas[0].Porcentaje != "15.00" {
		t.Fatalf("tarifas: %+v", tarifas)
	}
	for _, tf := range tarifas {
		if tf.Porcentaje == "12.00" {
			t.Fatal("el IVA 12 % histórico no debe ofrecerse")
		}
	}

	var cat catalogo.Categoria
	c.do("POST", "/v1/categorias", map[string]any{"nombre": "Ceviches", "icono": "fish", "color": "teal", "notasRapidas": []string{"Sin cebolla", "Extra limón"}}, 201, &cat)
	c.do("POST", "/v1/categorias", map[string]any{"nombre": "ceviches"}, 409, nil)
	c.do("POST", "/v1/categorias", map[string]any{"nombre": "Otro", "icono": "hamburger"}, 422, nil)

	var grupo catalogo.Grupo
	c.do("POST", "/v1/grupos-modificadores", map[string]any{"nombre": "Acompañado", "obligatorio": true, "modificadores": []map[string]string{{"nombre": "Arroz"}, {"nombre": "Patacones", "precioAdicional": "0.50"}}}, 201, &grupo)
	if grupo.Min != 1 || grupo.Max != 1 || len(grupo.Modificadores) != 2 {
		t.Fatalf("grupo: %+v", grupo)
	}

	foto := c.subirFoto(fotoJPEG(900, 600), 201)
	if !strings.HasPrefix(foto.Key, "t/") || foto.URLs["md"] == "" {
		t.Fatalf("foto: %+v", foto)
	}
	res, err := http.Get(foto.URLs["md"])
	if err != nil || res.StatusCode != 200 || res.Header.Get("Content-Type") != "image/webp" {
		t.Fatalf("media: %v %v", err, res)
	}
	_ = res.Body.Close()
	c.subirFoto([]byte("no soy una foto, soy un texto cualquiera que dice ser jpg"), 422)
	c.subirFoto(fotoJPEG(100, 100), 422) // demasiado pequeña

	var prod catalogo.Producto
	c.do("POST", "/v1/productos", map[string]any{
		"categoriaId": cat.ID, "nombre": "Ceviche mixto", "alias": "cm", "precio": "15.00", "tarifaIvaId": tarifas[0].ID,
		"imagenKey": foto.Key, "gruposModificadores": []any{grupo.ID},
	}, 201, &prod)
	if prod.Desglose != (catalogo.Desglose{Base: "13.04", IVA: "1.96", Total: "15.00"}) || *prod.Alias != "CM" || prod.ImagenURL == nil {
		t.Fatalf("producto: %+v", prod)
	}
	c.do("POST", "/v1/productos", map[string]any{"categoriaId": cat.ID, "nombre": "Otro", "alias": "CM", "precio": "1", "tarifaIvaId": tarifas[0].ID}, 409, nil)
	c.do("POST", "/v1/productos", map[string]any{"categoriaId": cat.ID, "nombre": "Caro", "precio": "150000", "tarifaIvaId": tarifas[0].ID}, 422, nil)
	c.do("POST", "/v1/productos", map[string]any{"categoriaId": cat.ID, "nombre": "Coma", "precio": "12,50", "tarifaIvaId": tarifas[0].ID}, 422, nil)

	var buscados []catalogo.Producto
	c.do("GET", "/v1/productos?q=cm", nil, 200, &buscados)
	if len(buscados) != 1 {
		t.Fatalf("búsqueda por alias: %d", len(buscados))
	}

	var zonas []salon.Zona
	c.do("GET", "/v1/zonas", nil, 200, &zonas)
	var lote []salon.Mesa
	c.do("POST", "/v1/mesas/lote", map[string]any{"zonaId": zonas[0].ID, "cantidad": 12}, 201, &lote)
	if len(lote) != 12 || lote[0].Nombre != "Mesa 2" || lote[11].PosX != 0 || lote[11].PosY != 2 {
		t.Fatalf("lote: %d, primera %+v, última %+v", len(lote), lote[0], lote[11])
	}
	c.do("DELETE", "/v1/zonas/"+zonas[0].ID.String(), nil, 409, nil)

	var mesero personal.Usuario
	c.do("POST", "/v1/usuarios", map[string]any{"nombreMostrar": "Carlos M.", "rol": "MESERO", "pin": "8899"}, 201, &mesero)
	c.do("POST", "/v1/usuarios", map[string]any{"nombreMostrar": "Ana", "rol": "MESERO", "pin": "8899"}, 409, nil)
	c.do("POST", "/v1/usuarios", map[string]any{"nombreMostrar": "Ana", "rol": "MESERO", "pin": "1234"}, 422, nil)
	c.do("POST", "/v1/usuarios", map[string]any{"nombreMostrar": "Ana", "rol": "MESERO"}, 422, nil)

	var cajero personal.Usuario
	c.do("POST", "/v1/usuarios", map[string]any{"nombreMostrar": "Luis", "rol": "CAJERO", "pin": "7391", "email": "luis@a.ec"}, 201, &cajero)
	inv, _ := e.mail.Last()
	if inv.To != "luis@a.ec" || !cajero.AccesoWeb {
		t.Fatalf("invitación: %+v %+v", inv, cajero)
	}

	c.do("GET", "/v1/resumen", nil, 200, &resumen)
	if resumen.Progreso != 75 { // menú (<3 platos) pendiente; fotos, salón y equipo listos
		t.Fatalf("progreso = %d: %+v", resumen.Progreso, resumen.Pasos)
	}
}

// Un cajero no puede configurar el menú ni el personal; entra al panel pero la API lo frena.
func TestPermisosPorRol(t *testing.T) {
	e := newEnv(t)
	c, _ := e.restaurante("1790011674001", "dueno@a.ec")
	c.do("POST", "/v1/usuarios", map[string]any{"nombreMostrar": "Luis", "rol": "CAJERO", "pin": "7391", "email": "luis@a.ec"}, 201, nil)
	msg, _ := e.mail.Last()
	temporal := strings.TrimSpace(strings.Split(strings.Split(msg.Text, "la primera vez:\n\n")[1], "\n")[0])
	cajero := e.login("luis@a.ec", temporal)
	cajero.do("GET", "/v1/categorias", nil, 403, nil) // debe cambiar la contraseña primero
	cajero.do("POST", "/v1/auth/password/cambiar", map[string]string{"actual": temporal, "nueva": "CajaSegura2026"}, 200, nil)
	cajero.do("GET", "/v1/me", nil, 200, nil)
	cajero.do("GET", "/v1/categorias", nil, 403, nil)
	cajero.do("POST", "/v1/usuarios", map[string]any{"nombreMostrar": "X", "rol": "ADMIN", "email": "x@a.ec"}, 403, nil)
	cajero.do("GET", "/v1/resumen", nil, 200, nil)

	anon := &cliente{e: e, http: http.DefaultClient}
	anon.do("GET", "/v1/categorias", nil, 401, nil)
	anon.token = "basura"
	anon.do("GET", "/v1/categorias", nil, 401, nil)
}

// Desactivar a alguien lo saca al instante aunque su access token no haya vencido (X-05).
func TestDesactivarCierraSesionAlInstante(t *testing.T) {
	e := newEnv(t)
	dueno, _ := e.restaurante("1790011674001", "dueno@a.ec")
	var u personal.Usuario
	dueno.do("POST", "/v1/usuarios", map[string]any{"nombreMostrar": "Admin 2", "rol": "ADMIN", "pin": "5821", "email": "admin2@a.ec"}, 201, &u)
	msg, _ := e.mail.Last()
	temporal := strings.TrimSpace(strings.Split(strings.Split(msg.Text, "la primera vez:\n\n")[1], "\n")[0])
	admin2 := e.login("admin2@a.ec", temporal)
	admin2.do("POST", "/v1/auth/password/cambiar", map[string]string{"actual": temporal, "nueva": "OtraClave2026"}, 200, nil)
	admin2.do("GET", "/v1/categorias", nil, 200, nil)
	dueno.do("PUT", "/v1/usuarios/"+u.ID.String()+"/estado", map[string]bool{"activo": false}, 200, nil)
	admin2.do("GET", "/v1/categorias", nil, 401, nil)
	var me struct{ ID string }
	dueno.do("GET", "/v1/me", nil, 200, &me)
	dueno.do("PUT", "/v1/usuarios/"+me.ID+"/estado", map[string]bool{"activo": false}, 409, nil) // el dueño no se desactiva
}

// Un restaurante nunca ve ni toca lo de otro por la API: 404, no 403 (no revela que existe).
func TestAislamientoPorAPI(t *testing.T) {
	e := newEnv(t)
	a, _ := e.restaurante("1790011674001", "a@a.ec")
	b, _ := e.restaurante("1760001550001", "b@b.ec")
	var catA []catalogo.Categoria
	a.do("GET", "/v1/categorias", nil, 200, &catA)
	b.do("PUT", "/v1/categorias/"+catA[0].ID.String(), map[string]any{"nombre": "Hackeado"}, 404, nil)
	b.do("DELETE", "/v1/categorias/"+catA[0].ID.String(), nil, 404, nil)
	fotoA := a.subirFoto(fotoJPEG(600, 400), 201)
	var tarifas []catalogo.TarifaIVA
	b.do("GET", "/v1/tarifas-iva", nil, 200, &tarifas)
	var catB []catalogo.Categoria
	b.do("GET", "/v1/categorias", nil, 200, &catB)
	// B intenta usar la foto de A en su producto.
	b.do("POST", "/v1/productos", map[string]any{"categoriaId": catB[0].ID, "nombre": "X", "precio": "1", "tarifaIvaId": tarifas[0].ID, "imagenKey": fotoA.Key}, 422, nil)
	// B intenta crear un producto en una categoría de A.
	b.do("POST", "/v1/productos", map[string]any{"categoriaId": catA[0].ID, "nombre": "X", "precio": "1", "tarifaIvaId": tarifas[0].ID}, 422, nil)
	// Pero sí puede usar la galería común.
	b.do("POST", "/v1/productos", map[string]any{"categoriaId": catB[0].ID, "nombre": "Ceviche", "precio": "10", "tarifaIvaId": tarifas[0].ID, "imagenKey": "biblioteca/ceviche-camaron"}, 201, nil)
}
