// Package api serves a JSON view of the tracked projects, releases, images,
// digests and feeds. It reads and writes the same SQLite store the CLI and the
// poll loop use, through the service layer.
package api

import (
	"net/http"

	"relwatch/internal/service"
	"relwatch/internal/update"
)

// Server is the HTTP API over the tracked projects, images and feeds.
type Server struct {
	svc     *service.Service
	version string
	token   string
	update  func() *update.Status
}

// New builds the API handler. version is reported by GET /api/v1/status. token,
// when non-empty, is required as `Authorization: Bearer <token>` on every request.
func New(svc *service.Service, version, token string) *Server {
	return &Server{svc: svc, version: version, token: token}
}

// SetUpdateStatus wires an optional provider of the latest update-check result,
// surfaced as `update` by GET /api/v1/status. When nil the field is omitted.
func (s *Server) SetUpdateStatus(f func() *update.Status) {
	s.update = f
}

// Handler returns the routed, authenticated API handler.
func (s *Server) Handler() http.Handler {
	return s.routes()
}
