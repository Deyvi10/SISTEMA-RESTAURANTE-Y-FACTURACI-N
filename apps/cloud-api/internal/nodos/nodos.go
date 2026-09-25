// Package nodos activa, lista y revoca los Nodos Locales (F2-02) y recibe su telemetría
// y sus eventos (F2-04).
package nodos

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/auth"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/platform/apperr"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/platform/db"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/platform/limite"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/clock"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/edgesync"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/ids"
)

const (
	// CodigoTTL: el dueño lo escribe en el nodo en el momento; 30 min sobra.
	CodigoTTL = 30 * time.Minute
	// Alfabeto sin caracteres confundibles (0/O, 1/I/L): 31^8 ≈ 8,5·10¹¹ combinaciones,
	// con bloqueo tras 10 intentos por IP y vida de 30 min, adivinarlo es inviable.
	alfabeto    = "ABCDEFGHJKMNPQRSTUVWXYZ23456789"
	largoCodigo = 8
	// EnLineaSi: sin heartbeat en 3 min (se envía cada 60 s) el nodo figura desconectado.
	EnLineaSi = 3 * time.Minute
)

type Service struct {
	DB      *db.DB
	Clock   clock.Clock
	Limiter *limite.Limiter // intentos de activación por IP
	Avisos  *Avisos         // long-poll del pull (nil = sin espera)
}

// New arma el servicio con su límite de intentos: 10 códigos erróneos por IP → 15 min.
func New(d *db.DB, clk clock.Clock) *Service {
	return &Service{DB: d, Clock: clk, Avisos: NewAvisos(), Limiter: &limite.Limiter{DB: d, Now: clk.Now, Max: 10, Bloqueo: 15 * time.Minute}}
}

// NuevoCodigo genera un código legible «ABCD-EFGH».
func NuevoCodigo() string {
	b := make([]byte, largoCodigo)
	_, _ = rand.Read(b)
	out := make([]byte, 0, largoCodigo+1)
	for i, x := range b {
		if i == largoCodigo/2 {
			out = append(out, '-')
		}
		out = append(out, alfabeto[int(x)%len(alfabeto)]) // sesgo despreciable: 256 % 31 = 8
	}
	return string(out)
}

// normalizar acepta el código con o sin guion, espacios o minúsculas.
func normalizar(c string) (string, bool) {
	var b strings.Builder
	for _, r := range strings.ToUpper(c) {
		switch {
		case r == '-' || r == ' ':
			continue
		case strings.ContainsRune(alfabeto, r):
			b.WriteRune(r)
		default:
			return "", false
		}
	}
	return b.String(), b.Len() == largoCodigo
}

func hashCodigo(normalizado string) []byte { return auth.HashToken("nodo:" + normalizado) }

// ---------- Backoffice ----------

type CodigoGenerado struct {
	Codigo   string    `json:"codigo"`
	LocalID  ids.ID    `json:"localId"`
	ExpiraAt time.Time `json:"expiraAt"`
}

