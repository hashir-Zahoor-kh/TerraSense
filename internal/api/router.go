package api

import "net/http"

// NewRouter wires all API routes to their handlers and returns a ready http.Handler.
func NewRouter(h *Handler) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("POST /api/v1/changes", h.CreateChange)
	mux.HandleFunc("GET /api/v1/changes", h.ListChanges)
	mux.HandleFunc("GET /api/v1/changes/{id}", h.GetChange)
	mux.HandleFunc("POST /api/v1/changes/{id}/approve", h.ApproveChange)
	mux.HandleFunc("POST /api/v1/changes/{id}/reject", h.RejectChange)
	mux.HandleFunc("DELETE /api/v1/demo/reset", h.DemoReset)

	return RequestLogger(RecoveryMiddleware(RequestID(mux)))
}
