// Package limite bloquea temporalmente una clave tras varios intentos fallidos
// (login, activación de nodos). Guarda solo el hash de la clave: nunca correos ni IPs.
package limite

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/platform/db"
)

type Limiter struct {
	DB      *db.DB
	Now     func() time.Time
	Max     int           // fallos seguidos antes de bloquear
	Bloqueo time.Duration // duración del bloqueo
}

// Clave arma la clave de conteo a partir de sus partes (p. ej. usuario e IP).
func Clave(partes ...string) string {
	h := sha256.Sum256([]byte(strings.Join(partes, "|")))
	return hex.EncodeToString(h[:])
}

func (l *Limiter) Bloqueado(ctx context.Context, clave string) (bool, error) {
	var hasta *time.Time
	err := l.DB.Global(ctx, func(tx db.Tx) error {
		return tx.QueryRow(ctx, `SELECT bloqueado_hasta FROM intentos_login WHERE clave = $1`, clave).Scan(&hasta)
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	return err == nil && hasta != nil && l.Now().Before(*hasta), err
}

func (l *Limiter) Fallo(ctx context.Context, clave string) error {
	now := l.Now()
	return l.DB.Global(ctx, func(tx db.Tx) error {
		// Un bloqueo vencido reinicia el conteo.
		_, err := tx.Exec(ctx, `INSERT INTO intentos_login (clave, fallos, updated_at) VALUES ($1, 1, $2)
			ON CONFLICT (clave) DO UPDATE SET
			  fallos = CASE WHEN intentos_login.bloqueado_hasta IS NOT NULL AND intentos_login.bloqueado_hasta <= $2 THEN 1 ELSE intentos_login.fallos + 1 END,
			  bloqueado_hasta = CASE WHEN intentos_login.bloqueado_hasta IS NOT NULL AND intentos_login.bloqueado_hasta <= $2 THEN NULL
			                         WHEN intentos_login.fallos + 1 >= $3 THEN $2 + $4::interval ELSE intentos_login.bloqueado_hasta END,
			  updated_at = $2`, clave, now, l.Max, fmt.Sprintf("%d seconds", int(l.Bloqueo.Seconds())))
		return err
	})
}

func (l *Limiter) Limpiar(ctx context.Context, clave string) error {
	return l.DB.Global(ctx, func(tx db.Tx) error {
		_, err := tx.Exec(ctx, `DELETE FROM intentos_login WHERE clave = $1`, clave)
		return err
	})
}
