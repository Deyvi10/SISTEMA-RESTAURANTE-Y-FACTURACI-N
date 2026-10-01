// Package facturacion guarda los datos tributarios del emisor y los puntos de emisión de cada
// caja (F5-02, pasos 3 a 5 del onboarding F5-06). La nube decide la configuración; el nodo
// dueño de cada punto asigna los secuenciales sin internet (docs/03 §3, docs/04 §7).
//
// Fuente normativa: ficha técnica offline v2.34 (documentacion-proyecto/docs/fuentes/sri/).
package facturacion

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/auditoria"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/auth"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/certificados"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/platform/apperr"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/platform/db"
	aud "github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/auditoria"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/ids"
)

type Service struct {
	DB *db.DB
}

// Regímenes (docs/04 §7). El negocio popular emite notas de venta: pendiente de DP-05.
const (
	General             = "GENERAL"
	RimpeEmprendedor    = "RIMPE_EMPRENDEDOR"
	RimpeNegocioPopular = "RIMPE_NEGOCIO_POPULAR"
)

// Config es lo que ve y edita el dueño en «Facturación SRI».
type Config struct {
	Guardada              bool    `json:"guardada"`
	Ambiente              int     `json:"ambiente"`
	RUC                   string  `json:"ruc"`
	RazonSocial           string  `json:"razonSocial"`
	NombreComercial       *string `json:"nombreComercial"`
	DireccionMatriz       string  `json:"direccionMatriz"`
	ObligadoContabilidad  bool    `json:"obligadoContabilidad"`
	ContribuyenteEspecial *string `json:"contribuyenteEspecial"`
	AgenteRetencion       *string `json:"agenteRetencion"`
	Regimen               string  `json:"regimen"`
	FacturacionActiva     bool    `json:"facturacionActiva"`
	Cajas                 []Punto `json:"cajas"`
	// Certificado activo (F5-07): solo sus datos visibles, nunca el archivo.
	Certificado *certificados.Info `json:"certificado"`
	// CambioProgramado de régimen o calificaciones, con su fecha de vigencia (F5-14).
	CambioProgramado *CambioRegimen `json:"cambioProgramado"`
	// PruebaAprobada: el SRI ya autorizó una factura de este restaurante en pruebas (F5-06 paso 6).
	PruebaAprobada bool `json:"pruebaAprobada"`
	// Pendientes explica en lenguaje claro qué falta para poder facturar.
	Pendientes []string `json:"pendientes"`
	// Avisos no impiden facturar, pero sin resolverlos los comprobantes no llegan al SRI.
	Avisos []string `json:"avisos"`
}

// Punto es una caja con su serie (establecimiento-punto).
type Punto struct {
	CajaID          ids.ID  `json:"cajaId"`
	Caja            string  `json:"caja"`
	LocalID         ids.ID  `json:"localId"`
	PuntoID         *ids.ID `json:"puntoId"`
	Establecimiento *string `json:"establecimiento"`
	PuntoEmision    *string `json:"puntoEmision"`
	ConNodo         bool    `json:"conNodo"` // el punto ya tiene un nodo dueño de su numeración
}

type ConfigInput struct {
	Ambiente              int     `json:"ambiente"`
	RazonSocial           string  `json:"razonSocial"`
	NombreComercial       *string `json:"nombreComercial"`
	DireccionMatriz       string  `json:"direccionMatriz"`
	ObligadoContabilidad  bool    `json:"obligadoContabilidad"`
	ContribuyenteEspecial *string `json:"contribuyenteEspecial"`
	AgenteRetencion       *string `json:"agenteRetencion"`
	Regimen               string  `json:"regimen"`
	FacturacionActiva     bool    `json:"facturacionActiva"`
}

// CambioRegimen rige desde la fecha (hora de Ecuador) para los comprobantes nuevos.
type CambioRegimen struct {
	Desde                 string  `json:"desde"` // AAAA-MM-DD
	Regimen               string  `json:"regimen"`
	ObligadoContabilidad  bool    `json:"obligadoContabilidad"`
	ContribuyenteEspecial *string `json:"contribuyenteEspecial"`
	AgenteRetencion       *string `json:"agenteRetencion"`
}

