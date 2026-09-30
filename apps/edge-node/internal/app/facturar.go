package app

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/shopspring/decimal"

	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/edge-node/internal/store"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/escpos"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/ids"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/money"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/sri"
)

// EventoComprobanteEmitido lleva a la nube el comprobante para firmarlo y enviarlo al SRI.
const EventoComprobanteEmitido = "comprobante.emitido"

// configFiscal es la configuración del emisor replicada desde la nube (F5-02).
type configFiscal struct {
	Ambiente              int
	RUC                   string
	RazonSocial           string
	NombreComercial       string
	DirMatriz             string
	ObligadoContabilidad  bool
	ContribuyenteEspecial string
	AgenteRetencion       string
	Regimen               string
}

// leerConfigFiscal devuelve la configuración si la facturación electrónica está activa.
func leerConfigFiscal(ctx context.Context, q queryer) (*configFiscal, error) {
	var c configFiscal
	var nombre, especial, agente sql.NullString
	var obligado, activa int
	err := q.QueryRowContext(ctx, `SELECT ambiente, ruc, razon_social, nombre_comercial, direccion_matriz, obligado_contabilidad,
		contribuyente_especial, agente_retencion, regimen, facturacion_activa FROM configuracion_fiscal LIMIT 1`).
		Scan(&c.Ambiente, &c.RUC, &c.RazonSocial, &nombre, &c.DirMatriz, &obligado, &especial, &agente, &c.Regimen, &activa)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && activa == 0) {
		return nil, nil //nolint:nilnil // sin facturación: el cobro emite el documento interno
	}
	if err != nil {
		return nil, err
	}
	c.NombreComercial, c.ContribuyenteEspecial, c.AgenteRetencion = nombre.String, especial.String, agente.String
	c.ObligadoContabilidad = obligado == 1
	return &c, nil
}

// facturaEmitida es lo que el cobro necesita para el documento, el ticket y el evento.
type facturaEmitida struct {
	ID          ids.ID
	Numero      string // 001-002-000000067
	Clave       sri.ClaveAcceso
	Ambiente    int
	Secuencial  int64
	Punto       PuntoEmision
	Desglose    sri.Factura
	Datos       sri.DatosFactura
	XML         []byte
	Hash        string
	DirLocal    string
	FechaLocal  time.Time
	RUCProveedo string
}

type emisionIn struct {
	CajaID    ids.ID
	Orden     Orden
	Totales   Totales // de la orden completa (reparto, tarifas, servicio)
	Cuenta    *CuentaTotales
	Cuentas   []cuentaReparto
	Total     money.Money
	Propina   money.Money
	Comprador CompradorIn
	Pagos     []PagoDoc
	Ahora     time.Time
	Zona      *time.Location
}

