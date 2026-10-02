// Package reportes da los reportes básicos del panel (F5-16): ventas del día y por periodo
// por fecha de negocio (jornada), por método de pago, notas de crédito, Cierres Z con su PDF y
// la exportación de ventas a Excel para el contador.
package reportes

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/shopspring/decimal"

	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/auth"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/caja"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/platform/apperr"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/platform/db"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/platform/httpx"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/platform/xlsx"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/cierrez"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/ids"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/money"
)

// EventoVentaCobrada es la venta que el nodo cobró (documento interno o factura).
const EventoVentaCobrada = "venta.cobrada"

// RegistrarVenta proyecta la venta del nodo en «ventas» (idempotente). Un payload ilegible
// queda solo en sync_eventos.
func RegistrarVenta(ctx context.Context, tx pgx.Tx, tenant, local ids.ID, payload []byte) error {
	var v struct {
		Documento struct {
			ID           ids.ID          `json:"id"`
			Tipo         string          `json:"tipo"`
			Codigo       string          `json:"codigo"`
			Mesa         string          `json:"mesa"`
			Cajero       string          `json:"cajero"`
			Comprador    string          `json:"comprador"`
			EmitidoAt    time.Time       `json:"emitidoAt"`
			FechaNegocio string          `json:"fechaNegocio"`
			Pagos        json.RawMessage `json:"pagos"`
			Metodo       string          `json:"metodo"` // ventas anteriores al pago mixto (F4-06)
			Totales      struct {
				Subtotal, IVA, Propina, Descuento, Total string
			} `json:"totales"`
		} `json:"documento"`
	}
	if err := json.Unmarshal(payload, &v); err != nil || v.Documento.ID == ids.Nil {
		return nil
	}
	d := v.Documento
	fecha := d.FechaNegocio
	if _, err := time.Parse(time.DateOnly, fecha); err != nil {
		fecha = d.EmitidoAt.In(guayaquil).Format(time.DateOnly)
	}
	monto := func(s string) string {
		m, err := money.Parse(s)
		if err != nil {
			return "0"
		}
		return m.String()
	}
	pagos := d.Pagos
	if len(pagos) == 0 || string(pagos) == "null" || string(pagos) == "[]" {
		// Antes del pago mixto el documento tenía un solo método por el total.
		pagos = json.RawMessage("[]")
		if d.Metodo != "" {
			pagos, _ = json.Marshal([]map[string]string{{"metodo": d.Metodo, "monto": monto(d.Totales.Total)}})
		}
	}
	_, err := tx.Exec(ctx, `INSERT INTO ventas (documento_id, tenant_id, local_id, fecha_negocio, emitido_at, tipo, codigo, mesa, cajero, comprador,
			subtotal, iva, propina, descuento, total, pagos)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16) ON CONFLICT DO NOTHING`,
		d.ID, tenant, local, fecha, d.EmitidoAt, d.Tipo, d.Codigo, d.Mesa, d.Cajero, d.Comprador,
		monto(d.Totales.Subtotal), monto(d.Totales.IVA), monto(d.Totales.Propina), monto(d.Totales.Descuento), monto(d.Totales.Total), pagos)
	return err
}

var guayaquil = func() *time.Location {
	l, err := time.LoadLocation("America/Guayaquil")
	if err != nil {
		return time.FixedZone("ECT", -5*3600)
	}
	return l
}()

type Service struct {
	DB *db.DB
}

// Totales de un día o del periodo.
type Totales struct {
	Documentos int    `json:"documentos"`
	Subtotal   string `json:"subtotal"`
	IVA        string `json:"iva"`
	Propina    string `json:"propina"`
	Descuento  string `json:"descuento"`
	Total      string `json:"total"`
}

type Dia struct {
	Fecha string `json:"fecha"`
	Totales
}

type PorMetodo struct {
	Metodo string `json:"metodo"`
	Monto  string `json:"monto"`
	Pagos  int    `json:"pagos"`
}

// Resumen de ventas de un periodo por fecha de negocio.
type Resumen struct {
	Desde          string      `json:"desde"`
	Hasta          string      `json:"hasta"`
	Dias           []Dia       `json:"dias"`
	Total          Totales     `json:"total"`
	TicketPromedio string      `json:"ticketPromedio"`
	PorMetodo      []PorMetodo `json:"porMetodo"`
	NotasCredito   struct {
		Cantidad int    `json:"cantidad"`
		Valor    string `json:"valor"`
	} `json:"notasCredito"`
	// Neto: ventas menos lo revertido con notas de crédito.
	Neto string `json:"neto"`
}

