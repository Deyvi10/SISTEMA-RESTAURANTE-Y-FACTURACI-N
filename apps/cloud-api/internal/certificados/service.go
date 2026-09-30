package certificados

import (
	"context"
	"crypto/x509"
	"encoding/asn1"
	"errors"
	"io"
	"net/http"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/auditoria"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/auth"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/platform/apperr"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/platform/db"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/platform/httpx"
	aud "github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/auditoria"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/ids"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/sri"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/sri/firma"
)

// MaxP12 limita el archivo: un .p12 de firma electrónica pesa unos pocos KB.
const MaxP12 = 64 << 10

// Service recibe el certificado en la API. Solo tiene la KEK pública: puede guardar un
// certificado pero no leerlo.
type Service struct {
	DB  *db.DB
	KEK Envolvedor // nil: la firma no está configurada en este entorno
	Now func() time.Time
}

// Info es lo que se muestra del certificado (nunca el archivo ni la contraseña).
type Info struct {
	ID          ids.ID    `json:"id"`
	Titular     string    `json:"titular"`
	RUC         *string   `json:"ruc"`
	Emisor      string    `json:"emisor"`
	Serial      string    `json:"serial"`
	ValidoDesde time.Time `json:"validoDesde"`
	ValidoHasta time.Time `json:"validoHasta"`
	SubidoAt    time.Time `json:"subidoAt"`
	// DiasRestantes para el vencimiento (F5-06: alertas a 30, 15, 7 y 1 días).
	DiasRestantes int `json:"diasRestantes"`
	// Aviso cuando no se pudo confirmar el RUC dentro del certificado.
	Aviso *string `json:"aviso,omitempty"`
}

// Contexto amarra el cifrado al restaurante y al certificado: un sobre copiado a otra fila
// no se puede abrir.
func Contexto(tenant, cert ids.ID) string { return "p12:" + tenant.String() + ":" + cert.String() }

