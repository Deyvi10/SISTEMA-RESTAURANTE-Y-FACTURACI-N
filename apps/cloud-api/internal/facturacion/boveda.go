package facturacion

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/auditoria"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/auth"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/platform/apperr"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/platform/db"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/platform/httpx"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/ride"
	aud "github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/auditoria"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/ids"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/sri/sriws"
)

// Grupos de estado que ve el dueño (RF-05-07). Emitido local es lo que el nodo todavía no
// subió: la nube no lo conoce y el nodo lo muestra en su página de estado.
const (
	GrupoEnviado      = "ENVIADO"
	GrupoAutorizado   = "AUTORIZADO"
	GrupoNoAutorizado = "NO_AUTORIZADO"
	GrupoAtencion     = "REQUIERE_ATENCION"
	GrupoAnulado      = "ANULADO"
)

var estadosDeGrupo = map[string][]string{
	GrupoEnviado:      {"EN_NUBE", "FIRMADO", "ENVIADO", "RECIBIDO"},
	GrupoAutorizado:   {"AUTORIZADO"},
	GrupoNoAutorizado: {"NO_AUTORIZADO"},
	GrupoAtencion:     {"REQUIERE_ATENCION", "DEVUELTO"},
	GrupoAnulado:      {"ANULADO"},
}

func grupoDe(estado string) string {
	for g, es := range estadosDeGrupo {
		for _, e := range es {
			if e == estado {
				return g
			}
		}
	}
	return GrupoEnviado
}

// LectorArchivo lee un XML autorizado del archivo inmutable (cuando ya no está en PostgreSQL).
type LectorArchivo interface {
	Leer(ctx context.Context, tenant, comprobante ids.ID) ([]byte, error)
}

// Boveda es la bóveda de comprobantes del panel (F5-12).
type Boveda struct {
	DB      *db.DB
	Correos *Correos
	Archivo LectorArchivo
}

// FilaComprobante es una fila de la lista.
type FilaComprobante struct {
	ID              ids.ID       `json:"id"`
	Tipo            string       `json:"tipo"`
	Numero          string       `json:"numero"`
	ClaveAcceso     string       `json:"claveAcceso"`
	FechaEmision    string       `json:"fechaEmision"`
	Comprador       *string      `json:"comprador"`
	Identificacion  *string      `json:"identificacion"`
	ImporteTotal    string       `json:"importeTotal"`
	Ambiente        int          `json:"ambiente"`
	Estado          string       `json:"estado"`
	Grupo           string       `json:"grupo"`
	Serie           string       `json:"serie"`
	TieneCorreo     bool         `json:"tieneCorreo"`
	CorreoEnviado   bool         `json:"correoEnviado"`
	Explicacion     *Explicacion `json:"explicacion,omitempty"`
	FechaAutorizado *time.Time   `json:"fechaAutorizacion"`
	// Sustento: el número de la factura que modifica una nota de crédito.
	Sustento *string `json:"sustento,omitempty"`
}

// Explicacion del último mensaje del SRI o del motivo por el que espera.
type Explicacion = sriws.Explicacion

// Lista es una página de la bóveda más los contadores de pendientes.
type Lista struct {
	Filas      []FilaComprobante `json:"filas"`
	Siguiente  *string           `json:"siguiente"`
	Pendientes map[string]int    `json:"pendientes"`
}

// Filtro de la lista.
type Filtro struct {
	Desde, Hasta string // fecha de emisión, AAAA-MM-DD
	Grupo        string
	Texto        string // comprador (nombre o identificación), número o clave
	Tipo         string
	Punto        string
	Cursor       string
	Limite       int
}

const columnasFila = `c.id, c.tipo, c.serie, c.secuencial, c.clave_acceso, c.fecha_emision, c.comprador_nombre, c.comprador_identificacion,
	c.importe_total::text, c.ambiente, c.estado, c.correo_comprador IS NOT NULL,
	EXISTS (SELECT 1 FROM comprobante_correos k WHERE k.comprobante_id = c.id AND k.ok),
	c.mensajes_sri, c.ultimo_error, c.hash_valido, c.fecha_autorizacion, c.recibido_at,
	(SELECT substr(s.serie, 1, 3) || '-' || substr(s.serie, 4, 3) || '-' || lpad(s.secuencial::text, 9, '0') FROM comprobantes s WHERE s.id = c.sustento_id),
	(SELECT s.estado FROM comprobantes s WHERE s.id = c.sustento_id)`