// prepararFactura arma el comprobante dentro de la transacción del cobro (F5-05): secuencial
// del punto de la caja (F5-02), clave de acceso, desglose fiscal (F5-04) y XML 1.1.0 sin
// firmar (F5-08). guardarComprobante lo registra después del documento de venta. Cualquier
// error deshace el cobro: nunca sale una venta con un comprobante a medias ni se pierde un número.
func prepararFactura(ctx context.Context, tx *store.Tx, cfg *configFiscal, in emisionIn) (facturaEmitida, error) {
	var f facturaEmitida
	punto, err := puntoDeCaja(ctx, tx, in.CajaID)
	if err != nil {
		return f, err
	}
	venta, err := ventaFiscal(ctx, tx, in)
	if err != nil {
		return f, err
	}
	desglose, err := sri.Desglosar(venta)
	if err != nil {
		return f, fmt.Errorf("desglose fiscal: %w", err)
	}
	sec, err := siguienteSecuencial(ctx, tx, punto.ID, sri.TipoFactura, cfg.Ambiente)
	if err != nil {
		return f, err
	}
	fecha := in.Ahora.In(in.Zona)
	clave, err := sri.NuevaClaveAcceso(sri.ClaveAccesoInput{FechaEmision: fecha, TipoComprobante: sri.TipoFactura, RUC: cfg.RUC,
		Ambiente: sri.Ambiente(cfg.Ambiente), Establecimiento: punto.Establecimiento, PuntoEmision: punto.Punto, Secuencial: sec})
	if err != nil {
		return f, err
	}
	var dirLocal string
	_ = tx.QueryRowContext(ctx, `SELECT direccion FROM locales LIMIT 1`).Scan(&dirLocal)
	var rucProveedor string
	_ = tx.QueryRowContext(ctx, `SELECT valor FROM parametros_globales WHERE clave = 'ruc_proveedor_sistema'`).Scan(&rucProveedor)
	datos := sri.DatosFactura{
		Ambiente: sri.Ambiente(cfg.Ambiente), ClaveAcceso: clave, Secuencial: sec, Fecha: fecha,
		Emisor: sri.Emisor{RUC: cfg.RUC, RazonSocial: cfg.RazonSocial, NombreComercial: cfg.NombreComercial, DirMatriz: cfg.DirMatriz,
			DirEstablecimiento: dirLocal, ObligadoContabilidad: cfg.ObligadoContabilidad, ContribuyenteEspecial: cfg.ContribuyenteEspecial,
			AgenteRetencion: cfg.AgenteRetencion, RIMPE: cfg.Regimen == "RIMPE_EMPRENDEDOR", Establecimiento: punto.Establecimiento, PuntoEmision: punto.Punto},
		Comprador: sri.Comprador{TipoIdentificacion: in.Comprador.TipoIdentificacion, Identificacion: in.Comprador.Identificacion,
			RazonSocial: in.Comprador.RazonSocial, Direccion: in.Comprador.Direccion},
	}
	for _, p := range in.Pagos {
		m, err := money.Parse(p.Monto)
		if err != nil {
			return f, err
		}
		datos.Pagos = append(datos.Pagos, sri.Pago{FormaPago: p.CodigoSRI, Total: m})
	}
	datos.Adicionales = []sri.CampoAdicional{{Nombre: "Email", Valor: in.Comprador.Email}, {Nombre: "Teléfono", Valor: in.Comprador.Telefono},
		{Nombre: "Mesa", Valor: in.Orden.Mesa}}
	// RUC del proveedor del sistema (Anexo 26 de la ficha, obligatorio con sistemas de terceros).
	if strings.TrimSpace(rucProveedor) != "" {
		datos.Adicionales = append(datos.Adicionales, sri.CampoAdicional{Nombre: "RUC Proveedor", Valor: strings.TrimSpace(rucProveedor)})
	}
	doc, err := sri.FacturaXML(datos, desglose)
	if err != nil {
		return f, err
	}
	suma := sha256.Sum256(doc)
	return facturaEmitida{ID: ids.New(), Numero: fmt.Sprintf("%s-%s-%09d", punto.Establecimiento, punto.Punto, sec), Clave: clave, Ambiente: cfg.Ambiente,
		Secuencial: sec, Punto: punto, Desglose: desglose, Datos: datos, XML: doc, Hash: hex.EncodeToString(suma[:]), DirLocal: dirLocal,
		FechaLocal: fecha, RUCProveedo: rucProveedor}, nil
}

