package app

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/mail"
	"net/url"
	"strings"
	"time"

	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/edge-node/internal/nube"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/edge-node/internal/store"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/edgesync"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/ids"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/sri"
)

// EventoClienteGuardado lleva a la nube los campos de un cliente con la hora en que cambió
// cada uno: la nube se queda, campo por campo, con el más reciente (RF-04-05, F4-07).
const EventoClienteGuardado = "cliente.guardado"

// EsperaNubeCliente: la búsqueda en la nube tiene que responder a tiempo para autocompletar
// en ≤ 300 ms; si tarda más se sigue con el formulario (y la nube recibe el cliente después).
const EsperaNubeCliente = 250 * time.Millisecond

// camposCliente son los datos que se fusionan por campo.
var camposCliente = []string{"razonSocial", "direccion", "email", "telefono"}

// Cliente es un comprador identificado del restaurante.
type Cliente struct {
	ID                 ids.ID `json:"id"`
	TipoIdentificacion string `json:"tipoIdentificacion"` // 04 RUC, 05 cédula, 06 pasaporte, 08 exterior
	Identificacion     string `json:"identificacion"`
	RazonSocial        string `json:"razonSocial"`
	Direccion          string `json:"direccion"`
	Email              string `json:"email"`
	Telefono           string `json:"telefono"`
	// CamposAt: cuándo cambió cada campo (last-writer-wins por campo en la nube).
	CamposAt map[string]time.Time `json:"camposAt,omitempty"`
}

// BusquedaCliente es la respuesta del autocompletado.
type BusquedaCliente struct {
	Valida            bool     `json:"valida"`
	Tipo              string   `json:"tipo"`
	TipoContribuyente string   `json:"tipoContribuyente,omitempty"`
	Motivo            string   `json:"motivo,omitempty"`
	Advertencia       string   `json:"advertencia,omitempty"`
	Cliente           *Cliente `json:"cliente"`
	Origen            string   `json:"origen,omitempty"` // NODO, NUBE
}

// CompradorIn son los datos del comprador que manda la caja al cobrar.
type CompradorIn struct {
	TipoIdentificacion string `json:"tipoIdentificacion"` // vacío = se deduce del número
	Identificacion     string `json:"identificacion"`
	RazonSocial        string `json:"razonSocial"`
	Direccion          string `json:"direccion"`
	Email              string `json:"email"`
	Telefono           string `json:"telefono"`
	// Consentimiento para guardarlo para futuras compras (LOPDP). Sin él, los datos solo
	// van en el documento de esta venta.
	Consentimiento bool `json:"consentimiento"`
}

// validarIdentificacion aplica la regla del SRI; el pasaporte se elige explícitamente.
func validarIdentificacion(tipo, id string) sri.Validacion {
	id = strings.ToUpper(strings.TrimSpace(id))
	switch tipo {
	case string(sri.IdPasaporte):
		return sri.ValidarPasaporte(id)
	case string(sri.IdExterior):
		v := sri.ValidarPasaporte(id)
		v.Tipo = sri.IdExterior
		return v
	}
	return sri.ValidarIdentificacion(id)
}

// normalizar valida y limpia el comprador. Devuelve el tipo SRI resuelto.
func (c *CompradorIn) normalizar() (sri.Validacion, error) {
	c.Identificacion = strings.ToUpper(strings.TrimSpace(c.Identificacion))
	v := validarIdentificacion(c.TipoIdentificacion, c.Identificacion)
	if !v.Valida {
		return v, invalido(v.Motivo)
	}
	c.TipoIdentificacion = string(v.Tipo)
	if v.Tipo == sri.IdConsumidorFinal {
		return v, nil
	}
	c.RazonSocial = strings.Join(strings.Fields(c.RazonSocial), " ")
	c.Direccion = recortar(c.Direccion, 300)
	c.Telefono = recortar(c.Telefono, 20)
	c.Email = strings.ToLower(strings.TrimSpace(c.Email))
	if n := len([]rune(c.RazonSocial)); n < 2 || n > 300 {
		return v, invalido("Escribe el nombre o la razón social del comprador.")
	}
	if c.Email == "" {
		return v, invalido("Escribe el correo del comprador: ahí le llega su comprobante.")
	}
	if a, err := mail.ParseAddress(c.Email); err != nil || a.Address != c.Email || !strings.Contains(c.Email[strings.LastIndex(c.Email, "@"):], ".") {
		return v, invalido("El correo del comprador no es válido.")
	}
	return v, nil
}

