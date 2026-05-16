package api

import "net/http"

// NewRouter wires all API routes to their handlers and returns a ready http.Handler.
// apiKey is required for mutating routes; pass an empty string only in tests that
// stub the handler directly (the middleware short-circuits on empty == empty).
func NewRouter(h *Handler, apiKey string) http.Handler {
	mux := http.NewServeMux()
	auth := RequireAPIKey(apiKey)

	// Read-only — no auth required.
	mux.HandleFunc("GET /api/v1/changes", h.ListChanges)
	mux.HandleFunc("GET /api/v1/changes/{id}", h.GetChange)

	// Mutating — API key required.
	mux.Handle("POST /api/v1/changes", auth(http.HandlerFunc(h.CreateChange)))
	mux.Handle("POST /api/v1/changes/{id}/approve", auth(http.HandlerFunc(h.ApproveChange)))
	mux.Handle("POST /api/v1/changes/{id}/reject", auth(http.HandlerFunc(h.RejectChange)))
	mux.Handle("DELETE /api/v1/demo/reset", auth(http.HandlerFunc(h.DemoReset)))

	return RequestLogger(RecoveryMiddleware(RequestID(mux)))
}
