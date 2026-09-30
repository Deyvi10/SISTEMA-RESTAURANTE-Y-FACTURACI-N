package db

import (
	"context"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Escuchar mantiene un LISTEN en el canal y avisa por despertar (sin bloquear) con cada
// NOTIFY. Si se pierde la conexión, reintenta con espera creciente hasta 30 s.
func Escuchar(ctx context.Context, pool *pgxpool.Pool, canal string, despertar chan<- struct{}, log *slog.Logger) {
	espera := time.Second
	for ctx.Err() == nil {
		err := func() error {
			pc, err := pool.Acquire(ctx)
			if err != nil {
				return err
			}
			conn := pc.Hijack()
			defer func() { _ = conn.Close(context.Background()) }()
			if _, err := conn.Exec(ctx, "LISTEN "+pgx.Identifier{canal}.Sanitize()); err != nil {
				return err
			}
			espera = time.Second
			for {
				if _, err := conn.WaitForNotification(ctx); err != nil {
					return err
				}
				select {
				case despertar <- struct{}{}:
				default:
				}
			}
		}()
		if ctx.Err() != nil {
			return
		}
		log.Warn("conexión LISTEN perdida, reintentando", "canal", canal, "err", err, "espera", espera)
		select {
		case <-ctx.Done():
			return
		case <-time.After(espera):
		}
		espera = min(espera*2, 30*time.Second)
	}
}