func escanearFila(r pgx.Row) (FilaComprobante, time.Time, error) {
	var f FilaComprobante
	var sec int64
	var fecha time.Time
	var amb int16
	var mensajes []byte
	var ultimoError *string
	var hashValido bool
	var recibido time.Time
	var estadoSustento *string
	err := r.Scan(&f.ID, &f.Tipo, &f.Serie, &sec, &f.ClaveAcceso, &fecha, &f.Comprador, &f.Identificacion, &f.ImporteTotal, &amb,
		&f.Estado, &f.TieneCorreo, &f.CorreoEnviado, &mensajes, &ultimoError, &hashValido, &f.FechaAutorizado, &recibido, &f.Sustento, &estadoSustento)
	if err != nil {
		return f, recibido, err
	}
	f.Numero = fmt.Sprintf("%s-%s-%09d", f.Serie[:3], f.Serie[3:], sec)
	f.FechaEmision = fecha.Format(time.DateOnly)
	f.Ambiente = int(amb)
	f.Grupo = grupoDe(f.Estado)
	f.Explicacion = explicacionDe(f.Estado, mensajes, ultimoError, hashValido)
	// Una nota de crédito espera a que su factura esté autorizada antes de ir al SRI.
	if f.Sustento != nil && f.Estado == "EN_NUBE" && estadoSustento != nil && *estadoSustento != "AUTORIZADO" && *estadoSustento != "ANULADO" {
		f.Explicacion = &Explicacion{Que: "Espera a que el SRI autorice la factura " + *f.Sustento + " que modifica.",
			Accion: "Sale sola en cuanto la factura se autorice; si la factura tiene un problema, resuélvelo primero."}
	}
	return f, recibido, nil
}

// explicacionDe dice en lenguaje claro por qué un comprobante no está autorizado.
func explicacionDe(estado string, mensajes []byte, ultimoError *string, hashValido bool) *Explicacion {
	switch {
	case estado == "AUTORIZADO" || estado == "ANULADO":
		return nil
	case !hashValido:
		return &Explicacion{Que: "El comprobante llegó del Nodo Local con un contenido que no coincide con su código de integridad.",
			Accion: "No se firma ni se envía: contacta al soporte.", Soporte: true}
	}
	var ms []sriws.Mensaje
	_ = json.Unmarshal(mensajes, &ms)
	for _, m := range ms {
		if m.Tipo != "ADVERTENCIA" {
			e := sriws.Explicar(m)
			return &e
		}
	}
	if ultimoError != nil {
		if strings.Contains(*ultimoError, "firma electrónica") || strings.Contains(*ultimoError, "certificado") {
			return &Explicacion{Que: *ultimoError, Accion: "Sube tu firma electrónica vigente en Facturación SRI."}
		}
		return &Explicacion{Que: "El SRI no respondió en el último intento.", Accion: "Se reintenta solo; no hace falta nada."}
	}
	return nil
}

