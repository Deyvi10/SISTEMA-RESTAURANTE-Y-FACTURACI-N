package app

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/edge-node/internal/nube"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/edge-node/internal/store"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/ids"
)

// Identidad es el nodo activado: a qué restaurante y local pertenece y su llave.
type Identidad struct {
	NodoID, TenantID, LocalID    ids.ID
	NubeURL                      string
	Llave                        ed25519.PrivateKey
	NombreLocal, NombreComercial string
	ActivadoAt                   time.Time
	RevocadoAt                   *time.Time
}

func (i *Identidad) Activo() bool { return i != nil && i.RevocadoAt == nil }

// Problema es un error para mostrar al usuario en la LAN (RFC 9457).
type Problema struct {
	Status       int
	Code, Detail string
}

func (p *Problema) Error() string { return p.Code + ": " + p.Detail }

func problema(status int, code, detail string) *Problema {
	return &Problema{Status: status, Code: code, Detail: detail}
}

// Identidad lee la activación guardada (nil si el nodo nunca se activó).
func (a *App) Identidad(ctx context.Context) (*Identidad, error) {
	var i Identidad
	var nodo, tenant, local, activado string
	var revocado sql.NullString
	err := a.Store.Read().QueryRowContext(ctx, `SELECT nodo_id, tenant_id, local_id, nube_url, llave_privada, nombre_local, nombre_comercial, activado_at, revocado_at FROM nodo WHERE id = 1`).
		Scan(&nodo, &tenant, &local, &i.NubeURL, &i.Llave, &i.NombreLocal, &i.NombreComercial, &activado, &revocado)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var e1, e2, e3, e4 error
	i.NodoID, e1 = ids.Parse(nodo)
	i.TenantID, e2 = ids.Parse(tenant)
	i.LocalID, e3 = ids.Parse(local)
	i.ActivadoAt, e4 = time.Parse(time.RFC3339Nano, activado)
	if revocado.Valid {
		t, err := time.Parse(time.RFC3339Nano, revocado.String)
		if err != nil {
			return nil, err
		}
		i.RevocadoAt = &t
	}
	return &i, errors.Join(e1, e2, e3, e4)
}

func normalizarCodigo(c string) (string, bool) {
	var b strings.Builder
	for _, r := range strings.ToUpper(c) {
		if r == '-' || r == ' ' {
			continue
		}
		if (r < 'A' || r > 'Z') && (r < '0' || r > '9') {
			return "", false
		}
		b.WriteRune(r)
	}
	return b.String(), b.Len() == 8
}

type activado struct {
	NodoID          ids.ID `json:"nodoId"`
	TenantID        ids.ID `json:"tenantId"`
	LocalID         ids.ID `json:"localId"`
	NombreLocal     string `json:"nombreLocal"`
	NombreComercial string `json:"nombreComercial"`
}

