//go:build windows

package spooler

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"

	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/edge-node/internal/impresion"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/escpos"
)

var (
	winspool              = windows.NewLazySystemDLL("winspool.drv")
	procEnumPrinters      = winspool.NewProc("EnumPrintersW")
	procGetDefaultPrinter = winspool.NewProc("GetDefaultPrinterW")
	procOpenPrinter       = winspool.NewProc("OpenPrinterW")
	procClosePrinter      = winspool.NewProc("ClosePrinter")
	procGetPrinter        = winspool.NewProc("GetPrinterW")
	procStartDocPrinter   = winspool.NewProc("StartDocPrinterW")
	procStartPagePrinter  = winspool.NewProc("StartPagePrinter")
	procWritePrinter      = winspool.NewProc("WritePrinter")
	procEndPagePrinter    = winspool.NewProc("EndPagePrinter")
	procEndDocPrinter     = winspool.NewProc("EndDocPrinter")
)

const (
	printerEnumLocal       = 0x2
	printerEnumConnections = 0x4
)

// printerInfo2 es PRINTER_INFO_2W.
type printerInfo2 struct {
	ServerName         *uint16
	PrinterName        *uint16
	ShareName          *uint16
	PortName           *uint16
	DriverName         *uint16
	Comment            *uint16
	Location           *uint16
	DevMode            uintptr
	SepFile            *uint16
	PrintProcessor     *uint16
	Datatype           *uint16
	Parameters         *uint16
	SecurityDescriptor uintptr
	Attributes         uint32
	Priority           uint32
	DefaultPriority    uint32
	StartTime          uint32
	UntilTime          uint32
	Status             uint32
	Jobs               uint32
	AveragePPM         uint32
}

type docInfo1 struct {
	DocName    *uint16
	OutputFile *uint16
	Datatype   *uint16
}

func str(p *uint16) string {
	if p == nil {
		return ""
	}
	return windows.UTF16PtrToString(p)
}

// Listar devuelve las impresoras físicas instaladas en Windows.
func Listar() ([]Instalada, error) {
	var needed, returned uint32
	flags := uintptr(printerEnumLocal | printerEnumConnections)
	_, _, _ = procEnumPrinters.Call(flags, 0, 2, 0, 0, uintptr(unsafe.Pointer(&needed)), uintptr(unsafe.Pointer(&returned)))
	if needed == 0 {
		return nil, nil
	}
	buf := make([]byte, needed)
	r, _, err := procEnumPrinters.Call(flags, 0, 2, uintptr(unsafe.Pointer(&buf[0])), uintptr(needed), uintptr(unsafe.Pointer(&needed)), uintptr(unsafe.Pointer(&returned)))
	if r == 0 {
		return nil, fmt.Errorf("spooler: EnumPrinters: %w", err)
	}
	pred := predeterminada()
	infos := unsafe.Slice((*printerInfo2)(unsafe.Pointer(&buf[0])), returned)
	out := make([]Instalada, 0, len(infos))
	for _, pi := range infos {
		i := Instalada{Nombre: str(pi.PrinterName), Puerto: str(pi.PortName), Driver: str(pi.DriverName)}
		if i.Nombre == "" || Virtual(i.Nombre, i.Puerto, i.Driver) {
			continue
		}
		i.Estado = EtiquetaEstado(pi.Status, pi.Attributes)
		i.AnchoSugerido = AnchoSugerido(i.Nombre, i.Driver)
		i.Predeterminada = strings.EqualFold(i.Nombre, pred)
		i.Host, i.PuertoTCP = puertoTCP(i.Puerto)
		out = append(out, i)
	}
	return out, nil
}

func predeterminada() string {
	var n uint32
	_, _, _ = procGetDefaultPrinter.Call(0, uintptr(unsafe.Pointer(&n)))
	if n == 0 {
		return ""
	}
	b := make([]uint16, n)
	if r, _, _ := procGetDefaultPrinter.Call(uintptr(unsafe.Pointer(&b[0])), uintptr(unsafe.Pointer(&n))); r == 0 {
		return ""
	}
	return windows.UTF16ToString(b)
}

