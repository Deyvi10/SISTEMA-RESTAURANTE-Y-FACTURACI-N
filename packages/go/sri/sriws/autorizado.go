package sriws

import (
	"bytes"
	"errors"
	"time"

	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/clock"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/sri"
)

// XMLAutorizado arma el comprobante autorizado que se entrega al comprador y se archiva: el
// elemento <autorizacion> del WSDL oficial de autorización (estado, numeroAutorizacion,
// fechaAutorizacion, ambiente y el comprobante firmado en CDATA), sin mensajes. Es
// determinista: el mismo comprobante y autorización dan los mismos bytes.
func XMLAutorizado(firmado []byte, numero string, fecha time.Time, ambiente sri.Ambiente) ([]byte, error) {
	if bytes.Contains(firmado, []byte("]]>")) {
		return nil, errors.New("sriws: el comprobante no puede ir en CDATA")
	}
	amb := "PRODUCCIÓN"
	if ambiente == sri.AmbientePruebas {
		amb = "PRUEBAS"
	}
	var b bytes.Buffer
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n<autorizacion>\n<estado>AUTORIZADO</estado>\n<numeroAutorizacion>")
	b.WriteString(numero)
	b.WriteString("</numeroAutorizacion>\n<fechaAutorizacion>")
	b.WriteString(fecha.In(clock.Guayaquil).Format("2006-01-02T15:04:05-07:00"))
	b.WriteString("</fechaAutorizacion>\n<ambiente>" + amb + "</ambiente>\n<comprobante><![CDATA[")
	b.Write(firmado)
	b.WriteString("]]></comprobante>\n</autorizacion>\n")
	return b.Bytes(), nil
}
