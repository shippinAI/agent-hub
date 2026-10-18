package httpapi

import "net/http"

// Handler is a JSON handler that gets the authenticated principal.
type Handler = handler

// Authed wraps h with authentication, the CSRF guard, the machine-token refusal and the error envelope.
func (s *Server) Authed(h Handler) http.HandlerFunc { return s.authed(h) }

// Decode reads a JSON body and returns the request id (Idempotency-Key header or request_id field).
func Decode(r *http.Request, v any) (string, error) { return decode(r, v) }

// JSON writes v with status code.
func JSON(w http.ResponseWriter, code int, v any) { js(w, code, v) }

// WriteErr writes an error envelope.
func WriteErr(w http.ResponseWriter, err error) { writeErr(w, err) }
