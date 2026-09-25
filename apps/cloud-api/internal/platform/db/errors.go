package db

import (
	"errors"

	"github.com/jackc/pgx/v5"

	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/platform/apperr"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/ids"
)

// NotFound convierte «sin filas» (inexistente o de otro tenant, oculto por RLS) en 404.
func NotFound(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return apperr.ErrNotFound
	}
	return err
}

// Translate convierte violaciones de la base en errores claros para el usuario:
// un índice único → 409 con el campo a corregir; una FK rota → 422.
func Translate(err error, unique, campo, msg string) error {
	switch {
	case err == nil:
		return nil
	case unique != "" && IsUniqueViolation(err, unique):
		return &apperr.Error{Kind: apperr.Conflict, Code: "DUPLICADO", Message: msg, Fields: []apperr.FieldError{{Campo: campo, Mensaje: msg}}}
	case IsForeignKeyViolation(err):
		return apperr.New(apperr.Invalid, "REFERENCIA_INVALIDA", "Uno de los elementos elegidos ya no existe. Recarga la página e intenta de nuevo.")
	}
	return err
}

// IDOrNew usa el UUID v7 que manda el cliente (creación idempotente) o genera uno.
func IDOrNew(id *ids.ID) ids.ID {
	if id != nil && *id != ids.Nil {
		return *id
	}
	return ids.New()
}
