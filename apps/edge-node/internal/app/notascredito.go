package app

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/shopspring/decimal"

	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/edge-node/internal/impresion"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/edge-node/internal/store"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/escpos"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/ids"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/money"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/rbac"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/sri"
)

// ComprobanteCaja es una factura o nota de crédito como la ve la caja (F5-13, F5-18).
type ComprobanteCaja struct {
	ID             ids.ID `json:"id"`
	Tipo           string `json:"tipo"` // 01 factura, 04 nota de crédito
	Numero         string `json:"numero"`
	ClaveAcceso    string `json:"claveAcceso"`
	Emitido        string `json:"emitido"` // fecha y hora de emisión (RFC 3339)
	Total          string `json:"total"`
	Comprador      string `json:"comprador"`
	Identificacion string `json:"identificacion"`
	Ambiente       int    `json:"ambiente"`
	// Estado según la nube: EMITIDO (aún no llega), ENVIADO, AUTORIZADO, NO_AUTORIZADO,
	// REQUIERE_ATENCION o ANULADO.
	Estado        string  `json:"estado"`
	Mensaje       *string `json:"mensaje,omitempty"`
	Autorizacion  *string `json:"fechaAutorizacion,omitempty"`
	Sustento      *string `json:"sustento,omitempty"` // número de la factura que modifica (NC)
	SaldoRevertir *string `json:"saldo,omitempty"`    // lo que aún se puede revertir (factura)
}

const columnasComprobanteCaja = `c.id, c.tipo, c.serie, c.secuencial, c.clave_acceso, c.created_at, c.importe_total, c.ambiente, c.xml,
	e.estado, e.mensaje, e.fecha_autorizacion,
	(SELECT substr(s.serie, 1, 3) || '-' || substr(s.serie, 4, 3) || '-' || printf('%09d', s.secuencial) FROM comprobantes s WHERE s.id = c.sustento_id)`

func escanearComprobanteCaja(r interface{ Scan(...any) error }) (ComprobanteCaja, string, error) {
	var c ComprobanteCaja
	var id, serie, doc string
	var sec int64
	var estado, mensaje, fechaAut, sustento sql.NullString
	if err := r.Scan(&id, &c.Tipo, &serie, &sec, &c.ClaveAcceso, &c.Emitido, &c.Total, &c.Ambiente, &doc, &estado, &mensaje, &fechaAut, &sustento); err != nil {
		return c, "", err
	}
	c.ID, _ = ids.Parse(id)
	c.Numero = fmt.Sprintf("%s-%s-%09d", serie[:3], serie[3:], sec)
	c.Estado = EstadoFiscal(estado.String)
	if !estado.Valid {
		c.Estado = "EMITIDO"
	} else if estado.String == "ANULADO" {
		c.Estado = "ANULADO"
	}
	if mensaje.Valid {
		c.Mensaje = &mensaje.String
	}
	if fechaAut.Valid && fechaAut.String != "" {
		c.Autorizacion = &fechaAut.String
	}
	if sustento.Valid {
		c.Sustento = &sustento.String
	}
	if l, err := sri.LeerComprobante([]byte(doc)); err == nil {
		c.Comprador, c.Identificacion = l.RazonSocialComprador, l.IdComprador
	}
	return c, doc, nil
}

