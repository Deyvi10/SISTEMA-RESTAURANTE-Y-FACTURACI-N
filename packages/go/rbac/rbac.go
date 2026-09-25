// Package rbac define roles, permisos y la matriz por defecto (docs/requisitos/README.md §3).
// Un permiso efectivo = valor por defecto del rol, sobrescrito por permisos_usuario (⚙️).
package rbac

import "slices"

type Rol string

const (
	Admin  Rol = "ADMIN"
	Cajero Rol = "CAJERO"
	Mesero Rol = "MESERO"
	Cocina Rol = "COCINA"
	Bodega Rol = "BODEGA"
)

func (r Rol) Valido() bool { return slices.Contains([]Rol{Admin, Cajero, Mesero, Cocina, Bodega}, r) }

type Permiso string

// Catálogo de permisos. Agregar uno aquí no requiere migración (se guardan en filas).
const (
	TomarPedido       Permiso = "TOMAR_PEDIDO"
	AnularItemEnviado Permiso = "ANULAR_ITEM_ENVIADO"
	TransferirMesa    Permiso = "TRANSFERIR_MESA"
	ImprimirPrecuenta Permiso = "IMPRIMIR_PRECUENTA"
	Cobrar            Permiso = "COBRAR"
	DividirCuenta     Permiso = "DIVIDIR_CUENTA"
	DarDescuento      Permiso = "DAR_DESCUENTO"
	AbrirCajon        Permiso = "ABRIR_CAJON"
	EmitirNC          Permiso = "EMITIR_NC"
	GestionarTurno    Permiso = "GESTIONAR_TURNO"
	VerEsperadoCierre Permiso = "VER_ESPERADO_CIERRE"
	MarcarListo       Permiso = "MARCAR_LISTO"
	RecargarCupo      Permiso = "RECARGAR_CUPO"
	ConfigurarMenu    Permiso = "CONFIGURAR_MENU"
	ConfigurarSalon   Permiso = "CONFIGURAR_SALON"
	AjustarInventario Permiso = "AJUSTAR_INVENTARIO"
	GestionarPersonal Permiso = "GESTIONAR_PERSONAL"
	ConfigurarSRI     Permiso = "CONFIGURAR_SRI"
	VerReportes       Permiso = "VER_REPORTES"
)

// Info describe un permiso para la pantalla de interruptores (RF-01-07).
type Info struct {
	Permiso     Permiso `json:"permiso"`
	Nombre      string  `json:"nombre"`
	Descripcion string  `json:"descripcion"`
	// Configurable indica que el Admin puede cambiarlo por usuario (⚙️ en la matriz).
	Configurable map[Rol]bool `json:"-"`
}

type regla struct {
	defecto      map[Rol]bool
	configurable map[Rol]bool
	nombre, desc string
}

func r(nombre, desc string, def []Rol, conf []Rol) regla {
	d, c := map[Rol]bool{Admin: true}, map[Rol]bool{}
	for _, x := range def {
		d[x] = true
	}
	for _, x := range conf {
		c[x] = true
	}
	return regla{defecto: d, configurable: c, nombre: nombre, desc: desc}
}

// matriz reproduce la tabla RBAC: ✅ = defecto, ⚙️ = configurable (inicia en falso).
var matriz = map[Permiso]regla{
	TomarPedido:       r("Tomar pedidos", "Abrir mesas, agregar platos y enviar comandas", []Rol{Cajero, Mesero}, nil),
	AnularItemEnviado: r("Anular platos enviados", "Quitar un plato que ya se envió a cocina (imprime ticket de anulación)", nil, []Rol{Cajero, Mesero}),
	TransferirMesa:    r("Transferir y unir mesas", "Mover pedidos entre mesas o pasar una mesa a otro mesero", []Rol{Cajero}, []Rol{Mesero}),
	ImprimirPrecuenta: r("Imprimir pre-cuenta", "Imprimir el detalle de consumo sin valor tributario", []Rol{Cajero, Mesero}, nil),
	Cobrar:            r("Cobrar y facturar", "Registrar pagos y emitir comprobantes", []Rol{Cajero}, []Rol{Mesero}),
	DividirCuenta:     r("Dividir cuentas", "Separar una orden en varias cuentas", []Rol{Cajero}, []Rol{Mesero}),
	DarDescuento:      r("Dar descuentos y cortesías", "Aplicar descuentos con motivo", nil, []Rol{Cajero}),
	AbrirCajon:        r("Abrir cajón sin venta", "Abrir el cajón de dinero sin cobrar (queda auditado)", nil, []Rol{Cajero}),
	EmitirNC:          r("Emitir notas de crédito", "Revertir ventas facturadas", nil, []Rol{Cajero}),
	GestionarTurno:    r("Abrir y cerrar turnos", "Abrir la caja con fondo y hacer el cierre ciego", []Rol{Cajero}, nil),
	VerEsperadoCierre: r("Ver totales esperados", "Ver cuánto debería haber antes del cierre", nil, nil),
	MarcarListo:       r("Marcar pedidos listos", "Cambiar el estado de las comandas en el KDS", []Rol{Cocina}, nil),
	RecargarCupo:      r("Recargar platos del día", "Sumar unidades a un plato con cupo diario", nil, []Rol{Cajero, Cocina}),
	ConfigurarMenu:    r("Configurar menú", "Crear y editar categorías, productos, precios y recetas", nil, nil),
	ConfigurarSalon:   r("Configurar salón", "Editar zonas, mesas, estaciones e impresoras", nil, nil),
	AjustarInventario: r("Ajustar inventario", "Tomas físicas y ajustes", nil, []Rol{Cocina, Bodega}),
	GestionarPersonal: r("Gestionar personal", "Crear usuarios, PIN y dispositivos", nil, nil),
	ConfigurarSRI:     r("Configurar facturación SRI", "Firma electrónica y puntos de emisión", nil, nil),
	VerReportes:       r("Ver reportes", "Ventas, cierres y auditoría", nil, []Rol{Cajero}),
}

// Defecto indica si el rol tiene el permiso sin ajustes individuales.
func Defecto(rol Rol, p Permiso) bool { return matriz[p].defecto[rol] }

// Configurable indica si el Admin puede cambiar ese permiso para un usuario de ese rol.
func Configurable(rol Rol, p Permiso) bool { return matriz[p].configurable[rol] }

// Existe indica si el permiso está en el catálogo.
func Existe(p Permiso) bool { _, ok := matriz[p]; return ok }

// Efectivos combina el rol con los ajustes individuales. Un ajuste solo cuenta si el
// permiso es configurable para el rol: así nadie puede darle «Configurar SRI» a un mesero.
func Efectivos(rol Rol, ajustes map[Permiso]bool) map[Permiso]bool {
	out := make(map[Permiso]bool, len(matriz))
	for p, reg := range matriz {
		v := reg.defecto[rol]
		if a, ok := ajustes[p]; ok && reg.configurable[rol] {
			v = a
		}
		out[p] = v
	}
	return out
}

// Catalogo devuelve los permisos en orden estable para la UI.
func Catalogo() []Info {
	out := make([]Info, 0, len(matriz))
	for p, reg := range matriz {
		out = append(out, Info{Permiso: p, Nombre: reg.nombre, Descripcion: reg.desc, Configurable: reg.configurable})
	}
	slices.SortFunc(out, func(a, b Info) int {
		if a.Permiso < b.Permiso {
			return -1
		}
		return 1
	})
	return out
}