// guardarComprobante registra la factura preparada, ya insertado su documento de venta.
func guardarComprobante(ctx context.Context, tx *store.Tx, f facturaEmitida, documento ids.ID, ahora time.Time) error {
	_, err := tx.ExecContext(ctx, `INSERT INTO comprobantes (id, documento_id, tipo, ambiente, punto_emision_id, serie, secuencial, clave_acceso,
		fecha_emision, importe_total, xml, hash, created_at) VALUES (?, ?, '01', ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		f.ID.String(), documento.String(), f.Ambiente, f.Punto.ID.String(), f.Punto.Serie(), f.Secuencial, f.Clave.String(),
		f.FechaLocal.Format(time.DateOnly), f.Desglose.ImporteTotal.String(), string(f.XML), f.Hash, ahora.Format(time.RFC3339Nano))
	return err
}

// ventaFiscal arma la venta para el desglose: la orden completa o, si se dividió, la cuenta
// que se cobra (cada plato con lo que esa cuenta lleva de él).
func ventaFiscal(ctx context.Context, tx *store.Tx, in emisionIn) (sri.Venta, error) {
	v := sri.Venta{PorTarifa: map[string]sri.TotalTarifa{}, Propina: in.Propina, IncluyeIVA: in.Totales.incluyeIVA, Total: in.Total}
	codigos := map[string]string{}
	rows, err := tx.QueryContext(ctx, `SELECT porcentaje, codigo_sri FROM tarifas_iva`)
	if err != nil {
		return v, err
	}
	for rows.Next() {
		var p, c string
		if err := rows.Scan(&p, &c); err != nil {
			_ = rows.Close()
			return v, err
		}
		if d, err := decimal.NewFromString(p); err == nil {
			codigos[d.String()] = c
		}
	}
	if err := rows.Close(); err != nil {
		return v, err
	}
	tarifa := func(pct string) (sri.Tarifa, error) {
		d, err := decimal.NewFromString(pct)
		if err != nil {
			return sri.Tarifa{}, err
		}
		c, ok := codigos[d.String()]
		if !ok {
			return sri.Tarifa{}, fmt.Errorf("la tarifa de IVA del %s %% no tiene código del SRI", pct)
		}
		return sri.Tarifa{Porcentaje: d.String(), Codigo: c}, nil
	}
	porTarifa := in.Totales.PorTarifa
	if in.Cuenta != nil {
		porTarifa = in.Cuenta.PorTarifa
	}
	for pct, t := range porTarifa {
		tf, err := tarifa(pct)
		if err != nil {
			return v, err
		}
		v.PorTarifa[tf.Porcentaje] = sri.TotalTarifa{Base: t.Base, IVA: t.IVA}
	}
	lineas := map[ids.ID]LineaOrden{}
	for _, l := range in.Orden.Lineas {
		lineas[l.ID] = l
	}
	fracciones, err := fraccionesDeCuenta(ctx, tx, in.Cuenta)
	if err != nil {
		return v, err
	}
	reparto := append([]lineaReparto(nil), in.Totales.reparto...)
	sort.SliceStable(reparto, func(i, j int) bool { return lineas[reparto[i].ID].CreadaAt.Before(lineas[reparto[j].ID].CreadaAt) })
	for _, r := range reparto {
		l := lineas[r.ID]
		tf, err := tarifa(r.Pct)
		if err != nil {
			return v, err
		}
		lv := sri.LineaVenta{Codigo: codigoProducto(l.ProductoID), Descripcion: descripcionLinea(l), Tarifa: tf}
		if in.Cuenta == nil {
			bruto, err := money.Parse(l.Total)
			if err != nil {
				return v, err
			}
			cant, err := decimal.NewFromString(l.Cantidad)
			if err != nil {
				return v, err
			}
			lv.Cantidad, lv.Bruto, lv.Final = cant, bruto, r.Final
		} else {
			parte, ok := in.Cuenta.Lineas[r.ID]
			if !ok {
				continue // el plato no es de esta cuenta
			}
			// Un plato compartido va como «1/3 Pizza» por el valor que lleva esta cuenta.
			lv.Cantidad, lv.Bruto, lv.Final = decimal.NewFromInt(1), parte, parte
			if fr := fracciones[r.ID]; fr != "" {
				lv.Descripcion = fr + " " + lv.Descripcion
			}
		}
		v.Lineas = append(v.Lineas, lv)
	}
	return v, nil
}

// fraccionesDeCuenta: la parte de cada plato compartido que lleva la cuenta («1/3»), contando
// los pesos de todas las cuentas de la orden, también las ya pagadas.
func fraccionesDeCuenta(ctx context.Context, q queryer, cuenta *CuentaTotales) (map[ids.ID]string, error) {
	out := map[ids.ID]string{}
	if cuenta == nil {
		return out, nil
	}
	rows, err := q.QueryContext(ctx, `SELECT a.orden_linea_id, a.peso, (SELECT sum(b.peso) FROM cuenta_asignaciones b JOIN cuentas c ON c.id = b.cuenta_id
			WHERE c.orden_id = (SELECT orden_id FROM cuentas WHERE id = a.cuenta_id) AND b.orden_linea_id = a.orden_linea_id)
		FROM cuenta_asignaciones a WHERE a.cuenta_id = ?`, cuenta.ID.String())
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var linea string
		var peso, suma int64
		if err := rows.Scan(&linea, &peso, &suma); err != nil {
			return nil, err
		}
		if suma > peso {
			l, _ := ids.Parse(linea)
			out[l] = fmt.Sprintf("%d/%d", peso, suma)
		}
	}
	return out, rows.Err()
}

func descripcionLinea(l LineaOrden) string {
	s := l.Producto
	if len(l.Modificadores) > 0 {
		nombres := make([]string, len(l.Modificadores))
		for i, m := range l.Modificadores {
			nombres[i] = m.Nombre
		}
		s += " (" + strings.Join(nombres, ", ") + ")"
	}
	return s
}

// codigoProducto: los primeros 8 caracteres del id del producto (el XSD admite hasta 25).
func codigoProducto(id ids.ID) string {
	s := strings.ReplaceAll(id.String(), "-", "")
	if len(s) > 8 {
		s = s[len(s)-8:]
	}
	return strings.ToUpper(s)
}

// precioRide: dos decimales, o los que tenga hasta seis (13.04, 2.608696).
func precioRide(d decimal.Decimal) string {
	if d.Exponent() >= -2 {
		return d.StringFixed(2)
	}
	return d.String()
}

// rideDe arma el ticket RIDE (Anexo 2) de la factura emitida.
func rideDe(cfg *configFiscal, f facturaEmitida, mesa, cajero string, comprador CompradorIn, pagos []PagoDoc, recibido, vuelto money.Money) escpos.Ride {
	r := escpos.Ride{
		NombreComercial: cfg.NombreComercial, RazonSocial: cfg.RazonSocial, RUC: cfg.RUC, DirMatriz: cfg.DirMatriz, DirEstablecimiento: f.DirLocal,
		ContribuyenteEspecial: cfg.ContribuyenteEspecial, ObligadoContabilidad: cfg.ObligadoContabilidad, AgenteRetencion: cfg.AgenteRetencion,
		RIMPE: cfg.Regimen == "RIMPE_EMPRENDEDOR", Numero: f.Numero, ClaveAcceso: f.Clave.String(), Pruebas: f.Ambiente == 1,
		Emision: f.FechaLocal, Mesa: mesa, Cajero: cajero, Comprador: comprador.RazonSocial, CompradorDireccion: comprador.Direccion,
		SubtotalSinImpuestos: f.Desglose.TotalSinImpuestos, TotalDescuento: f.Desglose.TotalDescuento, Propina: f.Desglose.Propina,
		Total: f.Desglose.ImporteTotal, Recibido: recibido, Vuelto: vuelto,
	}
	if comprador.TipoIdentificacion != string(sri.IdConsumidorFinal) {
		r.CompradorID = comprador.Identificacion
	}
	for _, l := range f.Desglose.Lineas {
		r.Lineas = append(r.Lineas, escpos.LineaRide{Cantidad: l.Cantidad.String(), Descripcion: l.Descripcion,
			PrecioUnitario: precioRide(l.PrecioUnitario), Descuento: l.Descuento, Total: l.PrecioTotalSinImpuesto})
	}
	for _, imp := range f.Desglose.Impuestos {
		if imp.BaseImponible.IsZero() {
			continue // el RIDE muestra solo los subtotales con valor (Anexo 2)
		}
		r.Subtotales = append(r.Subtotales, escpos.ValorRide{Etiqueta: "SUBTOTAL " + imp.Tarifa.Porcentaje + "%", Valor: imp.BaseImponible})
		if !imp.Valor.IsZero() {
			r.IVAs = append(r.IVAs, escpos.ValorRide{Etiqueta: "IVA " + imp.Tarifa.Porcentaje + "%", Valor: imp.Valor})
		}
	}
	for _, p := range pagos {
		m, _ := money.Parse(p.Monto)
		r.Pagos = append(r.Pagos, escpos.PagoRide{Codigo: p.CodigoSRI, Metodo: p.Metodo, Monto: m, Ultimos4: p.Ultimos4})
	}
	for _, a := range f.Datos.Adicionales {
		if strings.TrimSpace(a.Valor) != "" && a.Nombre != "Mesa" {
			r.Adicionales = append(r.Adicionales, escpos.ValorTexto{Nombre: a.Nombre, Valor: a.Valor})
		}
	}
	return r
}