// Activar canjea el código que el dueño generó en el backoffice (F2-02). Si la respuesta
// se pierde, reintentar con el mismo código usa la misma identidad y la nube responde igual.
func (a *App) Activar(ctx context.Context, codigo string) (*Identidad, error) {
	actual, err := a.Identidad(ctx)
	if err != nil {
		return nil, err
	}
	if actual.Activo() {
		return nil, problema(http.StatusConflict, "YA_ACTIVADO", "Este nodo ya está activado para "+actual.NombreComercial+". Para cambiarlo, revócalo primero en el backoffice.")
	}
	norm, ok := normalizarCodigo(codigo)
	if !ok {
		return nil, problema(http.StatusUnprocessableEntity, "CODIGO_INVALIDO", "El código tiene 8 letras y números, como ABCD-EFGH.")
	}
	h := sha256.Sum256([]byte(norm))
	hash := hex.EncodeToString(h[:])

	// Identidad pendiente: la misma para el mismo código.
	var nodoID ids.ID
	var llave ed25519.PrivateKey
	err = a.Store.Write(ctx, func(tx *store.Tx) error {
		var id, ch string
		var priv []byte
		err := tx.QueryRowContext(ctx, `SELECT nodo_id, codigo_hash, llave_privada FROM identidad_pendiente WHERE id = 1`).Scan(&id, &ch, &priv)
		if err == nil && ch == hash {
			nodoID, err = ids.Parse(id)
			llave = priv
			return err
		}
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		_, llave, err = ed25519.GenerateKey(nil)
		if err != nil {
			return err
		}
		nodoID = ids.New()
		_, err = tx.ExecContext(ctx, `INSERT INTO identidad_pendiente (id, codigo_hash, nodo_id, llave_privada, created_at) VALUES (1, ?, ?, ?, ?)
			ON CONFLICT (id) DO UPDATE SET codigo_hash = excluded.codigo_hash, nodo_id = excluded.nodo_id, llave_privada = excluded.llave_privada, created_at = excluded.created_at`,
			hash, nodoID.String(), []byte(llave), a.now())
		return err
	})
	if err != nil {
		return nil, err
	}

	equipo, _ := os.Hostname()
	cli := &nube.Client{BaseURL: a.Cfg.NubeURL, HTTP: a.httpNube}
	var res activado
	cctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	err = cli.Do(cctx, http.MethodPost, "/v1/nodos/activar", map[string]any{
		"codigo": norm, "nodoId": nodoID, "llavePublica": base64.StdEncoding.EncodeToString(llave.Public().(ed25519.PublicKey)),
		"version": Version, "nombreEquipo": equipo,
	}, &res, false)
	var ne *nube.Error
	switch {
	case errors.As(err, &ne):
		return nil, problema(ne.Status, ne.Code, ne.Detail)
	case err != nil:
		a.Log.Warn("activación: sin conexión con la nube", "err", err)
		return nil, problema(http.StatusBadGateway, "SIN_NUBE", "No hay conexión con la nube. Revisa que esta PC tenga internet e intenta de nuevo.")
	case res.NodoID != nodoID:
		return nil, fmt.Errorf("activación: la nube respondió otro nodo")
	}

	cambioDeLocal := actual != nil && (actual.TenantID != res.TenantID || actual.LocalID != res.LocalID)
	err = a.Store.Write(ctx, func(tx *store.Tx) error {
		if cambioDeLocal {
			// Otro restaurante u otro local: la réplica anterior no sirve. Lo operativo
			// pendiente de sincronizar NUNCA se borra (outbox intacto).
			if err := a.limpiarReplica(ctx, tx); err != nil {
				return err
			}
		}
		stmts := []struct {
			q    string
			args []any
		}{
			{`DELETE FROM nodo`, nil},
			{`INSERT INTO nodo (id, nodo_id, tenant_id, local_id, nube_url, llave_privada, nombre_local, nombre_comercial, activado_at) VALUES (1, ?, ?, ?, ?, ?, ?, ?, ?)`,
				[]any{nodoID.String(), res.TenantID.String(), res.LocalID.String(), a.Cfg.NubeURL, []byte(llave), res.NombreLocal, res.NombreComercial, a.now()}},
			{`DELETE FROM identidad_pendiente`, nil},
			{`INSERT INTO auditoria (id, accion, entidad, entidad_id, detalle, created_at) VALUES (?, 'NODO_ACTIVADO', 'nodo', ?, json_object('localId', ?), ?)`,
				[]any{ids.New().String(), nodoID.String(), res.LocalID.String(), a.now()}},
		}
		for _, s := range stmts {
			if _, err := tx.ExecContext(ctx, s.q, s.args...); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	a.Log.Info("nodo activado", "nodo", nodoID, "tenant", res.TenantID, "local", res.LocalID)
	id, err := a.Identidad(ctx)
	if err != nil {
		return nil, err
	}
	a.iniciarSync(id)
	return id, nil
}

// marcarRevocado se llama cuando la nube rechaza la identidad del nodo.
func (a *App) marcarRevocado(ctx context.Context) error {
	return a.Store.Write(ctx, func(tx *store.Tx) error {
		_, err := tx.ExecContext(ctx, `UPDATE nodo SET revocado_at = ? WHERE id = 1 AND revocado_at IS NULL`, a.now())
		return err
	})
}

// limpiarReplica borra los datos que vienen de la nube y sus cursores.
func (a *App) limpiarReplica(ctx context.Context, tx *store.Tx) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM inbox_cursores`); err != nil {
		return err
	}
	return a.replica.Vaciar(ctx, tx)
}

func (a *App) now() string { return a.Clock.Now().UTC().Format(time.RFC3339Nano) }
