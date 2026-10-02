package app

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/edge-node/internal/impresion"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/edge-node/internal/store"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/escpos"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/eventos"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/ids"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/money"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/rbac"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/sri"
)

// EventoVentaCobrada lleva a la nube el documento y el pago de una venta (dinero: en la
// misma transacción del cobro).
const EventoVentaCobrada = "venta.cobrada"

// Nombre del consumidor final en el documento (su identificación es sri.ConsumidorFinal).
const consumidorFinalNombre = "CONSUMIDOR FINAL"

// PagoIn es una parte del cobro con un método (RF-04-04). Monto es lo que se aplica a la
// cuenta; en efectivo, Recibido es lo que entregó el cliente (el vuelto sale de ahí).
type PagoIn struct {
	MetodoID   ids.ID `json:"metodoId"`
	Monto      string `json:"monto"`    // vacío en un cobro de un solo método = el total
	Recibido   string `json:"recibido"` // solo efectivo; vacío = exacto
	Referencia string `json:"referencia"`
	Lote       string `json:"lote"`
	Ultimos4   string `json:"ultimos4"`
}

type CobrarIn struct {
	CajaID          ids.ID   `json:"cajaId"`
	Pagos           []PagoIn `json:"pagos"` // pago mixto: varios métodos que suman el total
	ConsumidorFinal bool     `json:"consumidorFinal"`
	// Comprador identificado (F4-07). Nulo con ConsumidorFinal = venta a consumidor final.
	Comprador *CompradorIn `json:"comprador"`
	// Cuenta que se cobra cuando la orden está dividida (F4-08).
	CuentaID       *ids.ID `json:"cuentaId"`
	IdempotencyKey string  `json:"idempotencyKey"`
	// Cobro de un solo método (el toque en un billete): equivale a Pagos con un elemento.
	MetodoID   ids.ID `json:"metodoId"`
	Recibido   string `json:"recibido"`
	Referencia string `json:"referencia"`
	Lote       string `json:"lote"`
	Ultimos4   string `json:"ultimos4"`
}

// PagoDoc es un pago tal como quedó en el documento.
type PagoDoc struct {
	MetodoID   ids.ID `json:"metodoId"`
	Metodo     string `json:"metodo"`
	Tipo       string `json:"tipo"`
	CodigoSRI  string `json:"codigoSri"`
	Monto      string `json:"monto"`
	Recibido   string `json:"recibido,omitempty"`
	Vuelto     string `json:"vuelto,omitempty"`
	Referencia string `json:"referencia,omitempty"`
	Lote       string `json:"lote,omitempty"`
	Ultimos4   string `json:"ultimos4,omitempty"`
}

// DocumentoVenta es lo que devuelve el cobro: el documento emitido y el vuelto.
type DocumentoVenta struct {
	ID     ids.ID `json:"id"`
	Tipo   string `json:"tipo"` // INTERNO o FACTURA (F5-05, con la facturación electrónica activa)
	Numero int    `json:"numero"`
	Codigo string `json:"codigo"` // INT-000123 o, en una factura, 001-002-000000067
	// Factura (F5-05): clave de acceso (= número de autorización) y ambiente (1 pruebas).
	ClaveAcceso string  `json:"claveAcceso,omitempty"`
	Ambiente    int     `json:"ambiente,omitempty"`
	OrdenID     ids.ID  `json:"ordenId"`
	CuentaID    *ids.ID `json:"cuentaId,omitempty"`
	Cuenta      int     `json:"cuenta,omitempty"` // número de la cuenta si la orden se dividió
	Mesa        string  `json:"mesa"`
	Comprador   string  `json:"comprador"` // nombre o razón social
	// Identificación del comprador con su código SRI (07 = consumidor final).
	CompradorTipo           string    `json:"compradorTipo"`
	CompradorIdentificacion string    `json:"compradorIdentificacion"`
	CompradorEmail          string    `json:"compradorEmail,omitempty"`
	CompradorDireccion      string    `json:"compradorDireccion,omitempty"`
	CompradorTelefono       string    `json:"compradorTelefono,omitempty"`
	ClienteGuardado         bool      `json:"clienteGuardado"` // se guardó con consentimiento
	Totales                 Totales   `json:"totales"`
	Metodo                  string    `json:"metodo"` // «Efectivo» o «Efectivo + Tarjeta crédito»
	Pagos                   []PagoDoc `json:"pagos"`
	Recibido                string    `json:"recibido"` // efectivo entregado (0 si no hubo efectivo)
	Vuelto                  string    `json:"vuelto"`
	AbreCajon               bool      `json:"abreCajon"`
	Cajero                  string    `json:"cajero"`
	TurnoID                 ids.ID    `json:"turnoId"`
	EmitidoAt               time.Time `json:"emitidoAt"`
	// FechaNegocio de la jornada de la orden (AAAA-MM-DD): los reportes van por jornada (F5-16).
	FechaNegocio string `json:"fechaNegocio,omitempty"`
}

