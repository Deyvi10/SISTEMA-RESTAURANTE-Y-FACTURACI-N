package nodos

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/auth"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/platform/db"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/edgesync"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/ids"
)

// MaxEspera del long-poll: menor que el WriteTimeout del servidor (60 s) y que los
// timeouts típicos de proxies (60-100 s).
const MaxEspera = 25 * time.Second

// Avisos despierta a los nodos que esperan cambios de su tenant. Escucha el canal
// sync_cambios de PostgreSQL (NOTIFY desde el trigger, entregado al hacer commit).
type Avisos struct {
	mu   sync.Mutex
	subs map[ids.ID]map[chan struct{}]struct{}
}

func NewAvisos() *Avisos { return &Avisos{subs: map[ids.ID]map[chan struct{}]struct{}{}} }

// Suscribir devuelve un canal que recibe una señal cuando hay cambios del tenant.
func (a *Avisos) Suscribir(tenant ids.ID) (<-chan struct{}, func()) {
	ch := make(chan struct{}, 1)
	a.mu.Lock()
	if a.subs[tenant] == nil {
		a.subs[tenant] = map[chan struct{}]struct{}{}
	}
	a.subs[tenant][ch] = struct{}{}
	a.mu.Unlock()
	return ch, func() {
		a.mu.Lock()
		delete(a.subs[tenant], ch)
		if len(a.subs[tenant]) == 0 {
			delete(a.subs, tenant)
		}
		a.mu.Unlock()
	}
}