// GenerarCodigo crea un código de un solo uso para un local. Los códigos anteriores
// sin usar de ese local dejan de valer: solo hay uno vigente a la vez.
func (s *Service) GenerarCodigo(ctx context.Context, p auth.Principal, in struct {
	LocalID ids.ID `json:"localId"`
},
) (CodigoGenerado, error) {
	codigo := NuevoCodigo()
	norm, _ := normalizar(codigo)
	now := s.Clock.Now()
	out := CodigoGenerado{Codigo: codigo, LocalID: in.LocalID, ExpiraAt: now.Add(CodigoTTL)}
	err := s.DB.InTenant(ctx, p.TenantID, func(tx db.Tx) error {
		var existe bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM locales WHERE id = $1 AND deleted_at IS NULL)`, in.LocalID).Scan(&existe); err != nil {
			return err
		}
		if !existe {
			return &apperr.Error{Kind: apperr.Invalid, Code: "LOCAL_INVALIDO", Message: "Elige el local donde está la PC de caja.", Fields: []apperr.FieldError{{Campo: "localId", Mensaje: "Elige un local."}}}
		}
		if _, err := tx.Exec(ctx, `UPDATE codigos_activacion_nodo SET expira_at = $2 WHERE local_id = $1 AND usado_at IS NULL AND expira_at > $2`, in.LocalID, now); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `INSERT INTO codigos_activacion_nodo (id, tenant_id, local_id, codigo_hash, expira_at, creado_por) VALUES ($1, $2, $3, $4, $5, $6)`,
			ids.New(), p.TenantID, in.LocalID, hashCodigo(norm), out.ExpiraAt, p.UserID)
		return err
	})
	return out, err
}

type Nodo struct {
	ID                ids.ID          `json:"id"`
	LocalID           ids.ID          `json:"localId"`
	LocalNombre       string          `json:"localNombre"`
	NombreEquipo      string          `json:"nombreEquipo"`
	Estado            string          `json:"estado"`
	Version           string          `json:"version"`
	EnLinea           bool            `json:"enLinea"`
	UltimoHeartbeatAt *time.Time      `json:"ultimoHeartbeatAt"`
	Salud             json.RawMessage `json:"salud"`
	ActivadoAt        time.Time       `json:"activadoAt"`
	RevocadoAt        *time.Time      `json:"revocadoAt"`
}

func (s *Service) Listar(ctx context.Context, p auth.Principal) ([]Nodo, error) {
	var out []Nodo
	now := s.Clock.Now()
	err := s.DB.InTenant(ctx, p.TenantID, func(tx db.Tx) error {
		rows, err := tx.Query(ctx, `SELECT n.id, n.local_id, l.nombre, n.nombre_equipo, n.estado, n.version_software, n.ultimo_heartbeat_at, n.heartbeat, n.activado_at, n.revocado_at
			FROM nodos n JOIN locales l ON l.id = n.local_id
			ORDER BY (n.estado = 'ACTIVO') DESC, n.activado_at DESC LIMIT 50`)
		if err != nil {
			return err
		}
		out, err = pgx.CollectRows(rows, func(r pgx.CollectableRow) (Nodo, error) {
			var n Nodo
			err := r.Scan(&n.ID, &n.LocalID, &n.LocalNombre, &n.NombreEquipo, &n.Estado, &n.Version, &n.UltimoHeartbeatAt, &n.Salud, &n.ActivadoAt, &n.RevocadoAt)
			n.EnLinea = n.Estado == "ACTIVO" && n.UltimoHeartbeatAt != nil && now.Sub(*n.UltimoHeartbeatAt) < EnLineaSi
			return n, err
		})
		return err
	})
	if out == nil {
		out = []Nodo{}
	}
	return out, err
}

// Revocar desactiva un nodo (PC robada, reemplazada o dañada). Es inmediato: su próxima
// petición a la nube se rechaza.
func (s *Service) Revocar(ctx context.Context, p auth.Principal, id ids.ID) error {
	return s.DB.InTenant(ctx, p.TenantID, func(tx db.Tx) error {
		tag, err := tx.Exec(ctx, `UPDATE nodos SET estado = 'REVOCADO', revocado_at = $2 WHERE id = $1 AND estado = 'ACTIVO'`, id, s.Clock.Now())
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return apperr.ErrNotFound
		}
		return nil
	})
}

// ---------- Nodo ----------

// Activacion es lo que envía el nodo: el código que escribió el dueño y su identidad.
type Activacion struct {
	Codigo       string `json:"codigo"`
	NodoID       ids.ID `json:"nodoId"`
	LlavePublica string `json:"llavePublica"` // ed25519 en base64
	Version      string `json:"version"`
	NombreEquipo string `json:"nombreEquipo"`
}

type Activado struct {
	NodoID          ids.ID `json:"nodoId"`
	TenantID        ids.ID `json:"tenantId"`
	LocalID         ids.ID `json:"localId"`
	NombreLocal     string `json:"nombreLocal"`
	NombreComercial string `json:"nombreComercial"`
}

var (
	errCodigo   = apperr.New(apperr.Invalid, "CODIGO_INVALIDO", "El código no es correcto o ya venció. Genera uno nuevo en el backoffice, en Nodo Local.")
	errBloqueo  = apperr.New(apperr.TooMany, "DEMASIADOS_INTENTOS", "Demasiados códigos incorrectos. Espera 15 minutos e intenta de nuevo.")
	errIdentity = apperr.New(apperr.Invalid, "IDENTIDAD_INVALIDA", "La identidad del nodo no es válida. Reinstala el Nodo Local.")
)

// Activar canjea un código de un solo uso. Revoca cualquier otro nodo ACTIVO del mismo
// local (RF-02-10.4). Es idempotente: si el nodo reintenta con el mismo id y la misma
// llave (p. ej. se cortó la red antes de recibir la respuesta), recibe la misma respuesta.
func (s *Service) Activar(ctx context.Context, in Activacion, ip string) (Activado, error) {
	clave := limite.Clave("nodo-activacion", ip)
	bloqueado, err := s.Limiter.Bloqueado(ctx, clave)
	if err != nil {
		return Activado{}, err
	}
	if bloqueado {
		return Activado{}, errBloqueo
	}
	norm, ok := normalizar(in.Codigo)
	if !ok {
		_ = s.Limiter.Fallo(ctx, clave)
		return Activado{}, errCodigo
	}
	pub, err := base64.StdEncoding.DecodeString(in.LlavePublica)
	if err != nil || len(pub) != ed25519.PublicKeySize || in.NodoID == ids.Nil || in.NodoID.Version() != 7 {
		return Activado{}, errIdentity
	}
	var codigoID, tenantID, localID ids.ID
	var expira time.Time
	var usado *time.Time
	err = s.DB.Global(ctx, func(tx db.Tx) error {
		return tx.QueryRow(ctx, `SELECT codigo_id, tenant_id, local_id, expira_at, usado_at FROM nodo_buscar_codigo($1)`, hashCodigo(norm)).
			Scan(&codigoID, &tenantID, &localID, &expira, &usado)
	})
	if errors.Is(err, pgx.ErrNoRows) {
		_ = s.Limiter.Fallo(ctx, clave)
		return Activado{}, errCodigo
	}
	if err != nil {
		return Activado{}, err
	}
	now := s.Clock.Now()
	out := Activado{NodoID: in.NodoID, TenantID: tenantID, LocalID: localID}
	err = s.DB.InTenant(ctx, tenantID, func(tx db.Tx) error {
		if usado != nil {
			// ¿Es el mismo nodo reintentando?
			var llave []byte
			err := tx.QueryRow(ctx, `SELECT n.llave_publica FROM nodos n JOIN codigos_activacion_nodo c ON c.nodo_id = n.id
				WHERE c.id = $1 AND n.id = $2 AND n.estado = 'ACTIVO'`, codigoID, in.NodoID).Scan(&llave)
			if errors.Is(err, pgx.ErrNoRows) || (err == nil && !ed25519.PublicKey(llave).Equal(ed25519.PublicKey(pub))) {
				return errCodigo
			}
			if err != nil {
				return err
			}
		} else {
			if !now.Before(expira) {
				return errCodigo
			}
			var creadoPor ids.ID
			err := tx.QueryRow(ctx, `UPDATE codigos_activacion_nodo SET usado_at = $2, nodo_id = $3
				WHERE id = $1 AND usado_at IS NULL AND expira_at > $2 RETURNING creado_por`, codigoID, now, in.NodoID).Scan(&creadoPor)
			if errors.Is(err, pgx.ErrNoRows) {
				return errCodigo // otro nodo lo canjeó en paralelo
			}
			if err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `UPDATE nodos SET estado = 'REVOCADO', revocado_at = $2 WHERE local_id = $1 AND estado = 'ACTIVO'`, localID, now); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `INSERT INTO nodos (id, tenant_id, local_id, nombre_equipo, llave_publica, version_software, activado_at, activado_por)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
				in.NodoID, tenantID, localID, truncar(in.NombreEquipo, 80), pub, truncar(in.Version, 40), now, creadoPor); err != nil {
				if db.IsUniqueViolation(err, "nodos_pkey") {
					return errIdentity
				}
				return err
			}
		}
		return tx.QueryRow(ctx, `SELECT l.nombre, t.nombre_comercial FROM locales l JOIN tenants t ON t.id = l.tenant_id WHERE l.id = $1`, localID).
			Scan(&out.NombreLocal, &out.NombreComercial)
	})
	if errors.Is(err, errCodigo) {
		_ = s.Limiter.Fallo(ctx, clave)
	}
	if err != nil {
		return Activado{}, err
	}
	_ = s.Limiter.Limpiar(ctx, clave)
	return out, nil
}