type PuntoInput struct {
	Establecimiento string `json:"establecimiento"`
	PuntoEmision    string `json:"puntoEmision"`
}

var (
	tresDigitos = regexp.MustCompile(`^[0-9]{3}$`)
	especial    = regexp.MustCompile(`^[A-Za-z0-9]{3,13}$`)
	agente      = regexp.MustCompile(`^[0-9]{1,8}$`)
)

// Obtener devuelve la configuración guardada o, la primera vez, una propuesta con los datos
// del alta del restaurante para que el dueño solo confirme.
func (s *Service) Obtener(ctx context.Context, p auth.Principal) (Config, error) {
	var c Config
	err := s.DB.InTenant(ctx, p.TenantID, func(tx db.Tx) error {
		var err error
		c, err = leer(ctx, tx)
		return err
	})
	return c, err
}

func leer(ctx context.Context, tx db.Tx) (Config, error) {
	c := Config{Cajas: []Punto{}, Pendientes: []string{}, Avisos: []string{}}
	// Llegada su fecha, el cambio programado pasa a ser la configuración actual.
	if _, err := tx.Exec(ctx, `UPDATE configuracion_fiscal SET regimen = cambio_regimen, obligado_contabilidad = cambio_obligado_contabilidad,
			contribuyente_especial = cambio_contribuyente_especial, agente_retencion = cambio_agente_retencion,
			cambio_desde = NULL, cambio_regimen = NULL, cambio_obligado_contabilidad = NULL, cambio_contribuyente_especial = NULL,
			cambio_agente_retencion = NULL, updated_at = now(), version = version + 1
		WHERE cambio_desde <= (now() AT TIME ZONE 'America/Guayaquil')::date`); err != nil {
		return c, err
	}
	var cambio CambioRegimen
	var desde *time.Time
	var cRegimen *string
	var cObligado *bool
	err := tx.QueryRow(ctx, `SELECT ambiente, ruc, razon_social, nombre_comercial, direccion_matriz, obligado_contabilidad,
		contribuyente_especial, agente_retencion, regimen, facturacion_activa,
		cambio_desde, cambio_regimen, cambio_obligado_contabilidad, cambio_contribuyente_especial, cambio_agente_retencion FROM configuracion_fiscal`).
		Scan(&c.Ambiente, &c.RUC, &c.RazonSocial, &c.NombreComercial, &c.DireccionMatriz, &c.ObligadoContabilidad,
			&c.ContribuyenteEspecial, &c.AgenteRetencion, &c.Regimen, &c.FacturacionActiva,
			&desde, &cRegimen, &cObligado, &cambio.ContribuyenteEspecial, &cambio.AgenteRetencion)
	if err == nil && desde != nil && cRegimen != nil && cObligado != nil {
		cambio.Desde, cambio.Regimen, cambio.ObligadoContabilidad = desde.Format(time.DateOnly), *cRegimen, *cObligado
		c.CambioProgramado = &cambio
	}
	switch {
	case err == nil:
		c.Guardada = true
	case errors.Is(err, pgx.ErrNoRows):
		var nombre string
		if err := tx.QueryRow(ctx, `SELECT ruc, razon_social, nombre_comercial,
			coalesce((SELECT nullif(direccion, '') FROM locales WHERE deleted_at IS NULL ORDER BY codigo_establecimiento LIMIT 1), '')
			FROM tenants WHERE id = app_tenant()`).Scan(&c.RUC, &c.RazonSocial, &nombre, &c.DireccionMatriz); err != nil {
			return c, err
		}
		c.NombreComercial, c.Ambiente, c.Regimen = &nombre, 1, General
	default:
		return c, err
	}
	rows, err := tx.Query(ctx, `SELECT c.id, c.nombre, c.local_id, p.id, p.codigo_establecimiento, p.codigo_punto, p.nodo_id IS NOT NULL
		FROM cajas c LEFT JOIN puntos_emision p ON p.id = c.punto_emision_id AND p.deleted_at IS NULL
		WHERE c.deleted_at IS NULL AND c.activa ORDER BY c.nombre`)
	if err != nil {
		return c, err
	}
	c.Cajas, err = pgx.CollectRows(rows, pgx.RowToStructByPos[Punto])
	if err != nil {
		return c, err
	}
	cert, err := certificados.Leer(ctx, tx, time.Now())
	switch {
	case err == nil:
		c.Certificado = &cert
	case !errors.Is(err, pgx.ErrNoRows):
		return c, err
	}
	if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM comprobantes WHERE ambiente = 1 AND estado = 'AUTORIZADO')`).Scan(&c.PruebaAprobada); err != nil {
		return c, err
	}
	c.Pendientes, c.Avisos = pendientes(c), avisos(c)
	return c, nil
}

// pendientes: lo que falta para facturar, en el orden en que el dueño lo resuelve.
func pendientes(c Config) []string {
	out := []string{}
	if !c.Guardada {
		out = append(out, "Confirma los datos del emisor y guárdalos.")
	}
	if c.Regimen == RimpeNegocioPopular {
		out = append(out, "Los negocios populares RIMPE emiten notas de venta: esa modalidad todavía no está disponible.")
	}
	for _, p := range c.Cajas {
		if p.PuntoID == nil {
			out = append(out, "Asigna un punto de emisión a «"+p.Caja+"».")
		}
	}
	return out
}

// avisos: la caja factura igual (el nodo emite sin internet), pero la nube no puede firmar.
func avisos(c Config) []string {
	out := []string{}
	switch {
	case c.Certificado == nil:
		out = append(out, "Sube tu firma electrónica (.p12) para que las facturas lleguen al SRI.")
	case c.Certificado.DiasRestantes < 0:
		out = append(out, "Tu firma electrónica venció: sube la renovada para que las facturas sigan llegando al SRI.")
	case c.Certificado.DiasRestantes <= 30:
		out = append(out, fmt.Sprintf("Tu firma electrónica vence en %d días: renuévala con tu entidad certificadora.", c.Certificado.DiasRestantes))
	}
	return out
}

func (in *ConfigInput) validar() error {
	in.RazonSocial = unaLinea(in.RazonSocial)
	in.DireccionMatriz = unaLinea(in.DireccionMatriz)
	in.NombreComercial = opcional(in.NombreComercial)
	in.ContribuyenteEspecial = opcional(in.ContribuyenteEspecial)
	in.AgenteRetencion = opcional(in.AgenteRetencion)
	var v apperr.Validation
	v.Check(in.Ambiente == 1 || in.Ambiente == 2, "ambiente", "Elige pruebas o producción.")
	v.Check(largo(in.RazonSocial, 300), "razonSocial", "Escribe la razón social tal como consta en el RUC (hasta 300 caracteres).")
	v.Check(largo(in.DireccionMatriz, 300), "direccionMatriz", "Escribe la dirección de la matriz (hasta 300 caracteres).")
	v.Check(in.NombreComercial == nil || largo(*in.NombreComercial, 300), "nombreComercial", "El nombre comercial va hasta 300 caracteres.")
	v.Check(in.ContribuyenteEspecial == nil || especial.MatchString(*in.ContribuyenteEspecial), "contribuyenteEspecial", "El número de resolución de contribuyente especial tiene de 3 a 13 letras o números.")
	v.Check(in.AgenteRetencion == nil || agente.MatchString(*in.AgenteRetencion), "agenteRetencion", "La resolución de agente de retención son hasta 8 números.")
	v.Check(in.Regimen == General || in.Regimen == RimpeEmprendedor || in.Regimen == RimpeNegocioPopular, "regimen", "Elige el régimen tributario.")
	return v.Err()
}

// Guardar confirma los datos del emisor. El RUC es siempre el del restaurante.
func (s *Service) Guardar(ctx context.Context, p auth.Principal, in ConfigInput) (Config, error) {
	if err := in.validar(); err != nil {
		return Config{}, err
	}
	var out Config
	err := s.DB.InTenant(ctx, p.TenantID, func(tx db.Tx) error {
		antes, err := leer(ctx, tx)
		if err != nil {
			return err
		}
		// Una vez en producción no se vuelve a pruebas: los clientes recibirían comprobantes sin
		// validez tributaria (F5-15). Si de verdad hace falta, lo hace el soporte.
		if antes.Ambiente == 2 && in.Ambiente == 1 {
			return apperr.New(apperr.Conflict, "YA_EN_PRODUCCION", "Ya facturas en producción: no se vuelve al ambiente de pruebas desde el panel. Escríbenos si de verdad lo necesitas.")
		}
		// Producción solo con la firma vigente y una factura de prueba autorizada (F5-06 paso 6).
		if in.Ambiente == 2 && antes.Ambiente != 2 &&
			(antes.Certificado == nil || antes.Certificado.DiasRestantes < 0 || !antes.PruebaAprobada) {
			return apperr.New(apperr.Conflict, "PRODUCCION_SIN_FIRMA", "El ambiente de producción se habilita cuando esté cargada tu firma electrónica y el SRI haya autorizado una factura de prueba.")
		}
		if in.FacturacionActiva {
			if in.Regimen == RimpeNegocioPopular {
				return apperr.New(apperr.Conflict, "NEGOCIO_POPULAR", "Los negocios populares RIMPE emiten notas de venta; esa modalidad todavía no está disponible.")
			}
			for _, c := range antes.Cajas {
				if c.PuntoID == nil {
					return apperr.New(apperr.Conflict, "CAJA_SIN_PUNTO", "Antes de facturar, asigna un punto de emisión a «"+c.Caja+"».")
				}
			}
		}
		_, err = tx.Exec(ctx, `INSERT INTO configuracion_fiscal AS c (tenant_id, ambiente, ruc, razon_social, nombre_comercial, direccion_matriz,
				obligado_contabilidad, contribuyente_especial, agente_retencion, regimen, facturacion_activa)
			SELECT app_tenant(), $1, t.ruc, $2, $3, $4, $5, $6, $7, $8, $9 FROM tenants t WHERE t.id = app_tenant()
			ON CONFLICT (tenant_id) DO UPDATE SET ambiente = excluded.ambiente, razon_social = excluded.razon_social,
				nombre_comercial = excluded.nombre_comercial, direccion_matriz = excluded.direccion_matriz,
				obligado_contabilidad = excluded.obligado_contabilidad, contribuyente_especial = excluded.contribuyente_especial,
				agente_retencion = excluded.agente_retencion, regimen = excluded.regimen, facturacion_activa = excluded.facturacion_activa,
				updated_at = now(), version = c.version + 1`,
			in.Ambiente, in.RazonSocial, in.NombreComercial, in.DireccionMatriz, in.ObligadoContabilidad,
			in.ContribuyenteEspecial, in.AgenteRetencion, in.Regimen, in.FacturacionActiva)
		if err != nil {
			return err
		}
		if out, err = leer(ctx, tx); err != nil {
			return err
		}
		autor := p.UserID
		if antes.Ambiente != out.Ambiente {
			if err := auditoria.Registrar(ctx, tx, p.TenantID, aud.Registro{UsuarioID: &autor, Accion: "AMBIENTE_CAMBIADO", Entidad: "configuracion_fiscal",
				EntidadID: p.TenantID.String(), Antes: aud.Compactar(map[string]any{"ambiente": antes.Ambiente}), Despues: aud.Compactar(map[string]any{"ambiente": out.Ambiente})}, time.Now()); err != nil {
				return err
			}
		}
		return auditoria.Registrar(ctx, tx, p.TenantID, aud.Registro{UsuarioID: &autor, Accion: "FACTURACION_CONFIGURADA",
			Entidad: "configuracion_fiscal", EntidadID: p.TenantID.String(), Antes: aud.Compactar(resumen(antes)), Despues: aud.Compactar(resumen(out))}, time.Now())
	})
	return out, err
}

// AsignarPunto da a una caja su serie. Cambiar la serie crea un punto nuevo: una serie nunca
// se renumera y los secuenciales del punto anterior quedan como estaban.
func (s *Service) AsignarPunto(ctx context.Context, p auth.Principal, caja ids.ID, in PuntoInput) (Config, error) {
	in.Establecimiento, in.PuntoEmision = strings.TrimSpace(in.Establecimiento), strings.TrimSpace(in.PuntoEmision)
	var v apperr.Validation
	v.Check(tresDigitos.MatchString(in.Establecimiento) && in.Establecimiento != "000", "establecimiento", "El establecimiento son 3 números, como 001.")
	v.Check(tresDigitos.MatchString(in.PuntoEmision) && in.PuntoEmision != "000", "puntoEmision", "El punto de emisión son 3 números, como 001.")
	if err := v.Err(); err != nil {
		return Config{}, err
	}
	var out Config
	err := s.DB.InTenant(ctx, p.TenantID, func(tx db.Tx) error {
		var local ids.ID
		var actual *ids.ID
		var est, pto *string
		err := tx.QueryRow(ctx, `SELECT c.local_id, p.id, p.codigo_establecimiento, p.codigo_punto FROM cajas c
			LEFT JOIN puntos_emision p ON p.id = c.punto_emision_id AND p.deleted_at IS NULL
			WHERE c.id = $1 AND c.deleted_at IS NULL`, caja).Scan(&local, &actual, &est, &pto)
		if errors.Is(err, pgx.ErrNoRows) {
			return apperr.ErrNotFound
		}
		if err != nil {
			return err
		}
		if actual != nil && *est == in.Establecimiento && *pto == in.PuntoEmision {
			out, err = leer(ctx, tx)
			return err
		}
		if actual != nil {
			if _, err := tx.Exec(ctx, `UPDATE puntos_emision SET deleted_at = now(), updated_at = now(), version = version + 1 WHERE id = $1`, *actual); err != nil {
				return err
			}
		}
		// Dueño de la numeración: el nodo activo del local (si ya hay uno; si no, el que se active).
		nuevo := ids.New()
		_, err = tx.Exec(ctx, `INSERT INTO puntos_emision (id, tenant_id, local_id, codigo_establecimiento, codigo_punto, nodo_id)
			VALUES ($1, app_tenant(), $2, $3, $4, (SELECT id FROM nodos WHERE local_id = $2 AND estado = 'ACTIVO' ORDER BY activado_at DESC LIMIT 1))`,
			nuevo, local, in.Establecimiento, in.PuntoEmision)
		if err := db.Translate(err, "puntos_emision_serie", "puntoEmision", "Esa serie ya la usa otra caja."); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE cajas SET punto_emision_id = $2, updated_at = now(), version = version + 1 WHERE id = $1`, caja, nuevo); err != nil {
			return err
		}
		if out, err = leer(ctx, tx); err != nil {
			return err
		}
		autor := p.UserID
		return auditoria.Registrar(ctx, tx, p.TenantID, aud.Registro{UsuarioID: &autor, Accion: "PUNTO_EMISION_ASIGNADO",
			Entidad: "caja", EntidadID: caja.String(), Despues: aud.Compactar(map[string]any{"serie": in.Establecimiento + "-" + in.PuntoEmision})}, time.Now())
	})
	return out, err
}