func leerCliente(ctx context.Context, q queryer, tipo, id string) (*Cliente, error) {
	var c Cliente
	var cid, campos string
	var dir, email, tel sql.NullString
	err := q.QueryRowContext(ctx, `SELECT id, tipo_identificacion, identificacion, razon_social, direccion, email, telefono, campos_at FROM clientes
		WHERE identificacion = ? AND (? = '' OR tipo_identificacion = ?)`, id, tipo, tipo).
		Scan(&cid, &c.TipoIdentificacion, &c.Identificacion, &c.RazonSocial, &dir, &email, &tel, &campos)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	c.ID, _ = ids.Parse(cid)
	c.Direccion, c.Email, c.Telefono = dir.String, email.String, tel.String
	_ = json.Unmarshal([]byte(campos), &c.CamposAt)
	return &c, nil
}

// BuscarCliente: validación local y búsqueda en cascada, primero el nodo y luego la nube
// (con un tope de tiempo). Lo que llega de la nube queda en caché en el nodo.
func (a *App) BuscarCliente(ctx context.Context, tipo, identificacion string) (BusquedaCliente, error) {
	identificacion = strings.ToUpper(strings.TrimSpace(identificacion))
	v := validarIdentificacion(tipo, identificacion)
	out := BusquedaCliente{Valida: v.Valida, Tipo: string(v.Tipo), TipoContribuyente: string(v.TipoContribuyente), Motivo: v.Motivo, Advertencia: v.Advertencia}
	if !v.Valida || v.Tipo == sri.IdConsumidorFinal {
		return out, nil
	}
	c, err := leerCliente(ctx, a.Store.Read(), string(v.Tipo), identificacion)
	if err != nil {
		return out, err
	}
	if c != nil {
		out.Cliente, out.Origen = c, "NODO"
		return out, nil
	}
	a.syncMu.Lock()
	cli := a.nube
	a.syncMu.Unlock()
	if cli == nil {
		return out, nil
	}
	cctx, cancel := context.WithTimeout(ctx, EsperaNubeCliente)
	defer cancel()
	var nc Cliente
	q := url.Values{"tipo": {string(v.Tipo)}, "identificacion": {identificacion}}
	if err := cli.Do(cctx, http.MethodGet, "/v1/nodos/clientes?"+q.Encode(), nil, &nc, true); err != nil {
		var ne *nube.Error
		if !errors.As(err, &ne) || ne.Status != http.StatusNotFound {
			a.Log.Debug("cliente: la nube no respondió a tiempo", "err", err)
		}
		return out, nil
	}
	now := a.Clock.Now()
	if err := a.Store.Write(ctx, func(tx *store.Tx) error { return guardarCacheCliente(ctx, tx, nc, now) }); err != nil {
		return out, err
	}
	out.Cliente, out.Origen = &nc, "NUBE"
	return out, nil
}

