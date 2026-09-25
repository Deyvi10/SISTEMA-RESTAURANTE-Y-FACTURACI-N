package auth

import (
	"context"
	"crypto/ed25519"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/platform/apperr"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/platform/db"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/platform/httpx"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/ids"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/nodoauth"
)

// Nodo es el Nodo Local autenticado de una petición (rutas con Access.Nodo).
type Nodo struct {
	ID, TenantID, LocalID ids.ID
}

const nodoKey ctxKey = 100

// MustNodo devuelve el nodo autenticado; solo en rutas con Access.Nodo.
func MustNodo(ctx context.Context) Nodo {
	n, ok := ctx.Value(nodoKey).(Nodo)
	if !ok {
		panic("auth: handler de nodo sin nodo autenticado")
	}
	return n
}

var errNodo = apperr.New(apperr.Unauthorized, "NODO_NO_AUTORIZADO", "El Nodo Local no está activado o fue revocado. Actívalo con un código nuevo desde el backoffice.")

// guardNodo verifica el JWT firmado por el nodo y que siga ACTIVO en cada petición:
// revocar un nodo (o activar otro para el mismo local) le corta el acceso al instante.
func guardNodo(d *db.DB, now func() time.Time, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		raw, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
		if !ok {
			httpx.Error(w, r, errNodo)
			return
		}
		var n Nodo
		var estado string
		var dbErr error // una caída de la base es 500, no «nodo revocado»
		lookup := func(id ids.ID) (ed25519.PublicKey, error) {
			var pub []byte
			err := d.Global(ctx, func(tx db.Tx) error {
				return tx.QueryRow(ctx, `SELECT tenant_id, local_id, llave_publica, estado FROM auth_buscar_nodo($1)`, id).
					Scan(&n.TenantID, &n.LocalID, &pub, &estado)
			})
			if err != nil && !errors.Is(err, pgx.ErrNoRows) {
				dbErr = err
			}
			return pub, err
		}
		id, err := nodoauth.Verify(raw, lookup, now())
		switch {
		case dbErr != nil:
			httpx.Error(w, r, dbErr)
			return
		case err != nil || estado != "ACTIVO":
			httpx.Error(w, r, errNodo)
			return
		}
		n.ID = id
		next.ServeHTTP(w, r.WithContext(context.WithValue(ctx, nodoKey, n)))
	})
}
