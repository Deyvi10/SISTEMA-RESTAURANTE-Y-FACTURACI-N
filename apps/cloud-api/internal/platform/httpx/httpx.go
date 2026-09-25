// Package httpx reúne lo transversal de la API: JSON, errores RFC 9457 y middlewares.
package httpx

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/platform/apperr"
)

const maxBody = 1 << 20

// Problem es un error RFC 9457 con extensiones del proyecto.
type Problem struct {
	Type    string              `json:"type"`
	Title   string              `json:"title"`
	Status  int                 `json:"status"`
	Detail  string              `json:"detail"`
	Code    string              `json:"code"`
	Errors  []apperr.FieldError `json:"errors,omitempty"`
	TraceID string              `json:"traceId,omitempty"`
}

// JSON responde un cuerpo JSON.
func JSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if v != nil {
		_ = json.NewEncoder(w).Encode(v)
	}
}

// Decode lee un JSON estricto: rechaza campos desconocidos y cuerpos de más de 1 MB.
func Decode(w http.ResponseWriter, r *http.Request, v any) error {
	return decode(w, r, v, maxBody, true)
}

// DecodeNodo lee lo que envía el Nodo Local: tolera campos nuevos (un nodo con una versión
// más reciente sigue funcionando, docs/03 §5.4) y admite lotes de sincronización de hasta 2 MB.
func DecodeNodo(w http.ResponseWriter, r *http.Request, v any) error {
	return decode(w, r, v, 2*maxBody, false)
}

func decode(w http.ResponseWriter, r *http.Request, v any, limit int64, strict bool) error {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, limit))
	if strict {
		dec.DisallowUnknownFields()
	}
	if err := dec.Decode(v); err != nil {
		var syn *json.SyntaxError
		var typ *json.UnmarshalTypeError
		var tooBig *http.MaxBytesError
		switch {
		case errors.As(err, &typ):
			return &apperr.Error{
				Kind: apperr.Invalid, Code: "JSON_INVALIDO", Message: fmt.Sprintf("El campo %q tiene un tipo incorrecto.", typ.Field),
				Fields: []apperr.FieldError{{Campo: typ.Field, Mensaje: "Tipo incorrecto."}},
			}
		case errors.As(err, &tooBig):
			return apperr.New(apperr.Invalid, "CUERPO_GRANDE", fmt.Sprintf("La solicitud supera %d MB.", limit>>20))
		case strings.HasPrefix(err.Error(), "json: unknown field"):
			return apperr.New(apperr.Invalid, "CAMPO_DESCONOCIDO", "La solicitud trae un campo que no existe: "+strings.TrimPrefix(err.Error(), "json: unknown field "))
		case errors.As(err, &syn), errors.Is(err, io.EOF), errors.Is(err, io.ErrUnexpectedEOF):
			return apperr.New(apperr.Invalid, "JSON_INVALIDO", "El cuerpo de la solicitud no es JSON válido.")
		default:
			return apperr.New(apperr.Invalid, "JSON_INVALIDO", "No se pudo leer la solicitud.")
		}
	}
	return nil
}

var status = map[apperr.Kind]int{
	apperr.Invalid: http.StatusUnprocessableEntity, apperr.NotFound: http.StatusNotFound,
	apperr.Conflict: http.StatusConflict, apperr.Unauthorized: http.StatusUnauthorized,
	apperr.Forbidden: http.StatusForbidden, apperr.TooMany: http.StatusTooManyRequests,
}

// Error traduce cualquier error a un Problem. Los errores inesperados se registran con
// su detalle pero al cliente solo le llega un mensaje genérico (nunca SQL ni trazas).
func Error(w http.ResponseWriter, r *http.Request, err error) {
	p := Problem{Type: "about:blank", TraceID: RequestID(r.Context())}
	if ae, ok := apperr.As(err); ok {
		p.Status, p.Code, p.Detail, p.Errors = status[ae.Kind], ae.Code, ae.Message, ae.Fields
	} else {
		slog.ErrorContext(r.Context(), "error inesperado", "err", err, "ruta", r.Pattern)
		p.Status, p.Code = http.StatusInternalServerError, "ERROR_INTERNO"
		p.Detail = "Algo salió mal de nuestro lado. Intenta de nuevo en un momento."
	}
	p.Title = http.StatusText(p.Status)
	w.Header().Set("Content-Type", "application/problem+json; charset=utf-8")
	w.WriteHeader(p.Status)
	_ = json.NewEncoder(w).Encode(p)
}
