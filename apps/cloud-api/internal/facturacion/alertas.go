package facturacion

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/auth"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/certificados"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/platform/db"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/platform/mail"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/ids"
)

// Niveles de alerta.
const (
	Advertencia = "ADVERTENCIA"
	Critica     = "CRITICA"
)

// Alerta fiscal para el dueño (F5-16, RF-05-04.4): en lenguaje claro y con qué hacer.
type Alerta struct {
	Clave  string `json:"clave"`
	Nivel  string `json:"nivel"`
	Titulo string `json:"titulo"`
	Accion string `json:"accion"`
	Enlace string `json:"enlace"` // sección del panel donde se resuelve
}

// diasAvisoCertificado: el correo de vencimiento sale a 30, 15, 7 y 1 días (F5-06).
var diasAvisoCertificado = map[int]bool{30: true, 15: true, 7: true, 1: true}

// AlertasFiscales calcula las alertas del restaurante ahora.
func (s *Service) AlertasFiscales(ctx context.Context, p auth.Principal) ([]Alerta, error) {
	var out []Alerta
	err := s.DB.InTenant(ctx, p.TenantID, func(tx db.Tx) error {
		var err error
		out, err = calcularAlertas(ctx, tx, time.Now())
		return err
	})
	return out, err
}

func calcularAlertas(ctx context.Context, tx db.Tx, ahora time.Time) ([]Alerta, error) {
	out := []Alerta{}
	var activa bool
	var umbral int
	err := tx.QueryRow(ctx, `SELECT facturacion_activa, alerta_sin_autorizar_horas FROM configuracion_fiscal`).Scan(&activa, &umbral)
	if errors.Is(err, pgx.ErrNoRows) {
		return out, nil
	}
	if err != nil {
		return nil, err
	}
	// 1. Comprobantes que llevan más del umbral (12 h por defecto) sin autorización.
	var esperando int
	var masAntiguo *time.Time
	if err := tx.QueryRow(ctx, `SELECT count(*), min(recibido_at) FROM comprobantes
		WHERE estado IN ('EN_NUBE', 'FIRMADO', 'RECIBIDO') AND recibido_at < $1`, ahora.Add(-time.Duration(umbral)*time.Hour)).Scan(&esperando, &masAntiguo); err != nil {
		return nil, err
	}
	if esperando > 0 {
		a := Alerta{Clave: "sin-autorizar", Nivel: Advertencia, Enlace: "/comprobantes?estado=ENVIADO",
			Titulo: fmt.Sprintf("%d %s más de %d h sin autorización del SRI.", esperando, plural(esperando, "comprobante lleva", "comprobantes llevan"), umbral),
			Accion: "Revisa en Comprobantes por qué esperan (firma, conexión o el SRI)."}
		// Plazo legal de envío (DP-07): si está configurado, crítica al pasar el 80 %.
		var plazo string
		_ = tx.QueryRow(ctx, `SELECT valor FROM parametros_globales WHERE clave = 'plazo_envio_comprobante_horas'`).Scan(&plazo)
		if h, err := strconv.Atoi(strings.TrimSpace(plazo)); err == nil && h > 0 && masAntiguo != nil &&
			ahora.Sub(*masAntiguo) > time.Duration(h)*time.Hour*8/10 {
			a.Nivel = Critica
			a.Titulo = fmt.Sprintf("Hay comprobantes cerca del plazo legal de envío al SRI (%d h).", h)
			a.Accion = "Resuélvelo hoy: revisa en Comprobantes qué los detiene."
		}
		out = append(out, a)
	}
	// 2. Errores del SRI que necesitan a alguien.
	var conError int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM comprobantes WHERE estado IN ('DEVUELTO', 'NO_AUTORIZADO', 'REQUIERE_ATENCION')`).Scan(&conError); err != nil {
		return nil, err
	}
	if conError > 0 {
		out = append(out, Alerta{Clave: "errores-sri", Nivel: Critica, Enlace: "/comprobantes?estado=REQUIERE_ATENCION",
			Titulo: fmt.Sprintf("%d %s el SRI no aceptó.", conError, plural(conError, "comprobante que", "comprobantes que")),
			Accion: "Abre cada uno en Comprobantes: dice por qué y qué hacer."})
	}
	// 3. Firma electrónica: falta, venció o está por vencer.
	cert, err := certificados.Leer(ctx, tx, ahora)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		if activa {
			out = append(out, Alerta{Clave: "sin-firma", Nivel: Critica, Enlace: "/facturacion",
				Titulo: "Estás facturando sin firma electrónica: las facturas no llegan al SRI.", Accion: "Sube tu archivo .p12 en Facturación SRI."})
		}
	case err != nil:
		return nil, err
	case cert.DiasRestantes < 0:
		out = append(out, Alerta{Clave: "firma-vencida", Nivel: Critica, Enlace: "/facturacion",
			Titulo: "Tu firma electrónica venció: las facturas nuevas no se pueden enviar al SRI.", Accion: "Renuévala y sube la nueva en Facturación SRI."})
	case cert.DiasRestantes <= 30:
		nivel := Advertencia
		if cert.DiasRestantes <= 7 {
			nivel = Critica
		}
		out = append(out, Alerta{Clave: "firma-" + strconv.Itoa(cert.DiasRestantes), Nivel: nivel, Enlace: "/facturacion",
			Titulo: fmt.Sprintf("Tu firma electrónica vence en %d %s.", cert.DiasRestantes, plural(cert.DiasRestantes, "día", "días")),
			Accion: "Renuévala con tu entidad certificadora y súbela en Facturación SRI."})
	}
	return out, nil
}

func plural(n int, uno, varios string) string {
	if n == 1 {
		return uno
	}
	return varios
}

// NotificadorAlertas envía por correo a los dueños las alertas críticas y los avisos de
// vencimiento de la firma (30, 15, 7 y 1 días), una vez por alerta y día.
type NotificadorAlertas struct {
	DB   *db.DB
	Mail mail.Sender
	Log  *slog.Logger
	Now  func() time.Time
}

// Enviar revisa todos los restaurantes y devuelve cuántos correos mandó.
func (n *NotificadorAlertas) Enviar(ctx context.Context) (int, error) {
	var tenants []ids.ID
	if err := n.DB.Global(ctx, func(tx db.Tx) error {
		rows, err := tx.Query(ctx, `SELECT tenant_id FROM tenants_con_facturacion()`)
		if err != nil {
			return err
		}
		tenants, err = pgx.CollectRows(rows, pgx.RowTo[ids.ID])
		return err
	}); err != nil {
		return 0, err
	}
	enviados := 0
	ahora := n.Now()
	dia := ahora.In(guayaquil).Format(time.DateOnly)
	for _, t := range tenants {
		var alertas []Alerta
		var destinos []string
		var negocio string
		yaEnviadas := map[string]bool{}
		err := n.DB.InTenant(ctx, t, func(tx db.Tx) error {
			var err error
			if alertas, err = calcularAlertas(ctx, tx, ahora); err != nil {
				return err
			}
			_ = tx.QueryRow(ctx, `SELECT nombre_comercial FROM tenants WHERE id = app_tenant()`).Scan(&negocio)
			rows, err := tx.Query(ctx, `SELECT email::text FROM usuarios WHERE es_dueno AND activo AND email IS NOT NULL ORDER BY email`)
			if err != nil {
				return err
			}
			if destinos, err = pgx.CollectRows(rows, pgx.RowTo[string]); err != nil {
				return err
			}
			rows, err = tx.Query(ctx, `SELECT clave FROM alertas_enviadas WHERE dia = $1`, dia)
			if err != nil {
				return err
			}
			claves, err := pgx.CollectRows(rows, pgx.RowTo[string])
			for _, c := range claves {
				yaEnviadas[c] = true
			}
			return err
		})
		if err != nil {
			n.Log.Error("alertas: no se pudieron calcular", "tenant", t, "err", err)
			continue
		}
		for _, a := range alertas {
			if yaEnviadas[a.Clave] || !debeEnviarse(a) || len(destinos) == 0 {
				continue
			}
			text, html, err := mail.Render(mail.Contenido{Titulo: a.Titulo, Parrafos: []string{a.Accion}, Pie: negocio + " · Alerta fiscal"})
			if err != nil {
				return enviados, err
			}
			prefijo := "Aviso"
			if a.Nivel == Critica {
				prefijo = "⚠️ Alerta"
			}
			ok := true
			for _, to := range destinos {
				if err := n.Mail.Send(ctx, mail.Message{To: to, Subject: prefijo + ": " + a.Titulo, Text: text, HTML: html}); err != nil {
					n.Log.Warn("alertas: no se pudo enviar el correo", "tenant", t, "alerta", a.Clave, "err", err)
					ok = false
				}
			}
			if !ok {
				continue // se reintenta en la siguiente vuelta
			}
			if err := n.DB.InTenant(ctx, t, func(tx db.Tx) error {
				_, err := tx.Exec(ctx, `INSERT INTO alertas_enviadas (tenant_id, clave, dia, detalle) VALUES (app_tenant(), $1, $2, $3) ON CONFLICT DO NOTHING`,
					a.Clave, dia, a.Titulo)
				return err
			}); err != nil {
				return enviados, err
			}
			enviados++
		}
	}
	return enviados, nil
}

// debeEnviarse: las críticas siempre; la firma por vencer, solo los días 30, 15, 7 y 1.
func debeEnviarse(a Alerta) bool {
	if strings.HasPrefix(a.Clave, "firma-") {
		if d, err := strconv.Atoi(strings.TrimPrefix(a.Clave, "firma-")); err == nil {
			return diasAvisoCertificado[d]
		}
	}
	return a.Nivel == Critica
}

// Correr revisa al arrancar y cada hora.
func (n *NotificadorAlertas) Correr(ctx context.Context) {
	tick := time.NewTicker(time.Hour)
	defer tick.Stop()
	for {
		if _, err := n.Enviar(ctx); err != nil && ctx.Err() == nil {
			n.Log.Warn("alertas: no se pudieron revisar", "err", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
	}
}