// ComprobantesCaja busca facturas y notas de crédito del nodo por número, cliente o clave, y
// opcionalmente por fecha de emisión (AAAA-MM-DD).
func (a *App) ComprobantesCaja(ctx context.Context, u Usuario, q, fecha string) ([]ComprobanteCaja, error) {
	if !u.Puede(rbac.Cobrar) && !u.Puede(rbac.EmitirNC) {
		return nil, problema(http.StatusForbidden, "SIN_PERMISO", "No tienes permiso para ver los comprobantes.")
	}
	q = strings.TrimSpace(q)
	limpio := strings.ReplaceAll(q, "-", "")
	conds, args := []string{}, []any{}
	if q != "" {
		// El comprador está dentro del XML: se busca por número, clave o texto del XML.
		conds = append(conds, `(c.clave_acceso = ? OR (c.serie || printf('%09d', c.secuencial)) LIKE ? OR c.xml LIKE ?)`)
		args = append(args, limpio, "%"+limpio, "%"+q+"%")
	}
	if fecha != "" {
		if _, err := time.Parse(time.DateOnly, fecha); err != nil {
			return nil, invalido("La fecha no es válida.")
		}
		conds = append(conds, `c.fecha_emision = ?`)
		args = append(args, fecha)
	}
	where := ""
	if len(conds) > 0 {
		where = "WHERE " + strings.Join(conds, " AND ")
	}
	rows, err := a.Store.Read().QueryContext(ctx, `SELECT `+columnasComprobanteCaja+` FROM comprobantes c
		LEFT JOIN estados_comprobante e ON e.id = c.id `+where+` ORDER BY c.created_at DESC LIMIT 50`, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := []ComprobanteCaja{}
	for rows.Next() {
		c, _, err := escanearComprobanteCaja(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// LineaRevertible es una línea de la factura con lo que aún se puede devolver.
type LineaRevertible struct {
	Indice      int    `json:"indice"`
	Descripcion string `json:"descripcion"`
	Cantidad    string `json:"cantidad"`
	Disponible  string `json:"disponible"`
	Total       string `json:"total"` // con IVA, de la línea completa
}

// DetalleComprobanteCaja es la factura con sus líneas y sus notas de crédito.
type DetalleComprobanteCaja struct {
	ComprobanteCaja
	ConsumidorFinal bool              `json:"consumidorFinal"`
	Lineas          []LineaRevertible `json:"lineas"`
	NotasCredito    []ComprobanteCaja `json:"notasCredito"`
}

// DetalleComprobanteCajaDe devuelve la factura para revertirla (o la NC para reimprimirla).
func (a *App) DetalleComprobanteCajaDe(ctx context.Context, u Usuario, id ids.ID) (DetalleComprobanteCaja, error) {
	var d DetalleComprobanteCaja
	if !u.Puede(rbac.Cobrar) && !u.Puede(rbac.EmitirNC) {
		return d, problema(http.StatusForbidden, "SIN_PERMISO", "No tienes permiso para ver los comprobantes.")
	}
	q := a.Store.Read()
	c, doc, err := escanearComprobanteCaja(q.QueryRowContext(ctx, `SELECT `+columnasComprobanteCaja+` FROM comprobantes c
		LEFT JOIN estados_comprobante e ON e.id = c.id WHERE c.id = ?`, id.String()))
	if errors.Is(err, sql.ErrNoRows) {
		return d, problema(http.StatusNotFound, "NO_ENCONTRADO", "Ese comprobante no existe en este local.")
	}
	if err != nil {
		return d, err
	}
	d.ComprobanteCaja, d.Lineas, d.NotasCredito = c, []LineaRevertible{}, []ComprobanteCaja{}
	if c.Tipo != sri.TipoFactura {
		return d, nil
	}
	f, err := sri.LeerFactura([]byte(doc))
	if err != nil {
		return d, err
	}
	d.ConsumidorFinal = f.TipoIdComprador == string(sri.IdConsumidorFinal)
	previas, err := revertidoDe(ctx, q, id)
	if err != nil {
		return d, err
	}
	saldo, err := sri.Saldo(f, previas)
	if err != nil {
		return d, err
	}
	s := saldo.String()
	d.SaldoRevertir = &s
	ya := map[int]decimal.Decimal{}
	for _, p := range previas {
		ya[p.Indice] = ya[p.Indice].Add(p.Cantidad)
	}
	for i, det := range f.Detalles {
		cant, _ := decimal.NewFromString(det.Cantidad)
		total, _ := money.Parse(det.PrecioTotalSinImpuesto)
		for _, imp := range det.Impuestos {
			v, _ := money.Parse(imp.Valor)
			total = total.Add(v)
		}
		d.Lineas = append(d.Lineas, LineaRevertible{Indice: i, Descripcion: det.Descripcion, Cantidad: cant.String(),
			Disponible: cant.Sub(ya[i]).String(), Total: total.String()})
	}
	rows, err := q.QueryContext(ctx, `SELECT `+columnasComprobanteCaja+` FROM comprobantes c LEFT JOIN estados_comprobante e ON e.id = c.id
		WHERE c.sustento_id = ? ORDER BY c.created_at`, id.String())
	if err != nil {
		return d, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		nc, _, err := escanearComprobanteCaja(rows)
		if err != nil {
			return d, err
		}
		d.NotasCredito = append(d.NotasCredito, nc)
	}
	return d, rows.Err()
}

func revertidoDe(ctx context.Context, q queryer, factura ids.ID) ([]sri.Revertido, error) {
	rows, err := q.QueryContext(ctx, `SELECT indice, cantidad, total, descuento, iva FROM nc_lineas WHERE factura_id = ?`, factura.String())
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []sri.Revertido
	for rows.Next() {
		var r sri.Revertido
		var cant, total, desc, iva string
		if err := rows.Scan(&r.Indice, &cant, &total, &desc, &iva); err != nil {
			return nil, err
		}
		var e1, e2, e3, e4 error
		r.Cantidad, e1 = decimal.NewFromString(cant)
		r.Total, e2 = money.Parse(total)
		r.Descuento, e3 = money.Parse(desc)
		r.IVA, e4 = money.Parse(iva)
		if err := errors.Join(e1, e2, e3, e4); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// NotaCreditoIn es lo que manda la caja para revertir una factura.
type NotaCreditoIn struct {
	CajaID ids.ID `json:"cajaId"`
	Lineas []struct {
		Indice   int    `json:"indice"`
		Cantidad string `json:"cantidad"`
	} `json:"lineas"`
	Todo   bool   `json:"todo"`
	Motivo string `json:"motivo"`
	// Comprador: obligatorio si la factura fue a consumidor final (la NC no lo admite).
	Comprador *CompradorIn `json:"comprador"`
	// Devolucion: el método por el que se devuelve el dinero (nil: no se devuelve dinero,
	// p. ej. una factura mal emitida que el cliente no pagó de más).
	Devolucion *struct {
		MetodoID ids.ID `json:"metodoId"`
	} `json:"devolucion"`
	DevolverInventario bool   `json:"devolverInventario"` // solo el Administrador lo indica
	Autorizacion       string `json:"autorizacion"`       // PIN de supervisor si el usuario no tiene EMITIR_NC
	IdempotencyKey     string `json:"idempotencyKey"`
}

// NotaCreditoOut es la nota emitida.
type NotaCreditoOut struct {
	ComprobanteCaja
	Valor         string   `json:"valor"`
	RevierteTodo  bool     `json:"revierteTodo"` // la factura quedó revertida por completo
	Devuelto      *string  `json:"devuelto,omitempty"`
	Metodo        string   `json:"metodo,omitempty"`
	Impresoras    []string `json:"impresoras"`
	AutorizadoPor string   `json:"autorizadoPor,omitempty"`
	Aviso         string   `json:"aviso,omitempty"`
}

// EmitirNotaCredito revierte total o parcialmente una factura (F5-13, RF-05-08). El nodo
// numera la nota (es dueño del punto de emisión) en la misma transacción que registra lo
// revertido y el dinero devuelto; la nube la firma y la envía al SRI cuando la factura ya
// está autorizada. Funciona sin internet.
func (a *App) EmitirNotaCredito(ctx context.Context, d Dispositivo, u Usuario, factura ids.ID, in NotaCreditoIn) (NotaCreditoOut, error) {
	var out NotaCreditoOut
	in.IdempotencyKey = strings.TrimSpace(in.IdempotencyKey)
	if len(in.IdempotencyKey) < 8 || len(in.IdempotencyKey) > 64 {
		return out, invalido("Falta la clave de idempotencia de la nota de crédito.")
	}
	in.Motivo = strings.Join(strings.Fields(in.Motivo), " ")
	if n := len([]rune(in.Motivo)); n < 3 || n > 300 {
		return out, invalido("Escribe el motivo de la nota de crédito (de 3 a 300 caracteres).")
	}
	if in.DevolverInventario && u.Rol != string(rbac.Admin) {
		return out, problema(http.StatusForbidden, "SOLO_ADMIN", "Solo el administrador indica que se devuelva al inventario.")
	}
	var devolver []sri.Devolucion
	for _, l := range in.Lineas {
		c, err := decimal.NewFromString(strings.TrimSpace(l.Cantidad))
		if err != nil || !c.IsPositive() || c.Exponent() < -6 {
			return out, invalido("La cantidad a devolver no es válida.")
		}
		devolver = append(devolver, sri.Devolucion{Indice: l.Indice, Cantidad: c})
	}
	if !in.Todo && len(devolver) == 0 {
		return out, invalido("Elige qué se devuelve o marca la factura completa.")
	}
	now, loc := a.Clock.Now(), a.zonaLocal(ctx)
	var despertar []ids.ID
	err := a.Store.Write(ctx, func(tx *store.Tx) error {
		// Reintento del mismo pedido: se devuelve la nota ya emitida.
		var existente string
		if err := tx.QueryRowContext(ctx, `SELECT id FROM notas_credito WHERE idempotency_key = ?`, in.IdempotencyKey).Scan(&existente); err == nil {
			c, _, err := escanearComprobanteCaja(tx.QueryRowContext(ctx, `SELECT `+columnasComprobanteCaja+` FROM comprobantes c
				LEFT JOIN estados_comprobante e ON e.id = c.id WHERE c.id = ?`, existente))
			out.ComprobanteCaja, out.Valor, out.Impresoras = c, c.Total, []string{}
			return err
		} else if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		var autoriza *ids.ID
		if !u.Puede(rbac.EmitirNC) {
			if in.Autorizacion == "" {
				return problema(http.StatusForbidden, "REQUIERE_SUPERVISOR", "Emitir una nota de crédito requiere el PIN de un supervisor.")
			}
			por, err := consumirAutorizacion(ctx, tx, in.Autorizacion, d, rbac.EmitirNC, factura.String(), now)
			if err != nil {
				return err
			}
			autoriza = &por
			_ = tx.QueryRowContext(ctx, `SELECT nombre_mostrar FROM usuarios WHERE id = ?`, por.String()).Scan(&out.AutorizadoPor)
		}
		cfg, err := leerConfigFiscalSiempre(ctx, tx)
		if err != nil {
			return err
		}
		if cfg == nil {
			return problema(http.StatusConflict, "SIN_FACTURACION", "Este local no tiene la facturación electrónica configurada.")
		}
		var tipo, doc string
		var ambiente int
		err = tx.QueryRowContext(ctx, `SELECT tipo, xml, ambiente FROM comprobantes WHERE id = ?`, factura.String()).Scan(&tipo, &doc, &ambiente)
		if errors.Is(err, sql.ErrNoRows) {
			return problema(http.StatusNotFound, "NO_ENCONTRADO", "Esa factura no existe en este local.")
		}
		if err != nil {
			return err
		}
		if tipo != sri.TipoFactura {
			return problema(http.StatusConflict, "NO_ES_FACTURA", "Solo se emite una nota de crédito sobre una factura.")
		}
		f, err := sri.LeerFactura([]byte(doc))
		if err != nil {
			return err
		}
		previas, err := revertidoDe(ctx, tx, factura)
		if err != nil {
			return err
		}
		nc, err := sri.CalcularNC(f, devolver, previas, in.Todo)
		if err != nil {
			if errors.Is(err, sri.ErrNotaCredito) {
				return problema(http.StatusConflict, "NC_INVALIDA", strings.TrimPrefix(err.Error(), sri.ErrNotaCredito.Error()+": "))
			}
			return err
		}
		// La NC no admite consumidor final: si la factura fue a consumidor final, el cliente
		// se identifica ahora (ficha técnica, nota de la Tabla 6).
		comprador := sri.Comprador{TipoIdentificacion: f.TipoIdComprador, Identificacion: f.IdComprador, RazonSocial: f.RazonSocialComprador,
			Direccion: f.DireccionComprador}
		email := f.Adicional("Email")
		if f.TipoIdComprador == string(sri.IdConsumidorFinal) {
			if in.Comprador == nil {
				return problema(http.StatusUnprocessableEntity, "REQUIERE_CLIENTE", "La factura fue a consumidor final: para la nota de crédito, identifica al cliente.")
			}
			v, err := in.Comprador.normalizar()
			if err != nil {
				return err
			}
			if v.Tipo == sri.IdConsumidorFinal {
				return problema(http.StatusUnprocessableEntity, "REQUIERE_CLIENTE", "La nota de crédito no puede ir a consumidor final: identifica al cliente.")
			}
			comprador = sri.Comprador{TipoIdentificacion: in.Comprador.TipoIdentificacion, Identificacion: in.Comprador.Identificacion,
				RazonSocial: in.Comprador.RazonSocial, Direccion: in.Comprador.Direccion}
			email = in.Comprador.Email
		}
		punto, err := puntoDeCaja(ctx, tx, in.CajaID)
		if err != nil {
			return err
		}
		caja, err := cajaExiste(ctx, tx, in.CajaID)
		if err != nil {
			return err
		}
		// Devolución del dinero: del turno abierto de esta caja, por el método que se elija.
		var turno string
		var metodoNombre string
		if in.Devolucion != nil {
			if err := tx.QueryRowContext(ctx, `SELECT id FROM turnos_caja WHERE caja_id = ? AND estado = 'ABIERTO'`, in.CajaID.String()).Scan(&turno); err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					return problema(http.StatusConflict, "SIN_TURNO", "Para devolver dinero, abre el turno de la caja.")
				}
				return err
			}
			if err := tx.QueryRowContext(ctx, `SELECT nombre FROM metodos_pago WHERE id = ? AND activo = 1 AND deleted_at IS NULL`,
				in.Devolucion.MetodoID.String()).Scan(&metodoNombre); err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					return invalido("El método de devolución no existe o está inactivo.")
				}
				return err
			}
		}
		sec, err := siguienteSecuencial(ctx, tx, punto.ID, sri.TipoNotaCredito, ambiente)
		if err != nil {
			return err
		}
		fecha := now.In(loc)
		vigente := cfg.vigenteEn(fecha)
		cfg = &vigente
		clave, err := sri.NuevaClaveAcceso(sri.ClaveAccesoInput{FechaEmision: fecha, TipoComprobante: sri.TipoNotaCredito, RUC: cfg.RUC,
			Ambiente: sri.Ambiente(ambiente), Establecimiento: punto.Establecimiento, PuntoEmision: punto.Punto, Secuencial: sec})
		if err != nil {
			return err
		}
		var dirLocal, rucProveedor string
		_ = tx.QueryRowContext(ctx, `SELECT direccion FROM locales LIMIT 1`).Scan(&dirLocal)
		_ = tx.QueryRowContext(ctx, `SELECT valor FROM parametros_globales WHERE clave = 'ruc_proveedor_sistema'`).Scan(&rucProveedor)
		datos := sri.DatosNC{Ambiente: sri.Ambiente(ambiente), ClaveAcceso: clave, Secuencial: sec, Fecha: fecha,
			Emisor: sri.Emisor{RUC: cfg.RUC, RazonSocial: cfg.RazonSocial, NombreComercial: cfg.NombreComercial, DirMatriz: cfg.DirMatriz,
				DirEstablecimiento: dirLocal, ObligadoContabilidad: cfg.ObligadoContabilidad, ContribuyenteEspecial: cfg.ContribuyenteEspecial,
				AgenteRetencion: cfg.AgenteRetencion, RIMPE: cfg.Regimen == "RIMPE_EMPRENDEDOR", Establecimiento: punto.Establecimiento, PuntoEmision: punto.Punto},
			Comprador: comprador, Sustento: f, Motivo: in.Motivo,
			Adicionales: []sri.CampoAdicional{{Nombre: "Email", Valor: email}}}
		if strings.TrimSpace(rucProveedor) != "" {
			datos.Adicionales = append(datos.Adicionales, sri.CampoAdicional{Nombre: "RUC Proveedor", Valor: strings.TrimSpace(rucProveedor)})
		}
		xmlNC, err := sri.NotaCreditoXML(datos, nc)
		if err != nil {
			return err
		}
		suma := sha256.Sum256(xmlNC)
		hash := hex.EncodeToString(suma[:])
		id := ids.New()
		numero := fmt.Sprintf("%s-%s-%09d", punto.Establecimiento, punto.Punto, sec)
		if _, err := tx.ExecContext(ctx, `INSERT INTO comprobantes (id, tipo, ambiente, punto_emision_id, serie, secuencial, clave_acceso,
			fecha_emision, importe_total, xml, hash, created_at, sustento_id) VALUES (?, '04', ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			id.String(), ambiente, punto.ID.String(), punto.Serie(), sec, clave.String(), fecha.Format(time.DateOnly),
			nc.ValorModificacion.String(), string(xmlNC), hash, now.Format(time.RFC3339Nano), factura.String()); err != nil {
			return err
		}
		uid := u.ID
		var porID *string
		if autoriza != nil {
			s := autoriza.String()
			porID = &s
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO notas_credito (id, factura_id, motivo, devolver_inventario, emitida_por, autorizada_por, caja_id,
			idempotency_key, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`, id.String(), factura.String(), in.Motivo, map[bool]int{true: 1, false: 0}[in.DevolverInventario],
			uid.String(), porID, in.CajaID.String(), in.IdempotencyKey, now.Format(time.RFC3339Nano)); err != nil {
			return err
		}
		for _, l := range nc.Lineas {
			if _, err := tx.ExecContext(ctx, `INSERT INTO nc_lineas (nota_credito_id, factura_id, indice, cantidad, total, descuento, iva) VALUES (?, ?, ?, ?, ?, ?, ?)`,
				id.String(), factura.String(), l.Indice, l.Cantidad.String(), l.PrecioTotalSinImpuesto.String(), l.Descuento.String(), l.IVA.String()); err != nil {
				return err
			}
		}
		var devolucion map[string]any
		if in.Devolucion != nil {
			if _, err := tx.ExecContext(ctx, `INSERT INTO devoluciones_nc (id, nota_credito_id, turno_id, metodo_pago_id, monto, created_at) VALUES (?, ?, ?, ?, ?, ?)`,
				ids.New().String(), id.String(), turno, in.Devolucion.MetodoID.String(), nc.ValorModificacion.String(), now.Format(time.RFC3339Nano)); err != nil {
				return err
			}
			v := nc.ValorModificacion.String()
			out.Devuelto, out.Metodo = &v, metodoNombre
			devolucion = map[string]any{"metodoId": in.Devolucion.MetodoID, "metodo": metodoNombre, "monto": v, "turnoId": turno}
		}
		// Ticket RIDE de la nota en la impresora de la caja.
		var estacion sql.NullString
		_ = tx.QueryRowContext(ctx, `SELECT estacion_id FROM cajas WHERE id = ?`, in.CajaID.String()).Scan(&estacion)
		var est *ids.ID
		if e, err := ids.Parse(estacion.String); err == nil {
			est = &e
		}
		imps, respaldo, err := a.impresorasDeCaja(ctx, tx, est)
		if err != nil {
			return err
		}
		switch {
		case respaldo != "":
			out.Aviso = "La caja no tiene impresora: la nota de crédito salió en «" + respaldo + "»."
		case len(imps) == 0:
			out.Aviso = "No hay impresoras configuradas: la nota de crédito quedó registrada, pero no se imprimió."
		}
		ride := rideNotaCredito(cfg, datos, nc, numero, caja, u.Nombre, dirLocal)
		ride.AbrirCajon = devolucion != nil
		out.Impresoras = []string{}
		for i, imp := range imps {
			r := ride
			r.AbrirCajon = ride.AbrirCajon && i == 0
			if err := impresion.Encolar(ctx, tx, impresion.Trabajo{ID: ids.New(), ImpresoraID: imp.ID, Tipo: "VENTA"}, nil, escpos.ImprimirRide(imp.Ancho, r), nil, now); err != nil {
				return err
			}
			out.Impresoras = append(out.Impresoras, imp.Nombre)
			despertar = append(despertar, imp.ID)
		}
		if err := auditar(ctx, tx, "NOTA_CREDITO_EMITIDA", "comprobante", id, &uid, map[string]any{"factura": f.Numero(), "notaCredito": numero,
			"valor": nc.ValorModificacion.String(), "motivo": in.Motivo, "total": nc.Total, "autorizadoPor": autoriza, "devolucion": devolucion,
			"devolverInventario": in.DevolverInventario}, now); err != nil {
			return err
		}
		out.ComprobanteCaja = ComprobanteCaja{ID: id, Tipo: sri.TipoNotaCredito, Numero: numero, ClaveAcceso: clave.String(), Emitido: now.Format(time.RFC3339Nano),
			Total: nc.ValorModificacion.String(), Comprador: comprador.RazonSocial, Identificacion: comprador.Identificacion, Ambiente: ambiente, Estado: "EMITIDO"}
		sustento := f.Numero()
		out.Sustento, out.Valor, out.RevierteTodo = &sustento, nc.ValorModificacion.String(), nc.Total
		// La nube la firma y la envía cuando su factura ya esté autorizada.
		return a.eventoCaja(ctx, tx, EventoComprobanteEmitido, id, map[string]any{
			"id": id, "tipo": sri.TipoNotaCredito, "ambiente": ambiente, "puntoEmisionId": punto.ID, "serie": punto.Serie(), "secuencial": sec,
			"claveAcceso": clave.String(), "fechaEmision": fecha.Format(time.DateOnly), "importeTotal": nc.ValorModificacion.String(),
			"xml": string(xmlNC), "hash": hash, "sustentoId": factura, "sustentoClave": f.ClaveAcceso, "total": nc.Total,
			"devolucion": devolucion, "devolverInventario": in.DevolverInventario}, now)
	})
	if err != nil {
		return NotaCreditoOut{}, err
	}
	a.motor.Despertar(despertar...)
	a.notificarPush()
	return out, nil
}

// rideNotaCredito arma el ticket RIDE de la nota de crédito (Anexo 2).
func rideNotaCredito(cfg *configFiscal, d sri.DatosNC, nc sri.NotaCredito, numero, caja, cajero, dirLocal string) escpos.Ride {
	r := escpos.Ride{
		NombreComercial: cfg.NombreComercial, RazonSocial: cfg.RazonSocial, RUC: cfg.RUC, DirMatriz: cfg.DirMatriz, DirEstablecimiento: dirLocal,
		ContribuyenteEspecial: cfg.ContribuyenteEspecial, ObligadoContabilidad: cfg.ObligadoContabilidad, AgenteRetencion: cfg.AgenteRetencion,
		RIMPE: cfg.Regimen == "RIMPE_EMPRENDEDOR", Numero: numero, ClaveAcceso: d.ClaveAcceso.String(), Pruebas: d.Ambiente == sri.AmbientePruebas,
		Emision: d.Fecha, Mesa: caja, Cajero: cajero, Comprador: d.Comprador.RazonSocial, CompradorID: d.Comprador.Identificacion,
		SubtotalSinImpuestos: nc.TotalSinImpuestos, Total: nc.ValorModificacion,
		NotaCredito: true, Sustento: d.Sustento.Numero(), FechaSustento: d.Sustento.FechaEmision, Motivo: d.Motivo,
	}
	for _, l := range nc.Lineas {
		r.TotalDescuento = r.TotalDescuento.Add(l.Descuento)
		r.Lineas = append(r.Lineas, escpos.LineaRide{Cantidad: l.Cantidad.String(), Descripcion: l.Descripcion,
			PrecioUnitario: precioRide(l.PrecioUnitario), Descuento: l.Descuento, Total: l.PrecioTotalSinImpuesto})
	}
	for _, t := range nc.Impuestos {
		pct := ""
		for _, l := range nc.Lineas {
			if l.CodigoPorcentaje == t.CodigoPorcentaje {
				if v, err := decimal.NewFromString(l.Tarifa); err == nil {
					pct = v.String() // 15.00 → 15, 0.00 → 0
				}
			}
		}
		if !t.Base.IsZero() {
			r.Subtotales = append(r.Subtotales, escpos.ValorRide{Etiqueta: "SUBTOTAL " + pct + "%", Valor: t.Base})
		}
		if !t.Valor.IsZero() {
			r.IVAs = append(r.IVAs, escpos.ValorRide{Etiqueta: "IVA " + pct + "%", Valor: t.Valor})
		}
	}
	for _, a := range d.Adicionales {
		if strings.TrimSpace(a.Valor) != "" {
			r.Adicionales = append(r.Adicionales, escpos.ValorTexto{Nombre: a.Nombre, Valor: a.Valor})
		}
	}
	return r
}