// ProgramarCambio deja programado un cambio de régimen o de calificaciones tributarias desde
// una fecha futura (RF-05-05: lo ya emitido no cambia).
func (s *Service) ProgramarCambio(ctx context.Context, p auth.Principal, in CambioRegimen) (Config, error) {
	in.ContribuyenteEspecial, in.AgenteRetencion = opcional(in.ContribuyenteEspecial), opcional(in.AgenteRetencion)
	desde, err := time.Parse(time.DateOnly, in.Desde)
	var v apperr.Validation
	hoy := time.Now().In(guayaquil).Format(time.DateOnly)
	v.Check(err == nil && in.Desde > hoy, "desde", "Elige una fecha a partir de mañana: lo ya emitido no cambia.")
	v.Check(err != nil || desde.Before(time.Now().AddDate(2, 0, 0)), "desde", "Programa el cambio dentro de los próximos dos años.")
	v.Check(in.Regimen == General || in.Regimen == RimpeEmprendedor || in.Regimen == RimpeNegocioPopular, "regimen", "Elige el régimen tributario.")
	v.Check(in.ContribuyenteEspecial == nil || especial.MatchString(*in.ContribuyenteEspecial), "contribuyenteEspecial", "El número de resolución de contribuyente especial tiene de 3 a 13 letras o números.")
	v.Check(in.AgenteRetencion == nil || agente.MatchString(*in.AgenteRetencion), "agenteRetencion", "La resolución de agente de retención son hasta 8 números.")
	if err := v.Err(); err != nil {
		return Config{}, err
	}
	var out Config
	err = s.DB.InTenant(ctx, p.TenantID, func(tx db.Tx) error {
		antes, err := leer(ctx, tx)
		if err != nil {
			return err
		}
		if !antes.Guardada {
			return apperr.New(apperr.Conflict, "SIN_CONFIGURAR", "Primero confirma los datos del emisor.")
		}
		if in.Regimen == RimpeNegocioPopular && antes.FacturacionActiva {
			return apperr.New(apperr.Conflict, "NEGOCIO_POPULAR", "Los negocios populares RIMPE emiten notas de venta; esa modalidad todavía no está disponible.")
		}
		if _, err := tx.Exec(ctx, `UPDATE configuracion_fiscal SET cambio_desde = $1, cambio_regimen = $2, cambio_obligado_contabilidad = $3,
				cambio_contribuyente_especial = $4, cambio_agente_retencion = $5, updated_at = now(), version = version + 1`,
			in.Desde, in.Regimen, in.ObligadoContabilidad, in.ContribuyenteEspecial, in.AgenteRetencion); err != nil {
			return err
		}
		if out, err = leer(ctx, tx); err != nil {
			return err
		}
		autor := p.UserID
		return auditoria.Registrar(ctx, tx, p.TenantID, aud.Registro{UsuarioID: &autor, Accion: "REGIMEN_PROGRAMADO", Entidad: "configuracion_fiscal",
			EntidadID: p.TenantID.String(), Antes: aud.Compactar(resumen(antes)), Despues: aud.Compactar(map[string]any{"desde": in.Desde, "regimen": in.Regimen,
				"obligadoContabilidad": in.ObligadoContabilidad, "contribuyenteEspecial": in.ContribuyenteEspecial, "agenteRetencion": in.AgenteRetencion})}, time.Now())
	})
	return out, err
}