// Avisar despierta a todos los suscriptores del tenant (sin bloquear).
func (a *Avisos) Avisar(tenant ids.ID) {
	a.mu.Lock()
	defer a.mu.Unlock()
	for ch := range a.subs[tenant] {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
}

// Escuchar mantiene un LISTEN hasta que ctx termine, reconectando si se cae la conexión.
// Si se pierde un aviso durante la reconexión no pasa nada: el nodo reintenta el pull.
func (a *Avisos) Escuchar(ctx context.Context, pool *pgxpool.Pool, log *slog.Logger) {
	espera := time.Second
	for ctx.Err() == nil {
		err := a.escucharUna(ctx, pool)
		if ctx.Err() != nil {
			return
		}
		log.Warn("avisos de sync: conexión perdida, reintentando", "err", err, "espera", espera)
		select {
		case <-ctx.Done():
			return
		case <-time.After(espera):
		}
		espera = min(espera*2, 30*time.Second)
	}
}

func (a *Avisos) escucharUna(ctx context.Context, pool *pgxpool.Pool) error {
	pc, err := pool.Acquire(ctx)
	if err != nil {
		return err
	}
	// La conexión queda con LISTEN activo: se saca del pool y se cierra al terminar.
	conn := pc.Hijack()
	defer func() { _ = conn.Close(context.Background()) }()
	if _, err := conn.Exec(ctx, "LISTEN sync_cambios"); err != nil {
		return err
	}
	for {
		n, err := conn.WaitForNotification(ctx)
		if err != nil {
			return err
		}
		if t, err := ids.Parse(n.Payload); err == nil {
			a.Avisar(t)
		}
	}
}

// consultasVolcado arma el SELECT de cada tabla para el volcado completo, filtrando por
// el local del nodo donde aplica y quitando lo que el nodo no debe tener.
func consultasVolcado(tabla string) (string, bool) {
	switch tabla {
	case "locales":
		return `SELECT to_jsonb(t) FROM locales t WHERE t.id = $1 AND t.deleted_at IS NULL`, true
	case "estaciones", "zonas", "mesas", "impresoras":
		return fmt.Sprintf(`SELECT to_jsonb(t) FROM %s t WHERE t.local_id = $1 AND t.deleted_at IS NULL`, tabla), true
	case "usuario_locales", "estacion_impresoras":
		return fmt.Sprintf(`SELECT to_jsonb(t) FROM %s t WHERE t.local_id = $1`, tabla), true
	case "comandos_nodo":
		// Solo órdenes recientes y pendientes: un volcado nunca repite una prueba vieja.
		return `SELECT to_jsonb(t) FROM comandos_nodo t WHERE t.local_id = $1 AND t.ejecutado_at IS NULL AND t.created_at > now() - interval '10 minutes'`, true
	case "categorias", "productos", "grupos_modificadores":
		return fmt.Sprintf(`SELECT to_jsonb(t) FROM %s t WHERE t.deleted_at IS NULL`, tabla), false
	case "usuarios":
		return `SELECT to_jsonb(t) - 'password_hash' - 'totp_secret_cifrado' - 'email' - 'debe_cambiar_password' FROM usuarios t`, false
	case "modificadores", "producto_grupos_modificadores", "notas_rapidas", "permisos_usuario", "tarifas_iva":
		return fmt.Sprintf(`SELECT to_jsonb(t) FROM %s t`, tabla), false
	}
	return "", false
}

// Pull entrega al nodo los cambios desde su cursor (F2-03). Si no hay nada nuevo espera
// hasta `esperar` a que llegue algo (long-poll). Responde un volcado completo si el nodo
// lo pide (primera vez o repaso diario), si su cursor está adelantado (la nube se
// restauró) o si ya se purgaron cambios que necesita.
func (s *Service) Pull(ctx context.Context, n auth.Nodo, desde int64, volcado bool, esperar time.Duration) (edgesync.PullResponse, error) {
	if volcado {
		return s.volcado(ctx, n)
	}
	esperar = min(max(esperar, 0), MaxEspera)
	var ch <-chan struct{}
	if esperar > 0 && s.Avisos != nil {
		var cancel func()
		ch, cancel = s.Avisos.Suscribir(n.TenantID) // antes de consultar: no se pierde un aviso
		defer cancel()
	}
	limite := time.NewTimer(esperar)
	defer limite.Stop()
	for {
		res, err := s.pullUna(ctx, n, desde)
		if err != nil || res.Modo == edgesync.PullCompleto || len(res.Cambios) > 0 || res.Hasta > desde || ch == nil {
			return res, err
		}
		select {
		case <-ch:
		case <-limite.C:
			return res, nil
		case <-ctx.Done():
			return res, nil
		}
	}
}

func (s *Service) pullUna(ctx context.Context, n auth.Nodo, desde int64) (edgesync.PullResponse, error) {
	var ultimo int64
	var minimo *int64
	res := edgesync.PullResponse{Modo: edgesync.PullIncremental, Cambios: []edgesync.Cambio{}}
	err := s.DB.InTenant(ctx, n.TenantID, func(tx db.Tx) error {
		err := tx.QueryRow(ctx, `SELECT coalesce((SELECT ultimo FROM sync_seq_tenant), 0), (SELECT min(seq) FROM sync_cambios)`).Scan(&ultimo, &minimo)
		if err != nil {
			return err
		}
		if desde > ultimo || (minimo != nil && desde < *minimo-1) {
			res.Modo = edgesync.PullCompleto
			return nil
		}
		rows, err := tx.Query(ctx, `SELECT seq, tabla, op, local_id, datos FROM sync_cambios WHERE seq > $1 ORDER BY seq LIMIT $2`, desde, edgesync.MaxPull)
		if err != nil {
			return err
		}
		defer rows.Close()
		res.Hasta = desde
		for rows.Next() {
			var c edgesync.Cambio
			var local *ids.ID
			if err := rows.Scan(&c.Seq, &c.Tabla, &c.Op, &local, &c.Datos); err != nil {
				return err
			}
			res.Hasta = c.Seq // el cursor avanza también sobre cambios de otros locales
			if local != nil && *local != n.LocalID {
				continue
			}
			res.Cambios = append(res.Cambios, c)
		}
		res.Mas = res.Hasta < ultimo
		return rows.Err()
	})
	if err != nil || res.Modo == edgesync.PullIncremental {
		return res, err
	}
	return s.volcado(ctx, n)
}

// volcado lee todo lo que el nodo replica en una sola foto consistente.
func (s *Service) volcado(ctx context.Context, n auth.Nodo) (edgesync.PullResponse, error) {
	res := edgesync.PullResponse{Modo: edgesync.PullCompleto, Cambios: []edgesync.Cambio{}}
	err := s.DB.InTenantSnapshot(ctx, n.TenantID, func(tx db.Tx) error {
		if err := tx.QueryRow(ctx, `SELECT coalesce((SELECT ultimo FROM sync_seq_tenant), 0)`).Scan(&res.Hasta); err != nil {
			return err
		}
		for _, tabla := range edgesync.TablasReplica {
			q, porLocal := consultasVolcado(tabla)
			if q == "" {
				return fmt.Errorf("volcado: tabla sin consulta: %s", tabla)
			}
			var args []any
			if porLocal {
				args = append(args, n.LocalID)
			}
			rows, err := tx.Query(ctx, q, args...)
			if err != nil {
				return err
			}
			datos, err := pgx.CollectRows(rows, pgx.RowTo[json.RawMessage])
			if err != nil {
				return err
			}
			for _, d := range datos {
				res.Cambios = append(res.Cambios, edgesync.Cambio{Tabla: tabla, Op: "U", Datos: d})
			}
		}
		return nil
	})
	if errors.Is(err, pgx.ErrNoRows) {
		err = nil
	}
	return res, err
}
