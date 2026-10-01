package sriws

// Explicacion traduce un código de error o advertencia del SRI (ficha técnica offline v2.34,
// §11 «Códigos de errores y advertencias de validación») a lenguaje claro para el dueño, con
// qué hacer y si lo resuelve él o el soporte del sistema.
type Explicacion struct {
	Que     string `json:"que"`
	Accion  string `json:"accion"`
	Soporte bool   `json:"soporte"` // lo resuelve el proveedor del sistema, no el dueño
}

var explicaciones = map[string]Explicacion{
	"2":  {"El SRI tiene tu RUC como NO ACTIVO.", "Reactiva tu RUC en el SRI y luego reintenta.", false},
	"10": {"El establecimiento está clausurado en el SRI.", "El SRI autoriza de nuevo al terminar la clausura; luego reintenta.", false},
	"26": {"El comprobante supera el tamaño máximo que acepta el SRI.", "Contacta al soporte.", true},
	"27": {"Tu clase de contribuyente no puede emitir comprobantes electrónicos.", "Consulta tu situación en el SRI.", false},
	"28": {"No has aceptado el acuerdo de medios electrónicos del SRI.", "Acéptalo en SRI en línea y luego reintenta.", false},
	"35": {"El XML no pasó la validación de esquema del SRI.", "Contacta al soporte.", true},
	"36": {"La versión del esquema del comprobante está descontinuada.", "Contacta al soporte.", true},
	"37": {"Tu RUC no tiene autorización para emitir comprobantes electrónicos.", "Solicita en SRI en línea la emisión electrónica y luego reintenta.", false},
	"39": {"El SRI no reconoce la firma electrónica como válida.", "Revisa tu firma: si venció o la revocaron, sube la nueva en Facturación SRI.", false},
	"40": {"El SRI no pudo leer el certificado de la firma.", "Sube de nuevo tu firma electrónica (.p12) en Facturación SRI.", false},
	"43": {"El SRI ya tiene esta clave de acceso.", "No hace falta nada: se consulta su autorización.", false},
	"45": {"El SRI ya tiene otro comprobante con este número.", "Contacta al soporte: hay que revisar la numeración del punto de emisión.", true},
	"46": {"El RUC del emisor no existe en el SRI.", "Revisa el RUC del restaurante.", false},
	"47": {"El SRI no reconoce el tipo de comprobante.", "Contacta al soporte.", true},
	"48": {"No existe el esquema para este tipo de comprobante.", "Contacta al soporte.", true},
	"49": {"El SRI recibió una petición vacía.", "Reintenta; si se repite, contacta al soporte.", true},
	"50": {"El SRI tuvo un error interno.", "Reintenta más tarde.", false},
	"52": {"El SRI encontró diferencias en los cálculos de la factura.", "Contacta al soporte con el número de la factura.", true},
	"56": {"El establecimiento está cerrado en el SRI.", "Actualiza tus establecimientos en el RUC y luego reintenta.", false},
	"57": {"Tu autorización para emitir comprobantes electrónicos está suspendida.", "Consulta tu situación en el SRI.", false},
	"58": {"La clave de acceso no coincide con los datos del comprobante.", "Contacta al soporte.", true},
	"63": {"Tu RUC está clausurado por el SRI.", "Consulta tu situación en el SRI.", false},
	"65": {"El comprobante se envió fuera del plazo permitido.", "Consulta con tu contador cómo regularizarlo.", false},
	"67": {"Una fecha del comprobante tiene un formato inválido.", "Contacta al soporte.", true},
	"70": {"El SRI todavía está procesando este comprobante.", "No hace falta nada: se consulta su autorización.", false},
	"80": {"La clave de acceso consultada tiene un formato inválido.", "Contacta al soporte.", true},
}

// Explicar devuelve la explicación del código; si no se conoce, un texto genérico con el
// mensaje original del SRI.
func Explicar(m Mensaje) Explicacion {
	if e, ok := explicaciones[m.Identificador]; ok {
		return e
	}
	return Explicacion{Que: "El SRI respondió: " + m.String(), Accion: "Revisa el mensaje; si no es claro, contacta al soporte.", Soporte: true}
}