// puertoTCP lee del registro los datos de un puerto «Standard TCP/IP» en modo RAW.
func puertoTCP(puerto string) (string, int) {
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, `SYSTEM\CurrentControlSet\Control\Print\Monitors\Standard TCP/IP Port\Ports\`+puerto, registry.QUERY_VALUE)
	if err != nil {
		if ip := IPDePuerto(puerto); ip != "" {
			return ip, 9100
		}
		return "", 0
	}
	defer func() { _ = k.Close() }()
	if proto, _, err := k.GetIntegerValue("Protocol"); err == nil && proto != 1 {
		return "", 0 // LPR: se imprime por el spooler
	}
	host, _, _ := k.GetStringValue("HostName")
	if host == "" {
		host, _, _ = k.GetStringValue("IPAddress")
	}
	port, _, err := k.GetIntegerValue("PortNumber")
	if host == "" || err != nil || port == 0 {
		return "", 0
	}
	return host, int(port)
}

func abrir(nombre string) (windows.Handle, error) {
	p, err := windows.UTF16PtrFromString(nombre)
	if err != nil {
		return 0, err
	}
	var h windows.Handle
	if r, _, err := procOpenPrinter.Call(uintptr(unsafe.Pointer(p)), uintptr(unsafe.Pointer(&h)), 0); r == 0 {
		return 0, fmt.Errorf("%w: no se pudo abrir «%s»: %w", impresion.ErrSinConexion, nombre, err)
	}
	return h, nil
}

func cerrar(h windows.Handle) { _, _, _ = procClosePrinter.Call(uintptr(h)) }

// Transporte imprime por el spooler: el «addr» del motor es el nombre de la cola.
type Transporte struct{}

func (Transporte) Consultar(_ context.Context, nombre string) (escpos.Status, bool, error) {
	h, err := abrir(nombre)
	if err != nil {
		return escpos.Status{}, false, err
	}
	defer cerrar(h)
	var needed uint32
	_, _, _ = procGetPrinter.Call(uintptr(h), 2, 0, 0, uintptr(unsafe.Pointer(&needed)))
	if needed == 0 {
		return escpos.Status{}, false, nil
	}
	buf := make([]byte, needed)
	if r, _, _ := procGetPrinter.Call(uintptr(h), 2, uintptr(unsafe.Pointer(&buf[0])), uintptr(needed), uintptr(unsafe.Pointer(&needed))); r == 0 {
		return escpos.Status{}, false, nil
	}
	pi := (*printerInfo2)(unsafe.Pointer(&buf[0]))
	st, soporta := EstadoDe(pi.Status, pi.Attributes)
	return st, soporta, nil
}

func (Transporte) Enviar(_ context.Context, nombre string, data []byte) error {
	if len(data) == 0 {
		return nil
	}
	h, err := abrir(nombre)
	if err != nil {
		return err
	}
	defer cerrar(h)
	doc, _ := windows.UTF16PtrFromString("RestPOS")
	raw, _ := windows.UTF16PtrFromString("RAW")
	di := docInfo1{DocName: doc, Datatype: raw}
	if r, _, err := procStartDocPrinter.Call(uintptr(h), 1, uintptr(unsafe.Pointer(&di))); r == 0 {
		return fmt.Errorf("%w: StartDocPrinter: %w", impresion.ErrSinConexion, err)
	}
	defer func() { _, _, _ = procEndDocPrinter.Call(uintptr(h)) }()
	if r, _, err := procStartPagePrinter.Call(uintptr(h)); r == 0 {
		return fmt.Errorf("%w: StartPagePrinter: %w", impresion.ErrSinConexion, err)
	}
	var escritos uint32
	r, _, err := procWritePrinter.Call(uintptr(h), uintptr(unsafe.Pointer(&data[0])), uintptr(len(data)), uintptr(unsafe.Pointer(&escritos)))
	_, _, _ = procEndPagePrinter.Call(uintptr(h))
	if r == 0 {
		return fmt.Errorf("%w: WritePrinter: %w", impresion.ErrSinConexion, err)
	}
	if int(escritos) != len(data) {
		return errors.New("spooler: escritura incompleta")
	}
	return nil
}