func rango(desde, hasta string) (string, string, error) {
	hoy := time.Now().In(guayaquil).Format(time.DateOnly)
	if desde == "" {
		desde = hoy
	}
	if hasta == "" {
		hasta = desde
	}
	d, err1 := time.Parse(time.DateOnly, desde)
	h, err2 := time.Parse(time.DateOnly, hasta)
	if err := errors.Join(err1, err2); err != nil || h.Before(d) || h.Sub(d) > 366*24*time.Hour {
		return "", "", apperr.New(apperr.Invalid, "RANGO_INVALIDO", "Elige un periodo válido de hasta un año.")
	}
	return desde, hasta, nil
}

// Ventas resume el periodo (por defecto, la jornada de hoy).
func (s *Service) Ventas(ctx context.Context, p auth.Principal, desde, hasta string) (Resumen, error) {
	desde, hasta, err := rango(desde, hasta)
	if err != nil {
		return Resumen{}, err
	}
	r := Resumen{Desde: desde, Hasta: hasta, Dias: []Dia{}, PorMetodo: []PorMetodo{}}
	err = s.DB.InTenant(ctx, p.TenantID, func(tx db.Tx) error {
		rows, err := tx.Query(ctx, `SELECT fecha_negocio::text, count(*), sum(subtotal)::text, sum(iva)::text, sum(propina)::text, sum(descuento)::text, sum(total)::text
			FROM ventas WHERE fecha_negocio BETWEEN $1 AND $2 GROUP BY 1 ORDER BY 1`, desde, hasta)
		if err != nil {
			return err
		}
		if r.Dias, err = pgx.CollectRows(rows, pgx.RowToStructByPos[Dia]); err != nil {
			return err
		}
		if err := tx.QueryRow(ctx, `SELECT count(*), coalesce(sum(subtotal), 0)::text, coalesce(sum(iva), 0)::text, coalesce(sum(propina), 0)::text,
				coalesce(sum(descuento), 0)::text, coalesce(sum(total), 0)::text FROM ventas WHERE fecha_negocio BETWEEN $1 AND $2`, desde, hasta).
			Scan(&r.Total.Documentos, &r.Total.Subtotal, &r.Total.IVA, &r.Total.Propina, &r.Total.Descuento, &r.Total.Total); err != nil {
			return err
		}
		rows, err = tx.Query(ctx, `SELECT p->>'metodo', sum((p->>'monto')::numeric)::text, count(*)
			FROM ventas v, jsonb_array_elements(v.pagos) p WHERE v.fecha_negocio BETWEEN $1 AND $2 GROUP BY 1 ORDER BY sum((p->>'monto')::numeric) DESC`, desde, hasta)
		if err != nil {
			return err
		}
		if r.PorMetodo, err = pgx.CollectRows(rows, pgx.RowToStructByPos[PorMetodo]); err != nil {
			return err
		}
		// Notas de crédito autorizadas o en camino por su fecha de emisión.
		return tx.QueryRow(ctx, `SELECT count(*), coalesce(sum(importe_total), 0)::text FROM comprobantes
			WHERE tipo = '04' AND fecha_emision BETWEEN $1 AND $2 AND estado NOT IN ('DEVUELTO', 'NO_AUTORIZADO', 'REQUIERE_ATENCION')`, desde, hasta).
			Scan(&r.NotasCredito.Cantidad, &r.NotasCredito.Valor)
	})
	if err != nil {
		return r, err
	}
	total, _ := money.Parse(r.Total.Total)
	nc, _ := money.Parse(r.NotasCredito.Valor)
	r.Neto = total.Sub(nc).String()
	r.TicketPromedio = money.Zero.String()
	if r.Total.Documentos > 0 {
		r.TicketPromedio = money.FromDecimal(total.Decimal().Div(decimal.NewFromInt(int64(r.Total.Documentos)))).Round2().String()
	}
	return r, nil
}

// CierreFila es un Cierre Z de la lista.
type CierreFila struct {
	ID           ids.ID    `json:"id"`
	Numero       int       `json:"numero"`
	Caja         string    `json:"caja"`
	Cajero       string    `json:"cajero"`
	FechaNegocio string    `json:"fechaNegocio"`
	CerradoAt    time.Time `json:"cerradoAt"`
	Resultado    string    `json:"resultado"`
	HashValido   bool      `json:"hashValido"`
}

// Cierres lista los Cierres Z del periodo (por fecha de negocio).
func (s *Service) Cierres(ctx context.Context, p auth.Principal, desde, hasta string) ([]CierreFila, error) {
	desde, hasta, err := rango(desde, hasta)
	if err != nil {
		return nil, err
	}
	out := []CierreFila{}
	err = s.DB.InTenant(ctx, p.TenantID, func(tx db.Tx) error {
		rows, err := tx.Query(ctx, `SELECT id, numero, datos->>'caja', cajero, fecha_negocio::text, cerrado_at, resultado, hash_valido
			FROM cierres_z WHERE fecha_negocio BETWEEN $1 AND $2 ORDER BY cerrado_at DESC`, desde, hasta)
		if err != nil {
			return err
		}
		out, err = pgx.CollectRows(rows, pgx.RowToStructByPos[CierreFila])
		return err
	})
	return out, err
}