// guardarCacheCliente guarda en el nodo un cliente que vino de la nube, sin pisar lo que el
// nodo cambió después (mismo criterio por campo que la nube).
func guardarCacheCliente(ctx context.Context, tx *store.Tx, c Cliente, now time.Time) error {
	local, err := leerCliente(ctx, tx, c.TipoIdentificacion, c.Identificacion)
	if err != nil {
		return err
	}
	ts := now.UTC().Format(time.RFC3339Nano)
	if local == nil {
		campos, _ := json.Marshal(c.CamposAt)
		_, err := tx.ExecContext(ctx, `INSERT INTO clientes (id, tipo_identificacion, identificacion, razon_social, direccion, email, telefono, consentimiento_at, campos_at, origen, created_at, updated_at)
			VALUES (?, ?, ?, ?, nullif(?, ''), nullif(?, ''), nullif(?, ''), ?, ?, 'NUBE', ?, ?)`,
			c.ID.String(), c.TipoIdentificacion, c.Identificacion, c.RazonSocial, c.Direccion, c.Email, c.Telefono, ts, string(campos), ts, ts)
		return err
	}
	fusion := fusionar(*local, c)
	campos, _ := json.Marshal(fusion.CamposAt)
	_, err = tx.ExecContext(ctx, `UPDATE clientes SET razon_social = ?, direccion = nullif(?, ''), email = nullif(?, ''), telefono = nullif(?, ''), campos_at = ?, updated_at = ? WHERE id = ?`,
		fusion.RazonSocial, fusion.Direccion, fusion.Email, fusion.Telefono, string(campos), ts, local.ID.String())
	return err
}

func campo(c *Cliente, k string) *string {
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

// fusionar combina dos versiones de un cliente campo por campo: gana la más reciente.
func fusionar(a, b Cliente) Cliente {
	out := a
	out.CamposAt = map[string]time.Time{}
	for _, k := range camposCliente {
		ta, tb := a.CamposAt[k], b.CamposAt[k]
		if tb.After(ta) {
			*campo(&out, k) = *campo(&b, k)
			out.CamposAt[k] = tb
		} else {
			out.CamposAt[k] = ta
		}
	}
	return out
}

// guardarClienteEnTx guarda (con consentimiento) el comprador de una venta. Solo marcan
// hora los campos que cambiaron, así la fusión en la nube no pisa datos más nuevos.
func (a *App) guardarClienteEnTx(ctx context.Context, tx *store.Tx, c CompradorIn, now time.Time) (Cliente, error) {
	local, err := leerCliente(ctx, tx, c.TipoIdentificacion, c.Identificacion)
	if err != nil {
		return Cliente{}, err
	}
	nuevo := Cliente{TipoIdentificacion: c.TipoIdentificacion, Identificacion: c.Identificacion, RazonSocial: c.RazonSocial,
		Direccion: c.Direccion, Email: c.Email, Telefono: c.Telefono, CamposAt: map[string]time.Time{}}
	ts := now.UTC()
	if local == nil {
		nuevo.ID = ids.New()
		for _, k := range camposCliente {
			if *campo(&nuevo, k) != "" {
				nuevo.CamposAt[k] = ts
			}
		}
	} else {
		nuevo.ID = local.ID
		for _, k := range camposCliente {
			if *campo(&nuevo, k) == *campo(local, k) {
				nuevo.CamposAt[k] = local.CamposAt[k]
			} else {
				nuevo.CamposAt[k] = ts
			}
		}
	}
	campos, _ := json.Marshal(nuevo.CamposAt)
	f := ts.Format(time.RFC3339Nano)
	if _, err := tx.ExecContext(ctx, `INSERT INTO clientes (id, tipo_identificacion, identificacion, razon_social, direccion, email, telefono, consentimiento_at, campos_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, nullif(?, ''), nullif(?, ''), nullif(?, ''), ?, ?, ?, ?)
		ON CONFLICT (tipo_identificacion, identificacion) DO UPDATE SET razon_social = excluded.razon_social, direccion = excluded.direccion,
			email = excluded.email, telefono = excluded.telefono, consentimiento_at = excluded.consentimiento_at, campos_at = excluded.campos_at, updated_at = excluded.updated_at`,
		nuevo.ID.String(), nuevo.TipoIdentificacion, nuevo.Identificacion, nuevo.RazonSocial, nuevo.Direccion, nuevo.Email, nuevo.Telefono, f, string(campos), f, f); err != nil {
		return Cliente{}, err
	}
	ev, err := edgesync.NewEvent(EventoClienteGuardado, 1, nuevo.ID, map[string]any{"cliente": nuevo, "consentimientoAt": ts}, now)
	if err != nil {
		return Cliente{}, err
	}
	_, err = a.outbox.Append(ctx, tx, ev)
	return nuevo, err
}