// CancelarCambio quita el cambio programado (si su fecha aún no llega).
func (s *Service) CancelarCambio(ctx context.Context, p auth.Principal) (Config, error) {
	var out Config
	err := s.DB.InTenant(ctx, p.TenantID, func(tx db.Tx) error {
		antes, err := leer(ctx, tx)
		if err != nil {
			return err
		}
		if antes.CambioProgramado == nil {
			out = antes
			return nil
		}
		if _, err := tx.Exec(ctx, `UPDATE configuracion_fiscal SET cambio_desde = NULL, cambio_regimen = NULL, cambio_obligado_contabilidad = NULL,
				cambio_contribuyente_especial = NULL, cambio_agente_retencion = NULL, updated_at = now(), version = version + 1`); err != nil {
			return err
		}
		if out, err = leer(ctx, tx); err != nil {
			return err
		}
		autor := p.UserID
		return auditoria.Registrar(ctx, tx, p.TenantID, aud.Registro{UsuarioID: &autor, Accion: "REGIMEN_PROGRAMADO_CANCELADO", Entidad: "configuracion_fiscal",
			EntidadID: p.TenantID.String(), Antes: aud.Compactar(map[string]any{"desde": antes.CambioProgramado.Desde, "regimen": antes.CambioProgramado.Regimen})}, time.Now())
	})
	return out, err
}

var guayaquil = func() *time.Location {
	l, err := time.LoadLocation("America/Guayaquil")
	if err != nil {
		return time.FixedZone("ECT", -5*3600)
	}
	return l
}()

func resumen(c Config) map[string]any {
	return map[string]any{"ambiente": c.Ambiente, "razonSocial": c.RazonSocial, "direccionMatriz": c.DireccionMatriz,
		"obligadoContabilidad": c.ObligadoContabilidad, "regimen": c.Regimen, "facturacionActiva": c.FacturacionActiva}
}

func unaLinea(s string) string { return strings.Join(strings.Fields(s), " ") }

func opcional(s *string) *string {
	if s == nil {
		return nil
	}
	v := unaLinea(*s)
	if v == "" {
		return nil
	}
	return &v
}

func largo(s string, max int) bool { n := len([]rune(s)); return n >= 1 && n <= max }