// CierrePDF es el mismo PDF que recibe el dueño por correo.
func (s *Service) CierrePDF(ctx context.Context, p auth.Principal, id ids.ID) (string, []byte, error) {
	var raw []byte
	var zona string
	err := s.DB.InTenant(ctx, p.TenantID, func(tx db.Tx) error {
		err := tx.QueryRow(ctx, `SELECT c.datos, l.zona_horaria FROM cierres_z c JOIN locales l ON l.id = c.local_id WHERE c.id = $1`, id).Scan(&raw, &zona)
		if errors.Is(err, pgx.ErrNoRows) {
			return apperr.ErrNotFound
		}
		return err
	})
	if err != nil {
		return "", nil, err
	}
	var c cierrez.Cierre
	if err := json.Unmarshal(raw, &c); err != nil {
		return "", nil, err
	}
	loc, err := time.LoadLocation(zona)
	if err != nil {
		loc = guayaquil
	}
	return fmt.Sprintf("cierre-z-%04d-%s.pdf", c.Numero, c.FechaNegocio), caja.PDFCierre(c, loc), nil
}

// Excel exporta las ventas del periodo, una fila por documento, para el contador.
func (s *Service) Excel(ctx context.Context, p auth.Principal, desde, hasta string) (string, []byte, error) {
	desde, hasta, err := rango(desde, hasta)
	if err != nil {
		return "", nil, err
	}
	var filas [][]xlsx.Celda
	err = s.DB.InTenant(ctx, p.TenantID, func(tx db.Tx) error {
		rows, err := tx.Query(ctx, `SELECT v.fecha_negocio::text, v.emitido_at, v.tipo, v.codigo, coalesce(v.mesa, ''), coalesce(v.comprador, ''), coalesce(v.cajero, ''),
				v.subtotal::text, v.iva::text, v.propina::text, v.descuento::text, v.total::text,
				coalesce((SELECT string_agg(p->>'metodo', ' + ') FROM jsonb_array_elements(v.pagos) p), '')
			FROM ventas v WHERE v.fecha_negocio BETWEEN $1 AND $2 ORDER BY v.emitido_at`, desde, hasta)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var fecha, tipo, codigo, mesa, comprador, cajero, sub, iva, prop, desc, total, metodos string
			var emitido time.Time
			if err := rows.Scan(&fecha, &emitido, &tipo, &codigo, &mesa, &comprador, &cajero, &sub, &iva, &prop, &desc, &total, &metodos); err != nil {
				return err
			}
			filas = append(filas, []xlsx.Celda{xlsx.T(fecha), xlsx.T(emitido.In(guayaquil).Format("2006-01-02 15:04")), xlsx.T(tipo), xlsx.T(codigo),
				xlsx.T(mesa), xlsx.T(comprador), xlsx.T(cajero), xlsx.N(sub), xlsx.N(iva), xlsx.N(prop), xlsx.N(desc), xlsx.N(total), xlsx.T(metodos)})
		}
		return rows.Err()
	})
	if err != nil {
		return "", nil, err
	}
	b, err := xlsx.Libro("Ventas", []string{"Jornada", "Emitido", "Tipo", "Número", "Mesa", "Comprador", "Cajero", "Subtotal", "IVA", "Servicio",
		"Descuento", "Total", "Pago"}, filas)
	return fmt.Sprintf("ventas-%s-a-%s.xlsx", desde, hasta), b, err
}

// ---------- HTTP ----------

func (s *Service) HandleVentas(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	out, err := s.Ventas(r.Context(), auth.MustPrincipal(r.Context()), q.Get("desde"), q.Get("hasta"))
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, out)
}

func (s *Service) HandleCierres(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	out, err := s.Cierres(r.Context(), auth.MustPrincipal(r.Context()), q.Get("desde"), q.Get("hasta"))
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, out)
}

func archivo(w http.ResponseWriter, nombre, tipo string, b []byte) {
	w.Header().Set("Content-Type", tipo)
	w.Header().Set("Content-Disposition", `attachment; filename="`+nombre+`"`)
	w.Header().Set("Cache-Control", "private, no-store")
	_, _ = w.Write(b)
}

func (s *Service) HandleCierrePDF(w http.ResponseWriter, r *http.Request) {
	id, err := ids.Parse(r.PathValue("id"))
	if err != nil {
		httpx.Error(w, r, apperr.ErrNotFound)
		return
	}
	nombre, b, err := s.CierrePDF(r.Context(), auth.MustPrincipal(r.Context()), id)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	archivo(w, nombre, "application/pdf", b)
}

func (s *Service) HandleExcel(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	nombre, b, err := s.Excel(r.Context(), auth.MustPrincipal(r.Context()), q.Get("desde"), q.Get("hasta"))
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	archivo(w, nombre, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", b)
}