// Subir valida el .p12 con su contraseña (vigente, RSA ≥ 2048, del RUC del restaurante), lo
// cifra y lo deja como el certificado activo. El anterior queda inactivo, no se borra.
func (s *Service) Subir(ctx context.Context, p auth.Principal, p12 []byte, clave string) (Info, error) {
	if s.KEK == nil {
		return Info{}, apperr.New(apperr.Conflict, "FIRMA_NO_DISPONIBLE", "La firma electrónica todavía no está disponible en este servidor.")
	}
	if len(p12) == 0 || len(p12) > MaxP12 {
		return Info{}, apperr.New(apperr.Invalid, "P12_INVALIDO", "Adjunta el archivo .p12 de tu firma electrónica (hasta 64 KB).")
	}
	ahora := s.Now()
	f, err := firma.CargarP12(p12, clave, ahora)
	if err != nil {
		return Info{}, apperr.New(apperr.Invalid, "P12_INVALIDO", mensajeP12(err))
	}
	c := f.Certificado()
	id := ids.New()
	var out Info
	err = s.DB.InTenant(ctx, p.TenantID, func(tx db.Tx) error {
		var rucTenant string
		if err := tx.QueryRow(ctx, `SELECT ruc FROM tenants WHERE id = app_tenant()`).Scan(&rucTenant); err != nil {
			return err
		}
		ruc, aviso, err := rucDelCertificado(c, rucTenant)
		if err != nil {
			return err
		}
		sobre, err := Sellar(s.KEK, Contexto(p.TenantID, id), p12, []byte(clave))
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE certificados_firma SET activo = false, reemplazado_at = now() WHERE activo`); err != nil {
			return err
		}
		// Lo que esperaba por la firma sale ya (el worker lo toma en su siguiente pasada).
		if _, err := tx.Exec(ctx, `UPDATE comprobantes SET proximo_intento_at = NULL WHERE estado = 'EN_NUBE'`); err != nil {
			return err
		}
		autor := p.UserID
		if _, err := tx.Exec(ctx, `INSERT INTO certificados_firma (id, tenant_id, p12_cifrado, password_cifrada, dek_cifrada, kek_id,
				titular, ruc, emisor, serial, valido_desde, valido_hasta, subido_por)
			VALUES ($1, app_tenant(), $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`,
			id, sobre.Datos[0], sobre.Datos[1], sobre.DEK, sobre.KEK, titular(c), ruc, c.Issuer.CommonName, c.SerialNumber.String(),
			c.NotBefore, c.NotAfter, autor); err != nil {
			return err
		}
		if out, err = Leer(ctx, tx, ahora); err != nil {
			return err
		}
		out.Aviso = aviso
		return auditoria.Registrar(ctx, tx, p.TenantID, aud.Registro{UsuarioID: &autor, Accion: "CERTIFICADO_SUBIDO",
			Entidad: "certificado_firma", EntidadID: id.String(), Despues: aud.Compactar(map[string]any{
				"titular": out.Titular, "serial": out.Serial, "validoHasta": out.ValidoHasta.Format(time.DateOnly),
			})}, ahora)
	})
	return out, err
}

// Activo devuelve el certificado activo, o nil si no hay.
func (s *Service) Activo(ctx context.Context, p auth.Principal) (*Info, error) {
	var out *Info
	err := s.DB.InTenant(ctx, p.TenantID, func(tx db.Tx) error {
		i, err := Leer(ctx, tx, s.Now())
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		out = &i
		return err
	})
	return out, err
}

// Leer devuelve el certificado activo dentro de una transacción del tenant (pgx.ErrNoRows si no hay).
func Leer(ctx context.Context, tx db.Tx, ahora time.Time) (Info, error) {
	var i Info
	err := tx.QueryRow(ctx, `SELECT id, titular, ruc, emisor, serial, valido_desde, valido_hasta, created_at
		FROM certificados_firma WHERE activo`).Scan(&i.ID, &i.Titular, &i.RUC, &i.Emisor, &i.Serial, &i.ValidoDesde, &i.ValidoHasta, &i.SubidoAt)
	i.DiasRestantes = DiasRestantes(i.ValidoHasta, ahora)
	return i, err
}

// DiasRestantes cuenta los días completos hasta el vencimiento (0 el último día).
func DiasRestantes(hasta, ahora time.Time) int {
	if !ahora.Before(hasta) {
		return -1
	}
	return int(hasta.Sub(ahora) / (24 * time.Hour))
}

func titular(c *x509.Certificate) string {
	if c.Subject.CommonName != "" {
		return c.Subject.CommonName
	}
	return c.Subject.String()
}

func mensajeP12(err error) string {
	m := err.Error()
	switch {
	case strings.Contains(m, "no son válidos"):
		return "No se pudo abrir el archivo: revisa que sea tu firma .p12 y que la contraseña sea la correcta."
	case strings.Contains(m, "no está vigente"):
		return "El certificado no está vigente" + m[strings.Index(m, " (del"):] + ". Renueva tu firma con tu entidad certificadora."
	case strings.Contains(m, "bits"), strings.Contains(m, "no es RSA"):
		return "El certificado no cumple lo que exige el SRI (llave RSA de 2048 bits)."
	default:
		return "No se pudo usar este certificado: " + strings.TrimPrefix(m, "firma: no se pudo firmar: ")
	}
}

var numeros = regexp.MustCompile(`[0-9]{10}(?:001)?`)

// rucDelCertificado busca el RUC o la cédula del titular en el certificado. 🔎 Cada entidad
// certificadora del Ecuador lo pone en un lugar distinto (serialNumber del sujeto o una
// extensión propia) y falta un .p12 real para confirmarlo (DP-04): se buscan en el sujeto y en
// las extensiones las identificaciones con dígito verificador válido. Si hay alguna y ninguna
// es del restaurante, se bloquea (F5-06); si no hay ninguna, se acepta con un aviso.
func rucDelCertificado(c *x509.Certificate, rucTenant string) (*string, *string, error) {
	var textos []string
	for _, n := range c.Subject.Names {
		if s, ok := n.Value.(string); ok {
			textos = append(textos, s)
		}
	}
	for _, e := range c.Extensions {
		var s string
		if _, err := asn1.Unmarshal(e.Value, &s); err == nil {
			textos = append(textos, s)
		} else {
			textos = append(textos, string(e.Value))
		}
	}
	var hallados []string
	for _, t := range textos {
		for _, n := range numeros.FindAllString(t, -1) {
			if (len(n) == 13 && sri.ValidarRUC(n).Valida) || (len(n) == 10 && sri.ValidarCedula(n).Valida) {
				hallados = append(hallados, n)
			}
		}
	}
	if len(hallados) == 0 {
		aviso := "No pudimos leer el RUC dentro del certificado: confirma que la firma es de " + rucTenant + "."
		return nil, &aviso, nil
	}
	// Persona natural: su RUC es la cédula + 001 y su firma suele traer la cédula.
	if slices.Contains(hallados, rucTenant) || slices.Contains(hallados, rucTenant[:10]) {
		return &rucTenant, nil, nil
	}
	return nil, nil, apperr.New(apperr.Conflict, "RUC_DISTINTO",
		"Esta firma es de "+hallados[0]+" y el restaurante factura con el RUC "+rucTenant+". Sube la firma del titular del RUC.")
}

// HandleSubir: POST /v1/facturacion/certificado (multipart: «p12» y «clave»). La contraseña
// no se registra en ningún log.
func (s *Service) HandleSubir(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, MaxP12+16<<10)
	f, _, err := r.FormFile("p12")
	if err != nil {
		httpx.Error(w, r, apperr.New(apperr.Invalid, "P12_INVALIDO", "Adjunta el archivo .p12 de tu firma electrónica (hasta 64 KB)."))
		return
	}
	defer func() { _ = f.Close() }()
	data, err := io.ReadAll(io.LimitReader(f, MaxP12+1))
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	defer Borrar(data)
	clave := r.FormValue("clave")
	if clave == "" {
		httpx.Error(w, r, apperr.New(apperr.Invalid, "P12_INVALIDO", "Escribe la contraseña de tu firma electrónica."))
		return
	}
	info, err := s.Subir(r.Context(), auth.MustPrincipal(r.Context()), data, clave)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, info)
}

// HandleActivo: GET /v1/facturacion/certificado (204 si no hay).
func (s *Service) HandleActivo(w http.ResponseWriter, r *http.Request) {
	info, err := s.Activo(r.Context(), auth.MustPrincipal(r.Context()))
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	if info == nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	httpx.JSON(w, http.StatusOK, info)
}
