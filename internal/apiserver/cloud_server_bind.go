// Package apiserver — dispatchers that bridge the Server's mux to
// CloudService methods. Kept separate from cloud_handlers.go so the
// CloudService type stays reusable outside the main Server.
package apiserver

import (
	"net/http"
)

// cloudGuard wraps a handler so it returns 503 when the Cloud backend
// is disabled (STOCKYARD_CLOUD_ENABLED != 1, which leaves s.desktopCloud
// nil). The 503 body is JSON so the desktop client can show a friendly
// "Cloud coming soon" message.
func (s *Server) cloudGuard(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.desktopCloud == nil {
			writeJSON(w, 503, map[string]string{
				"error": "Cloud backend is not enabled yet",
				"hint":  "set STOCKYARD_CLOUD_ENABLED=1 to enable",
			})
			return
		}
		h(w, r)
	}
}

// Dispatcher methods. Each one is a tiny forwarder so that server.go
// route registration reads cleanly and CloudService doesn't need to
// know about the Server.

func (s *Server) cloudHandlerLoginRequest(w http.ResponseWriter, r *http.Request) {
	s.desktopCloud.HandleLoginRequest(w, r)
}
func (s *Server) cloudHandlerLoginVerify(w http.ResponseWriter, r *http.Request) {
	s.desktopCloud.HandleLoginVerify(w, r)
}
func (s *Server) cloudHandlerLogout(w http.ResponseWriter, r *http.Request) {
	s.desktopCloud.HandleLogout(w, r)
}
func (s *Server) cloudHandlerMe(w http.ResponseWriter, r *http.Request) {
	s.desktopCloud.HandleMe(w, r)
}
func (s *Server) cloudHandlerBackupUpload(w http.ResponseWriter, r *http.Request) {
	s.desktopCloud.HandleBackupUpload(w, r)
}
func (s *Server) cloudHandlerBackupLatest(w http.ResponseWriter, r *http.Request) {
	s.desktopCloud.HandleBackupLatest(w, r)
}
func (s *Server) cloudHandlerBackupList(w http.ResponseWriter, r *http.Request) {
	s.desktopCloud.HandleBackupList(w, r)
}
func (s *Server) cloudHandlerBackupByID(w http.ResponseWriter, r *http.Request) {
	s.desktopCloud.HandleBackupByID(w, r)
}
func (s *Server) cloudHandlerSitesList(w http.ResponseWriter, r *http.Request) {
	s.desktopCloud.HandleSitesList(w, r)
}
func (s *Server) cloudHandlerSitesCreate(w http.ResponseWriter, r *http.Request) {
	s.desktopCloud.HandleSitesCreate(w, r)
}