// Listar devuelve una página de comprobantes, del más reciente al más antiguo.
func (b *Boveda) Listar(ctx context.Context, p auth.Principal, f Filtro) (Lista, error) {
	if f.Limite <= 0 || f.Limite > 100 {
		f.Limite = 50
	}
	var conds []string
	var args []any
	arg := func(v any) string { args = append(args, v); return "$" + strconv.Itoa(len(args)) }
	if f.Desde != "" {
		if _, err := time.Parse(time.DateOnly, f.Desde); err != nil {
			return Lista{}, apperr.New(apperr.Invalid, "FILTRO_INVALIDO", "La fecha «desde» no es válida.")
		}
		conds = append(conds, "c.fecha_emision >= "+arg(f.Desde)+"::date")
	}
	if f.Hasta != "" {
		if _, err := time.Parse(time.DateOnly, f.Hasta); err != nil {
			return Lista{}, apperr.New(apperr.Invalid, "FILTRO_INVALIDO", "La fecha «hasta» no es válida.")
		}
		conds = append(conds, "c.fecha_emision <= "+arg(f.Hasta)+"::date")
	}
	if f.Grupo != "" {
		es, ok := estadosDeGrupo[f.Grupo]
		if !ok {
			return Lista{}, apperr.New(apperr.Invalid, "FILTRO_INVALIDO", "Estado desconocido.")
		}
		conds = append(conds, "c.estado = ANY("+arg(es)+")")
	}
	if f.Tipo != "" {
		conds = append(conds, "c.tipo = "+arg(f.Tipo))
	}
	if f.Punto != "" {
		id, err := ids.Parse(f.Punto)
		if err != nil {
			return Lista{}, apperr.New(apperr.Invalid, "FILTRO_INVALIDO", "Punto de emisión desconocido.")
		}
		conds = append(conds, "c.punto_emision_id = "+arg(id))
	}
	if t := strings.TrimSpace(f.Texto); t != "" {
		limpio := strings.ReplaceAll(t, "-", "")
		like := "%" + strings.NewReplacer("%", `\%`, "_", `\_`).Replace(t) + "%"
		q := arg(like)
		conds = append(conds, "(c.comprador_nombre ILIKE "+q+" OR c.comprador_identificacion LIKE "+arg(limpio+"%")+
			" OR c.clave_acceso = "+arg(limpio)+" OR (c.serie || lpad(c.secuencial::text, 9, '0')) LIKE "+arg("%"+limpio)+")")
	}
	if f.Cursor != "" {
		partes := strings.SplitN(f.Cursor, "_", 2)
		ns, err1 := strconv.ParseInt(partes[0], 10, 64)
		var id ids.ID
		var err2 error
		if len(partes) == 2 {
			id, err2 = ids.Parse(partes[1])
		}
		if err1 != nil || len(partes) != 2 || err2 != nil {
			return Lista{}, apperr.New(apperr.Invalid, "FILTRO_INVALIDO", "Página inválida.")
		}
		conds = append(conds, "(c.recibido_at, c.id) < ("+arg(time.Unix(0, ns).UTC())+", "+arg(id)+")")
	}
	where := ""
	if len(conds) > 0 {
		where = "WHERE " + strings.Join(conds, " AND ")
	}
	out := Lista{Filas: []FilaComprobante{}, Pendientes: map[string]int{}}
	err := b.DB.InTenant(ctx, p.TenantID, func(tx db.Tx) error {
		rows, err := tx.Query(ctx, `SELECT `+columnasFila+` FROM comprobantes c `+where+
			` ORDER BY c.recibido_at DESC, c.id DESC LIMIT `+arg(f.Limite+1), args...)
		if err != nil {
			return err
		}
		var ultimo time.Time
		for rows.Next() {
			fila, recibido, err := escanearFila(rows)
			if err != nil {
				rows.Close()
				return err
			}
			if len(out.Filas) == f.Limite {
				c := fmt.Sprintf("%d_%s", ultimo.UnixNano(), out.Filas[len(out.Filas)-1].ID)
				out.Siguiente = &c
				break
			}
			out.Filas, ultimo = append(out.Filas, fila), recibido
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return err
		}
		pend, err := tx.Query(ctx, `SELECT estado, count(*) FROM comprobantes
			WHERE estado NOT IN ('AUTORIZADO', 'ANULADO') GROUP BY estado`)
		if err != nil {
			return err
		}
		defer pend.Close()
		for pend.Next() {
			var e string
			var n int
			if err := pend.Scan(&e, &n); err != nil {
				return err
			}
			out.Pendientes[grupoDe(e)] += n
		}
		return pend.Err()
	})
	return out, err
}

// Detalle es un comprobante con su historia y sus correos.
type Detalle struct {
	FilaComprobante
	Correo   *string         `json:"correo"`
	Eventos  []EventoFiscal  `json:"eventos"`
	Correos  []CorreoEnvio   `json:"correos"`
	Mensajes []sriws.Mensaje `json:"mensajes"`
}

type EventoFiscal struct {
	Estado  string          `json:"estado"`
	Fecha   time.Time       `json:"fecha"`
	Detalle json.RawMessage `json:"detalle"`
}

type CorreoEnvio struct {
	Destino string    `json:"destino"`
	Motivo  string    `json:"motivo"`
	Ok      bool      `json:"ok"`
	Fecha   time.Time `json:"fecha"`
}

// Obtener devuelve el detalle de un comprobante.
func (b *Boveda) Obtener(ctx context.Context, p auth.Principal, id ids.ID) (Detalle, error) {
	var d Detalle
	err := b.DB.InTenant(ctx, p.TenantID, func(tx db.Tx) error {
		fila, _, err := escanearFila(tx.QueryRow(ctx, `SELECT `+columnasFila+` FROM comprobantes c WHERE c.id = $1`, id))
		if errors.Is(err, pgx.ErrNoRows) {
			return apperr.ErrNotFound
		}
		if err != nil {
			return err
		}
		d.FilaComprobante = fila
		var mensajes []byte
		if err := tx.QueryRow(ctx, `SELECT correo_comprador, mensajes_sri FROM comprobantes WHERE id = $1`, id).Scan(&d.Correo, &mensajes); err != nil {
			return err
		}
		d.Mensajes = []sriws.Mensaje{}
		_ = json.Unmarshal(mensajes, &d.Mensajes)
		rows, err := tx.Query(ctx, `SELECT estado, created_at, detalle FROM comprobante_eventos WHERE comprobante_id = $1 ORDER BY created_at, id`, id)
		if err != nil {
			return err
		}
		if d.Eventos, err = pgx.CollectRows(rows, pgx.RowToStructByPos[EventoFiscal]); err != nil {
			return err
		}
		rows, err = tx.Query(ctx, `SELECT destino, motivo, ok, created_at FROM comprobante_correos WHERE comprobante_id = $1 ORDER BY created_at DESC LIMIT 20`, id)
		if err != nil {
			return err
		}
		d.Correos, err = pgx.CollectRows(rows, pgx.RowToStructByPos[CorreoEnvio])
		return err
	})
	return d, err
}