type CobroOut struct {
	Documento  DocumentoVenta `json:"documento"`
	Cerrada    bool           `json:"cerrada"` // la orden quedó cerrada (con división: al cobrar la última cuenta)
	Impresoras []string       `json:"impresoras"`
	Aviso      string         `json:"aviso,omitempty"`
}

var ultimos4Re = regexp.MustCompile(`^\d{4}$`)

func recortar(s string, max int) string {
	s = strings.TrimSpace(s)
	if r := []rune(s); len(r) > max {
		return string(r[:max])
	}
	return s
}

// Cobrar cobra la orden completa con un método en un solo gesto (RF-04-03.3): registra el
// pago en el turno, emite el documento, cierra la orden (la mesa queda libre), imprime con el
// pulso del cajón si el método lo abre y deja el evento para la nube. Todo o nada.
func (a *App) Cobrar(ctx context.Context, u Usuario, orden ids.ID, in CobrarIn) (CobroOut, error) {
	var out CobroOut
	if !u.Puede(rbac.Cobrar) {
		return out, problema(http.StatusForbidden, "SIN_PERMISO", "No tienes permiso para cobrar.")
	}
	in.IdempotencyKey = strings.TrimSpace(in.IdempotencyKey)
	if len(in.IdempotencyKey) < 8 || len(in.IdempotencyKey) > 64 {
		return out, invalido("Falta la clave de idempotencia del cobro.")
	}
	// Comprador: identificado o consumidor final (el mismo número 9999999999999 también lo es).
	comprador := CompradorIn{TipoIdentificacion: string(sri.IdConsumidorFinal), Identificacion: sri.ConsumidorFinal, RazonSocial: consumidorFinalNombre}
	if in.Comprador != nil {
		if _, err := in.Comprador.normalizar(); err != nil {
			return out, err
		}
		if in.Comprador.TipoIdentificacion != string(sri.IdConsumidorFinal) {
			comprador = *in.Comprador
		}
	} else if !in.ConsumidorFinal {
		return out, invalido("Indica los datos del comprador o cobra a consumidor final.")
	}
	esCF := comprador.TipoIdentificacion == string(sri.IdConsumidorFinal)
	unico := len(in.Pagos) == 0
	if unico {
		in.Pagos = []PagoIn{{MetodoID: in.MetodoID, Recibido: in.Recibido, Referencia: in.Referencia, Lote: in.Lote, Ultimos4: in.Ultimos4}}
	}
	if len(in.Pagos) > 10 {
		return out, invalido("Un cobro admite hasta 10 pagos.")
	}
	pagos := make([]pagoValido, len(in.Pagos))
	for i, p := range in.Pagos {
		v := pagoValido{PagoIn: p}
		v.Referencia, v.Lote, v.Ultimos4 = recortar(p.Referencia, 40), recortar(p.Lote, 20), strings.TrimSpace(p.Ultimos4)
		if v.Ultimos4 != "" && !ultimos4Re.MatchString(v.Ultimos4) {
			return out, invalido("Los últimos 4 dígitos de la tarjeta son 4 números.")
		}
		if strings.TrimSpace(p.Recibido) != "" {
			m, err := montoNoNegativo(p.Recibido, "El monto recibido")
			if err != nil {
				return out, err
			}
			v.recibido = m
		}
		if !unico {
			m, err := montoNoNegativo(p.Monto, "El monto de cada pago")
			if err != nil {
				return out, err
			}
			if m.IsZero() {
				return out, invalido("Cada pago debe ser mayor que cero.")
			}
			v.monto = m
		}
		pagos[i] = v
	}
	now, loc := a.Clock.Now(), a.zonaLocal(ctx)
	var despertar []ids.ID
	var mesa *ids.ID
	err := a.Store.Write(ctx, func(tx *store.Tx) error {
		// Reintento o doble toque con la misma clave: el mismo documento, sin cobrar dos veces.
		var datos string
		err := tx.QueryRowContext(ctx, `SELECT datos FROM documentos_venta WHERE idempotency_key = ?`, in.IdempotencyKey).Scan(&datos)
		if err == nil {
			return json.Unmarshal([]byte(datos), &out.Documento)
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		t, err := turnoAbiertoDe(ctx, tx, in.CajaID)
		if err != nil {
			return err
		}
		o, err := leerOrden(ctx, tx, orden)
		if errors.Is(err, errOrdenNoExiste) || (err == nil && o.Estado != "ABIERTA" && o.Estado != "PRECUENTA") {
			return problema(http.StatusConflict, "ORDEN_CERRADA", "Esta orden ya se cobró o se cerró.")
		}
		if err != nil {
			return err
		}
		if o.MesaID != nil {
			b, err := leerBloqueo(ctx, tx, *o.MesaID, now)
			if err != nil {
				return err
			}
			if b != nil {
				return errOcupada(b)
			}
		}
		tot, base, iva, propina, total, err := a.calcularTotales(ctx, tx, o)
		if err != nil {
			return err
		}
		totOrden := tot // la orden completa (reparto por plato y tarifas), aun si se cobra una cuenta
		// Orden dividida (F4-08): se cobra una cuenta, con sus propios totales.
		_, cuentas, res, _, errCuentas := a.cuentasDe(ctx, tx, o)
		var cuenta *CuentaTotales
		switch {
		case len(cuentas) == 0 && in.CuentaID != nil:
			return invalido("Esta orden no está dividida.")
		case len(cuentas) > 0:
			if errCuentas != nil {
				return errDivision(errCuentas, o)
			}
			if in.CuentaID == nil {
				return problema(http.StatusConflict, "ORDEN_DIVIDIDA", "La orden está dividida: elige qué cuenta cobras.")
			}
			for i := range res {
				if res[i].ID == *in.CuentaID {
					cuenta = &res[i]
				}
			}
			if cuenta == nil {
				return problema(http.StatusNotFound, "NO_ENCONTRADO", "Esa cuenta no es de esta orden.")
			}
			if cuenta.Pagada {
				return problema(http.StatusConflict, "CUENTA_PAGADA", "Esa cuenta ya se cobró.")
			}
			base, iva, propina, total = cuenta.Subtotal, cuenta.IVA, cuenta.Propina, cuenta.Total
			lineas := map[ids.ID]string{}
			for l, m := range cuenta.Lineas {
				lineas[l] = m.String()
			}
			tot = Totales{Subtotal: base.String(), IVA: iva.String(), Propina: propina.String(), Total: total.String(), PropinaActiva: tot.PropinaActiva,
				PropinaPorcentaje: tot.PropinaPorcentaje, PropinaRetirada: tot.PropinaRetirada, Descuento: "0.00", Descuentos: []Descuento{}, Lineas: lineas, PorTarifa: cuenta.PorTarifa}
		}
		// Total en cero: si todo es cortesía la orden se cierra con un documento en $0 (sin pagos
		// ni cajón); sin platos o sin descuentos no hay nada que cobrar.
		todoCortesia := total.IsZero() && (tot.Descuento != "0.00" || cuenta != nil) && len(o.Lineas) > 0
		if !total.GreaterThan(money.Money{}) && !todoCortesia {
			return problema(http.StatusConflict, "ORDEN_VACIA", "La orden no tiene nada que cobrar.")
		}
		if max := consumidorFinalMaximo(ctx, tx); esCF && total.GreaterThan(max) {
			return problema(http.StatusUnprocessableEntity, "CONSUMIDOR_FINAL_EXCEDIDO",
				"El total supera $"+max.String()+", el máximo para consumidor final. Hacen falta los datos del comprador.")
		}
		var docPagos []PagoDoc
		var recibido, vuelto money.Money
		var abreCajon bool
		metodo := "Cortesía"
		if !todoCortesia {
			if docPagos, recibido, vuelto, abreCajon, err = resolverPagos(ctx, tx, pagos, total, unico); err != nil {
				return err
			}
			nombres := make([]string, len(docPagos))
			for i, p := range docPagos {
				nombres[i] = p.Metodo
			}
			metodo = strings.Join(nombres, " + ")
		}
		if docPagos == nil {
			docPagos = []PagoDoc{}
		}
		// Con la facturación electrónica activa el documento es la factura (F5-05); una cuenta
		// invitada completa ($0) sigue siendo un documento interno.
		tipoDoc := "INTERNO"
		var cfg *configFiscal
		if !todoCortesia {
			if cfg, err = leerConfigFiscal(ctx, tx); err != nil {
				return err
			}
			if cfg != nil {
				tipoDoc = "FACTURA"
			}
		}
		var numero int
		if err := tx.QueryRowContext(ctx, `INSERT INTO contadores (clave, valor) VALUES ('documento:' || ?, 1)
			ON CONFLICT (clave) DO UPDATE SET valor = valor + 1 RETURNING valor`, tipoDoc).Scan(&numero); err != nil {
			return err
		}
		doc := DocumentoVenta{ID: ids.New(), Tipo: tipoDoc, Numero: numero, Codigo: escpos.NumeroInterno(numero), OrdenID: o.ID, CuentaID: in.CuentaID, Mesa: o.Mesa,
			Comprador: comprador.RazonSocial, CompradorTipo: comprador.TipoIdentificacion, CompradorIdentificacion: comprador.Identificacion,
			CompradorEmail: comprador.Email, CompradorDireccion: comprador.Direccion, CompradorTelefono: comprador.Telefono, Totales: tot, Metodo: metodo, Pagos: docPagos, Recibido: recibido.String(), Vuelto: vuelto.String(),
			AbreCajon: abreCajon, Cajero: u.Nombre, TurnoID: t.ID, EmitidoAt: now}
		_ = tx.QueryRowContext(ctx, `SELECT fecha_negocio FROM ordenes WHERE id = ?`, o.ID.String()).Scan(&doc.FechaNegocio)
		// Con consentimiento, el comprador queda guardado para la próxima compra (LOPDP).
		if !esCF && comprador.Consentimiento {
			if _, err := a.guardarClienteEnTx(ctx, tx, comprador, now); err != nil {
				return err
			}
			doc.ClienteGuardado = true
		}
		if cuenta != nil {
			doc.Cuenta = cuenta.Numero
		}
		var fact *facturaEmitida
		if cfg != nil {
			f, err := prepararFactura(ctx, tx, cfg, emisionIn{CajaID: in.CajaID, Orden: o, Totales: totOrden, Cuenta: cuenta, Cuentas: cuentas,
				Total: total, Propina: propina, Comprador: comprador, Pagos: docPagos, Ahora: now, Zona: loc})
			if err != nil {
				return err
			}
			fact = &f
			doc.Codigo, doc.ClaveAcceso, doc.Ambiente = f.Numero, f.Clave.String(), f.Ambiente
		}
		raw, err := json.Marshal(doc)
		if err != nil {
			return err
		}
		ts := now.Format(time.RFC3339Nano)
		if _, err := tx.ExecContext(ctx, `INSERT INTO documentos_venta (id, tipo, numero, orden_id, turno_id, comprador_tipo, comprador_identificacion, comprador_nombre,
			subtotal, iva, propina, total, datos, idempotency_key, emitido_por, created_at, cuenta_id) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			doc.ID.String(), doc.Tipo, numero, o.ID.String(), t.ID.String(), comprador.TipoIdentificacion, comprador.Identificacion, comprador.RazonSocial,
			base.String(), iva.String(), propina.String(), total.String(), string(raw), in.IdempotencyKey, u.ID.String(), ts, idOrNil(in.CuentaID)); err != nil {
			return err
		}
		if fact != nil {
			if err := guardarComprobante(ctx, tx, *fact, doc.ID, now); err != nil {
				return err
			}
		}
		for _, p := range docPagos {
			if _, err := tx.ExecContext(ctx, `INSERT INTO pagos (id, orden_id, cuenta_id, turno_id, metodo_pago_id, monto, recibido, vuelto, referencia, lote, ultimos4, documento_id, created_at)
				VALUES (?, ?, ?, ?, ?, ?, nullif(?, ''), nullif(?, ''), nullif(?, ''), nullif(?, ''), nullif(?, ''), ?, ?)`,
				ids.New().String(), o.ID.String(), idOrNil(in.CuentaID), t.ID.String(), p.MetodoID.String(), p.Monto, p.Recibido, p.Vuelto, p.Referencia, p.Lote, p.Ultimos4, doc.ID.String(), ts); err != nil {
				return err
			}
		}
		// La propina queda registrada por orden y mesero para el reparto (RF-04-08.4, F8-03).
		if propina.GreaterThan(money.Money{}) {
			var fecha string
			if err := tx.QueryRowContext(ctx, `SELECT fecha_negocio FROM ordenes WHERE id = ?`, o.ID.String()).Scan(&fecha); err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, `INSERT INTO propinas (id, orden_id, documento_id, mesero_id, mesero_nombre, fecha_negocio, monto, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
				ids.New().String(), o.ID.String(), doc.ID.String(), o.MeseroID.String(), o.MeseroNombre, fecha, propina.String(), ts); err != nil {
				return err
			}
		}
		// Una cuenta cobrada queda congelada con lo que pagó; la orden se cierra con la última.
		cerrar := true
		if cuenta != nil {
			pagado, err := json.Marshal(cuenta)
			if err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, `UPDATE cuentas SET estado = 'PAGADA', documento_id = ?, pagado = ?, pagada_at = ? WHERE id = ?`,
				doc.ID.String(), string(pagado), ts, cuenta.ID.String()); err != nil {
				return err
			}
			var abiertas int
			if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM cuentas WHERE orden_id = ? AND estado = 'ABIERTA'`, o.ID.String()).Scan(&abiertas); err != nil {
				return err
			}
			cerrar = abiertas == 0
		}
		if cerrar {
			// La orden se cierra y la mesa queda libre; se suelta cualquier bloqueo vencido.
			if _, err := tx.ExecContext(ctx, `UPDATE ordenes SET estado = 'CERRADA', cerrada_at = ?, version = version + 1 WHERE id = ?`, ts, o.ID.String()); err != nil {
				return err
			}
		}
		if o.MesaID != nil && cerrar {
			if _, err := tx.ExecContext(ctx, `DELETE FROM bloqueos_mesa WHERE mesa_id = ?`, o.MesaID.String()); err != nil {
				return err
			}
			mesa = o.MesaID
		}
		// Impresión en la estación de la caja, con el pulso del cajón delante.
		var estacion sql.NullString
		if err := tx.QueryRowContext(ctx, `SELECT estacion_id FROM cajas WHERE id = ?`, in.CajaID.String()).Scan(&estacion); err != nil {
			return err
		}
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
			out.Aviso = "La caja no tiene impresora: el documento salió en «" + respaldo + "»."
		case len(imps) == 0:
			out.Aviso = "No hay impresoras configuradas: la venta quedó registrada, pero no se imprimió."
		}
		var local string
		_ = tx.QueryRowContext(ctx, `SELECT nombre FROM locales LIMIT 1`).Scan(&local)
		fracciones, err := fraccionesDeCuenta(ctx, tx, cuenta)
		if err != nil {
			return err
		}
		ticket := escpos.DocumentoVenta{Local: local, Numero: numero, Hora: now.In(loc), Mesa: o.Mesa, Cajero: u.Nombre, Comprador: comprador.RazonSocial,
			CompradorID: map[bool]string{true: "", false: comprador.Identificacion}[esCF],
			Lineas:      lineasTicket(o, cuenta, fracciones), Descuento: money.MustParse(tot.Descuento), Subtotal: base, IVA: iva, Propina: propina, Total: total, Recibido: recibido, Vuelto: vuelto}
		for _, p := range docPagos {
			ticket.Pagos = append(ticket.Pagos, escpos.PagoTicket{Metodo: p.Metodo, Monto: money.MustParse(p.Monto), Ultimos4: p.Ultimos4})
		}
		out.Impresoras = []string{}
		oid := o.ID.String()
		var ride escpos.Ride
		if fact != nil {
			ride = rideDe(&fact.Config, *fact, o.Mesa, u.Nombre, comprador, docPagos, recibido, vuelto)
		}
		for i, imp := range imps {
			ticket.AbrirCajon = abreCajon && i == 0 // un solo pulso: el cajón cuelga de la primera
			contenido := escpos.ImprimirDocumentoVenta(imp.Ancho, ticket)
			if fact != nil { // la factura sale como RIDE (F5-05)
				ride.AbrirCajon = ticket.AbrirCajon
				contenido = escpos.ImprimirRide(imp.Ancho, ride)
			}
			tr := impresion.Trabajo{ID: ids.New(), ImpresoraID: imp.ID, Tipo: "VENTA", ComandaID: &oid}
			if err := impresion.Encolar(ctx, tx, tr, nil, contenido, nil, now); err != nil {
				return err
			}
			out.Impresoras = append(out.Impresoras, imp.Nombre)
			despertar = append(despertar, imp.ID)
		}
		uid := u.ID
		if err := auditar(ctx, tx, "COBRO", "orden", o.ID, &uid, map[string]any{"documento": doc.Codigo, "total": total.String(), "metodo": metodo, "turno": t.ID}, now); err != nil {
			return err
		}
		out.Documento = doc
		out.Cerrada = cerrar
		if err := a.eventoCaja(ctx, tx, EventoVentaCobrada, doc.ID, map[string]any{"documento": doc, "lineas": o.Lineas, "cuenta": cuenta,
			"meseroId": o.MeseroID, "meseroNombre": o.MeseroNombre, "propina": propina.String(), "propinaRetirada": o.PropinaRetirada}, now); err != nil {
			return err
		}
		if fact == nil {
			return nil
		}
		// La nube firma y envía al SRI (el .p12 no sale de ella); el hash asegura que el
		// contenido tributario llega intacto (F5-05, F5-08).
		return a.eventoCaja(ctx, tx, EventoComprobanteEmitido, fact.ID, map[string]any{
			"id": fact.ID, "documentoId": doc.ID, "tipo": "01", "ambiente": fact.Ambiente, "puntoEmisionId": fact.Punto.ID,
			"serie": fact.Punto.Serie(), "secuencial": fact.Secuencial, "claveAcceso": fact.Clave.String(),
			"fechaEmision": fact.FechaLocal.Format(time.DateOnly), "importeTotal": fact.Desglose.ImporteTotal.String(),
			"xml": string(fact.XML), "hash": fact.Hash}, now)
	})
	if err != nil {
		return CobroOut{}, err
	}
	a.motor.Despertar(despertar...)
	if mesa != nil {
		_ = a.hub.Difundir(eventos.TableUnlocked{TableID: *mesa, Reason: "RELEASED"})
		a.avisarMesa(ctx, *mesa)
	}
	a.notificarPush()
	return out, nil
}

type pagoValido struct {
	PagoIn
	monto, recibido money.Money
}

// resolverPagos valida el pago (simple o mixto) contra el total: métodos activos, un solo
// pago en efectivo (el único que da vuelto) y montos que suman exactamente el total.
func resolverPagos(ctx context.Context, tx *store.Tx, pagos []pagoValido, total money.Money, unico bool) (out []PagoDoc, recibido, vuelto money.Money, abreCajon bool, err error) {
	suma := money.Money{}
	efectivos := 0
	for i := range pagos {
		p := &pagos[i]
		var d PagoDoc
		var abre bool
		err := tx.QueryRowContext(ctx, `SELECT nombre, tipo, codigo_forma_pago_sri, abre_cajon FROM metodos_pago WHERE id = ? AND deleted_at IS NULL AND activo = 1`, p.MetodoID.String()).
			Scan(&d.Metodo, &d.Tipo, &d.CodigoSRI, &abre)
		if errors.Is(err, sql.ErrNoRows) {
			return nil, money.Money{}, money.Money{}, false, invalido("Ese método de pago no existe o está desactivado.")
		}
		if err != nil {
			return nil, money.Money{}, money.Money{}, false, err
		}
		if unico {
			p.monto = total
		}
		d.MetodoID, d.Monto, d.Referencia, d.Lote, d.Ultimos4 = p.MetodoID, p.monto.String(), p.Referencia, p.Lote, p.Ultimos4
		if d.Tipo == "EFECTIVO" {
			efectivos++
			if p.recibido.IsZero() {
				p.recibido = p.monto
			}
			if p.recibido.LessThan(p.monto) {
				falta := "el total ($" + total.String() + ")"
				if !unico {
					falta = "su parte en efectivo ($" + p.monto.String() + ")"
				}
				return nil, money.Money{}, money.Money{}, false, invalido("Lo recibido ($" + p.recibido.String() + ") no alcanza para " + falta + ".")
			}
			recibido, vuelto = p.recibido, p.recibido.Sub(p.monto)
			d.Recibido, d.Vuelto = recibido.String(), vuelto.String()
		} else if !p.recibido.IsZero() && !p.recibido.Equal(p.monto) {
			return nil, money.Money{}, money.Money{}, false, invalido("Con " + d.Metodo + " se cobra el monto exacto. Para combinar métodos usa el pago mixto.")
		}
		abreCajon = abreCajon || abre
		suma = suma.Add(p.monto)
		out = append(out, d)
	}
	if efectivos > 1 {
		return nil, money.Money{}, money.Money{}, false, invalido("Registra el efectivo en un solo pago.")
	}
	if !suma.Equal(total) {
		return nil, money.Money{}, money.Money{}, false, invalido("Los pagos suman $" + suma.String() + " y el total es $" + total.String() + ".")
	}
	return out, recibido, vuelto, abreCajon, nil
}

// lineasTicket: el detalle del documento. Con división, lo que lleva esa cuenta de cada plato
// («1/3 Pizza» si se repartió); sin división, la orden completa.
func lineasTicket(o Orden, cuenta *CuentaTotales, fracciones map[ids.ID]string) []escpos.LineaCuenta {
	if cuenta == nil {
		return lineasCuenta(o.Lineas)
	}
	var out []escpos.LineaCuenta
	for _, l := range o.Lineas {
		m, ok := cuenta.Lineas[l.ID]
		if !ok || l.Estado == "ANULADA" {
			continue
		}
		cant := l.Cantidad
		if fr := fracciones[l.ID]; fr != "" {
			cant = fr
		}
		out = append(out, escpos.LineaCuenta{Cantidad: cant, Producto: l.Producto, Total: m})
	}
	return out
}
