// Package apperr define los errores de dominio. Los servicios devuelven estos errores;
// la capa HTTP los traduce a códigos y mensajes en un solo lugar (docs/11 §2.6).
// Los mensajes son para el usuario final: lenguaje claro y qué hacer (RNF-33).
package apperr

import (
	"errors"
	"fmt"
)

type Kind int

const (
	Invalid      Kind = iota + 1 // 422: datos que no cumplen una regla
	NotFound                     // 404
	Conflict                     // 409: duplicado o estado incompatible
	Unauthorized                 // 401
	Forbidden                    // 403
	TooMany                      // 429: demasiados intentos
)

// FieldError señala el campo exacto que hay que corregir.
type FieldError struct {
	Campo   string `json:"campo"`
	Mensaje string `json:"mensaje"`
}

type Error struct {
	Kind    Kind
	Code    string // estable, para el cliente: "RUC_INVALIDO", "PIN_REPETIDO"…
	Message string
	Fields  []FieldError
}

func (e *Error) Error() string { return fmt.Sprintf("%s: %s", e.Code, e.Message) }

func New(k Kind, code, msg string) *Error { return &Error{Kind: k, Code: code, Message: msg} }

// Validation acumula errores de campos y devuelve nil si no hubo ninguno.
type Validation struct{ fields []FieldError }

func (v *Validation) Add(campo, mensaje string) {
	v.fields = append(v.fields, FieldError{campo, mensaje})
}

func (v *Validation) Check(ok bool, campo, mensaje string) {
	if !ok {
		v.Add(campo, mensaje)
	}
}

func (v *Validation) Err() error {
	if len(v.fields) == 0 {
		return nil
	}
	msg := v.fields[0].Mensaje
	if len(v.fields) > 1 {
		msg = fmt.Sprintf("Revisa %d campos.", len(v.fields))
	}
	return &Error{Kind: Invalid, Code: "DATOS_INVALIDOS", Message: msg, Fields: v.fields}
}

func As(err error) (*Error, bool) {
	var e *Error
	ok := errors.As(err, &e)
	return e, ok
}

var (
	ErrNotFound  = New(NotFound, "NO_ENCONTRADO", "No encontramos lo que buscas. Puede que se haya eliminado.")
	ErrForbidden = New(Forbidden, "SIN_PERMISO", "No tienes permiso para hacer esto. Pide a un administrador que te lo habilite.")
)