// XML devuelve el XML autorizado: de PostgreSQL o, si ya salió de ahí, del archivo inmutable.
func (b *Boveda) XML(ctx context.Context, p auth.Principal, id ids.ID) (string, []byte, error) {
	var d Documentos
	var firmado bool
	err := b.DB.InTenant(ctx, p.TenantID, func(tx db.Tx) error {
		if err := tx.QueryRow(ctx, `SELECT xml_firmado IS NOT NULL FROM comprobantes WHERE id = $1`, id).Scan(&firmado); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return apperr.ErrNotFound
			}
			return err
		}
		if !firmado {
			return nil
		}
		var err error
		d, err = LeerDocumentos(ctx, tx, id)
		return err
	})
	if err != nil {
		return "", nil, err
	}
	if !firmado {
		if b.Archivo == nil {
			return "", nil, apperr.New(apperr.Conflict, "NO_AUTORIZADO", "La factura todavía no está autorizada por el SRI.")
		}
		doc, err := b.Archivo.Leer(ctx, p.TenantID, id)
		if errors.Is(err, apperr.ErrNotFound) {
			return "", nil, apperr.New(apperr.Conflict, "NO_AUTORIZADO", "La factura todavía no está autorizada por el SRI.")
		}
		return "comprobante.xml", doc, err
	}
	return d.Archivo + ".xml", d.XML, nil
}

// PDF devuelve el RIDE: el autorizado o, si aún no se autoriza, con «Pendiente de autorización».
func (b *Boveda) PDF(ctx context.Context, p auth.Principal, id ids.ID) (string, []byte, error) {
	var doc string
	var fecha *time.Time
	var hashValido bool
	var numero, tipo string
	err := b.DB.InTenant(ctx, p.TenantID, func(tx db.Tx) error {
		var serie string
		var sec int64
		err := tx.QueryRow(ctx, `SELECT coalesce(xml_firmado, xml), fecha_autorizacion, hash_valido, serie, secuencial, tipo FROM comprobantes WHERE id = $1`, id).
			Scan(&doc, &fecha, &hashValido, &serie, &sec, &tipo)
		if errors.Is(err, pgx.ErrNoRows) {
			return apperr.ErrNotFound
		}
		numero = fmt.Sprintf("%s-%s-%09d", serie[:3], serie[3:], sec)
		return err
	})
	if err != nil {
		return "", nil, err
	}
	if !hashValido {
		return "", nil, apperr.New(apperr.Conflict, "COMPROBANTE_ALTERADO", "Este comprobante llegó alterado del Nodo Local: no se genera su RIDE.")
	}
	pdf, err := ride.PDF([]byte(doc), ride.Autorizacion{Fecha: fecha})
	return NombreArchivo(tipo, numero) + ".pdf", pdf, err
}

// ReenvioInput: vacío reenvía al correo del comprador.
type ReenvioInput struct {
	Correo string `json:"correo"`
}

// Reenviar manda otra vez el correo al comprador (o a otro correo).
func (b *Boveda) Reenviar(ctx context.Context, p auth.Principal, id ids.ID, in ReenvioInput) error {
	destino := strings.TrimSpace(in.Correo)
	var v apperr.Validation
	v.Check(destino == "" || (len(destino) <= 200 && strings.Count(destino, "@") == 1 && !strings.ContainsAny(destino, " \r\n,;<>")), "correo", "Escribe un correo válido.")
	if err := v.Err(); err != nil {
		return err
	}
	autor := p.UserID
	return b.Correos.Enviar(ctx, p.TenantID, id, destino, CorreoReenvio, &autor)
}

