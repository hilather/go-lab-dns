package rest

import (
	"encoding/json"
	"net/http"

	"github.com/hilather/go-lab-dns/internal/capabilities"
	"github.com/hilather/go-lab-dns/internal/domainerr"
)

// problemDoc is capabilities.Problem plus the docs/06 expectedRevision extension.
type problemDoc struct {
	capabilities.Problem
	ExpectedRevision string `json:"expectedRevision,omitempty"`
}

func (s *Server) writeProblem(w http.ResponseWriter, r *http.Request, instance string, err error) {
	p := capabilities.ProblemFrom(err, instance)
	s.writeProblemDoc(w, r, problemDoc{Problem: p})
}

func (s *Server) writeRevisionProblem(w http.ResponseWriter, r *http.Request, instance string, err error, expected string) {
	p := capabilities.ProblemFrom(err, instance)
	s.writeProblemDoc(w, r, problemDoc{Problem: p, ExpectedRevision: expected})
}

func (s *Server) writeProblemDoc(w http.ResponseWriter, r *http.Request, p problemDoc) {
	if p.Status == http.StatusUnauthorized {
		w.Header().Set("WWW-Authenticate", `Bearer realm="labdns"`)
	}
	body, err := json.Marshal(p)
	if err != nil {
		http.Error(w, `{"type":"urn:labdns:error:internal-error","title":"Internal error","status":500,"code":"internal_error"}`, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", capabilities.ProblemContentType)
	w.WriteHeader(p.Status)
	_, _ = w.Write(body)
	_ = r
}

func asDomain(err error) error {
	if err == nil {
		return nil
	}
	if _, ok := domainerr.As(err); ok {
		return err
	}
	return domainerr.Internal("internal error")
}
