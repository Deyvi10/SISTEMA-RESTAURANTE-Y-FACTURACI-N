package caja

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/platform/db"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/ids"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/sri"
)

// EventoClienteGuardado: un nodo guardó (con consentimiento) los datos de un comprador.
const EventoClienteGuardado = "cliente.guardado"

var camposCliente = []string{"razonSocial", "direccion", "email", "telefono"}

// Cliente del restaurante con la hora en que cambió cada campo.
type Cliente struct {
	ID                 ids.ID               `json:"id"`
	TipoIdentificacion string               `json:"tipoIdentificacion"`
	Identificacion     string               `json:"identificacion"`
	RazonSocial        string               `json:"razonSocial"`
	Direccion          string               `json:"direccion"`
	Email              string               `json:"email"`
	Telefono           string               `json:"telefono"`
	CamposAt           map[string]time.Time `json:"camposAt"`
}

func (c *Cliente) campo(k string) *string {
	switch k {
	case "razonSocial":
		return &c.RazonSocial
	case "direccion":
		return &c.Direccion
	case "email":
		return &c.Email
	default:
		return &c.Telefono
	}
}

// Fusionar combina dos versiones campo por campo: gana la marca de tiempo más reciente
// (a igual hora se queda la que ya estaba). Así dos cajas sin internet que cambian campos
// distintos no se pisan.
func Fusionar(actual, llega Cliente) Cliente {
	out := actual
	out.CamposAt = map[string]time.Time{}
	for _, k := range camposCliente {
		ta, tl := actual.CamposAt[k], llega.CamposAt[k]
		if tl.After(ta) {
			*out.campo(k) = *llega.campo(k)
			out.CamposAt[k] = tl
		} else if !ta.IsZero() {
			out.CamposAt[k] = ta
		}
	}
	return out
}

// Proveedor es la tercera fuente de la búsqueda en cascada (RF-04-05.2c): un servicio
// externo de datos de contribuyentes. Opcional: sin proveedor configurado no se consulta.
type Proveedor interface {
	Buscar(ctx context.Context, tipo, identificacion string) (*Cliente, error)
}

func leerCliente(ctx context.Context, tx pgx.Tx, tipo, identificacion string) (*Cliente, error) {
	var c Cliente
	var dir, email, tel *string
	var campos []byte
	err := tx.QueryRow(ctx, `SELECT id, tipo_identificacion, identificacion, razon_social, direccion, email::text, telefono, campos_at FROM clientes
		WHERE identificacion = $1 AND ($2 = '' OR tipo_identificacion = $2) FOR UPDATE`, identificacion, tipo).
		Scan(&c.ID, &c.TipoIdentificacion, &c.Identificacion, &c.RazonSocial, &dir, &email, &tel, &campos)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	for p, v := range map[*string]*string{&c.Direccion: dir, &c.Email: email, &c.Telefono: tel} {
		if v != nil {
			*p = *v
		}
	}
	_ = json.Unmarshal(campos, &c.CamposAt)
	return &c, nil
}

// RegistrarCliente aplica el evento de un nodo dentro de la transacción del push.
func RegistrarCliente(ctx context.Context, tx pgx.Tx, tenant ids.ID, payload []byte) error {
	var ev struct {
		Cliente          Cliente   `json:"cliente"`
		ConsentimientoAt time.Time `json:"consentimientoAt"`
	}
	if err := json.Unmarshal(payload, &ev); err != nil {
		return nil // queda en la bitácora; no detiene la sincronización
	}
	c := ev.Cliente
	c.Identificacion = strings.ToUpper(strings.TrimSpace(c.Identificacion))
	switch sri.TipoIdentificacion(c.TipoIdentificacion) {
	case sri.IdRUC, sri.IdCedula:
		if !sri.ValidarIdentificacion(c.Identificacion).Valida {
			return nil
		}
	case sri.IdPasaporte, sri.IdExterior:
		if !sri.ValidarPasaporte(c.Identificacion).Valida {
			return nil
		}
	default:
		return nil
	}
	if c.ID == ids.Nil || strings.TrimSpace(c.RazonSocial) == "" || ev.ConsentimientoAt.IsZero() {
		return nil
	}
	actual, err := leerCliente(ctx, tx, c.TipoIdentificacion, c.Identificacion)
	if err != nil {
		return err
	}
	final := c
	if actual != nil {
		final = Fusionar(*actual, c)
	}
	campos, _ := json.Marshal(final.CamposAt)
	nulo := func(s string) any {
		if s == "" {
			return nil
		}
		return s
	}
	if actual == nil {
		_, err = tx.Exec(ctx, `INSERT INTO clientes (id, tenant_id, tipo_identificacion, identificacion, razon_social, direccion, email, telefono, consentimiento_at, campos_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10) ON CONFLICT DO NOTHING`,
			c.ID, tenant, c.TipoIdentificacion, c.Identificacion, final.RazonSocial, nulo(final.Direccion), nulo(final.Email), nulo(final.Telefono), ev.ConsentimientoAt, campos)
		return err
	}
	_, err = tx.Exec(ctx, `UPDATE clientes SET razon_social = $2, direccion = $3, email = $4, telefono = $5, campos_at = $6,
		consentimiento_at = greatest(consentimiento_at, $7), updated_at = now(), version = version + 1 WHERE id = $1`,
		actual.ID, final.RazonSocial, nulo(final.Direccion), nulo(final.Email), nulo(final.Telefono), campos, ev.ConsentimientoAt)
	return err
}

// Clientes responde la búsqueda del nodo: primero la base del restaurante y, si hay, el
// proveedor externo.
type Clientes struct {
	DB        *db.DB
	Proveedor Proveedor
}

func (s *Clientes) Buscar(ctx context.Context, tenant ids.ID, tipo, identificacion string) (*Cliente, error) {
	identificacion = strings.ToUpper(strings.TrimSpace(identificacion))
	var c *Cliente
	err := s.DB.InTenant(ctx, tenant, func(tx db.Tx) error {
		var err error
		c, err = leerCliente(ctx, tx, tipo, identificacion)
		return err
	})
	if err != nil || c != nil || s.Proveedor == nil {
		return c, err
	}
	return s.Proveedor.Buscar(ctx, tipo, identificacion)
}