// Reintentar vuelve a mandar al SRI un comprobante que espera o que el SRI no aceptó. Si fue
// DEVUELTO o NO AUTORIZADO se vuelve a firmar con la firma vigente y se envía con la misma
// clave y secuencial (ficha §5.10). Lo que llegó alterado del nodo nunca se envía.
func (b *Boveda) Reintentar(ctx context.Context, p auth.Principal, id ids.ID, _ struct{}) (FilaComprobante, error) {
	var out FilaComprobante
	err := b.DB.InTenant(ctx, p.TenantID, func(tx db.Tx) error {
		var estado string
		var hashValido bool
		err := tx.QueryRow(ctx, `SELECT estado, hash_valido FROM comprobantes WHERE id = $1 FOR UPDATE`, id).Scan(&estado, &hashValido)
		if errors.Is(err, pgx.ErrNoRows) {
			return apperr.ErrNotFound
		}
		if err != nil {
			return err
		}
		switch {
		case !hashValido:
			return apperr.New(apperr.Conflict, "COMPROBANTE_ALTERADO", "Este comprobante llegó alterado del Nodo Local y no se puede enviar: contacta al soporte.")
		case estado == "AUTORIZADO" || estado == "ANULADO":
			return apperr.New(apperr.Conflict, "YA_AUTORIZADO", "Este comprobante ya está autorizado.")
		case estado == "DEVUELTO" || estado == "NO_AUTORIZADO":
			_, err = tx.Exec(ctx, `UPDATE comprobantes SET estado = 'EN_NUBE', xml_firmado = NULL, intentos = 0, proximo_intento_at = NULL,
				ultimo_error = NULL, updated_at = now() WHERE id = $1`, id)
		default:
			_, err = tx.Exec(ctx, `UPDATE comprobantes SET intentos = 0, proximo_intento_at = NULL, updated_at = now() WHERE id = $1`, id)
		}
		if err != nil {
			return err
		}
		autor := p.UserID
		detalle, _ := json.Marshal(map[string]any{"reintentoManual": true, "estadoAnterior": estado, "usuario": autor})
		if _, err := tx.Exec(ctx, `INSERT INTO comprobante_eventos (id, tenant_id, comprobante_id, estado, detalle)
			SELECT $1, app_tenant(), id, estado, $2 FROM comprobantes WHERE id = $3`, ids.New(), detalle, id); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `SELECT pg_notify('comprobantes', $1)`, id.String()); err != nil {
			return err
		}
		if out, _, err = escanearFila(tx.QueryRow(ctx, `SELECT `+columnasFila+` FROM comprobantes c WHERE c.id = $1`, id)); err != nil {
			return err
		}
		return auditoria.Registrar(ctx, tx, p.TenantID, aud.Registro{UsuarioID: &autor, Accion: "COMPROBANTE_REINTENTADO",
			Entidad: "comprobante", EntidadID: id.String(), Antes: aud.Compactar(map[string]any{"estado": estado}),
			Despues: aud.Compactar(map[string]any{"estado": out.Estado})}, time.Now())
	})
	return out, err
}

// ---------- HTTP ----------

// HandleListar: GET /v1/comprobantes?desde&hasta&estado&q&tipo&punto&cursor
func (b *Boveda) HandleListar(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	limite, _ := strconv.Atoi(q.Get("limite"))
	l, err := b.Listar(r.Context(), auth.MustPrincipal(r.Context()), Filtro{Desde: q.Get("desde"), Hasta: q.Get("hasta"),
		Grupo: q.Get("estado"), Texto: q.Get("q"), Tipo: q.Get("tipo"), Punto: q.Get("punto"), Cursor: q.Get("cursor"), Limite: limite})
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, l)
}

// HandleDescargar sirve el XML o el PDF como archivo adjunto.
func (b *Boveda) HandleDescargar(formato string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := ids.Parse(r.PathValue("id"))
		if err != nil {
			httpx.Error(w, r, apperr.ErrNotFound)
			return
		}
		p := auth.MustPrincipal(r.Context())
		var nombre, tipo string
		var datos []byte
		if formato == "xml" {
			nombre, datos, err = b.XML(r.Context(), p, id)
			tipo = "application/xml"
		} else {
			nombre, datos, err = b.PDF(r.Context(), p, id)
			tipo = "application/pdf"
		}
		if err != nil {
			httpx.Error(w, r, err)
			return
		}
		w.Header().Set("Content-Type", tipo)
		w.Header().Set("Content-Disposition", `attachment; filename="`+nombre+`"`)
		w.Header().Set("Cache-Control", "private, no-store")
		_, _ = w.Write(datos)
	}
}