// Heartbeat registra la salud del nodo y le devuelve la hora de la nube (F2-04).
func (s *Service) Heartbeat(ctx context.Context, n auth.Nodo, hb edgesync.Heartbeat) (edgesync.HeartbeatResponse, error) {
	now := s.Clock.Now()
	deriva := hb.HoraNodo.Sub(now)
	res := edgesync.HeartbeatResponse{HoraNube: now, DerivaSegundos: int64(deriva / time.Second)}
	res.AlertaReloj = deriva > edgesync.MaxDerivaReloj || deriva < -edgesync.MaxDerivaReloj
	salud, err := json.Marshal(map[string]any{
		"discoLibreMb": hb.DiscoLibreMB, "baseMb": hb.BaseMB, "outboxPendientes": hb.OutboxPendientes,
		"outboxAntiguedadSeg": hb.OutboxAntiguedad, "impresoras": hb.Impresoras, "derivaSegundos": res.DerivaSegundos,
		"alertaReloj": res.AlertaReloj, "arranqueAt": hb.ArranqueAt,
	})
	if err != nil {
		return res, err
	}
	err = s.DB.InTenant(ctx, n.TenantID, func(tx db.Tx) error {
		_, err := tx.Exec(ctx, `UPDATE nodos SET ultimo_heartbeat_at = $2, heartbeat = $3, version_software = $4 WHERE id = $1`,
			n.ID, now, salud, truncar(hb.Version, 40))
		return err
	})
	return res, err
}

func truncar(s string, n int) string {
	if r := []rune(strings.TrimSpace(s)); len(r) > n {
		return string(r[:n])
	}
	return strings.TrimSpace(s)
}
