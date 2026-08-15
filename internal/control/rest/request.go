package rest

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/hilather/go-lab-dns/internal/app"
	"github.com/hilather/go-lab-dns/internal/domainerr"
)

func (s *Server) decodeJSON(w http.ResponseWriter, r *http.Request, instance string, dst any) bool {
	if err := s.checkJSONContentType(r); err != nil {
		s.writeProblem(w, r, instance, err)
		return false
	}
	r.Body = http.MaxBytesReader(w, r.Body, s.maxBody)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		s.writeProblem(w, r, instance, decodeError(err))
		return false
	}
	if err := dec.Decode(&struct{}{}); err != io.EOF {
		s.writeProblem(w, r, instance, domainerr.ValidationFailed("request body must contain a single JSON value",
			domainerr.FieldViolation{Path: "", Code: "invalid_value", Message: "trailing JSON is not allowed"}))
		return false
	}
	return true
}

func (s *Server) decodeJSONOptional(w http.ResponseWriter, r *http.Request, instance string, dst any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, s.maxBody)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		s.writeProblem(w, r, instance, decodeError(err))
		return false
	}
	if len(bytesTrimSpace(body)) == 0 {
		return true
	}
	if err := s.checkJSONContentType(r); err != nil {
		s.writeProblem(w, r, instance, err)
		return false
	}
	dec := json.NewDecoder(strings.NewReader(string(body)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		s.writeProblem(w, r, instance, decodeError(err))
		return false
	}
	return true
}

func (s *Server) checkJSONContentType(r *http.Request) error {
	ct := r.Header.Get("Content-Type")
	if ct == "" {
		return domainerr.ValidationFailed("content type is required",
			domainerr.FieldViolation{Path: "content-type", Code: "required", Message: "POST bodies must be application/json"})
	}
	media := strings.TrimSpace(strings.Split(ct, ";")[0])
	if !strings.EqualFold(media, "application/json") {
		return domainerr.ValidationFailed("unsupported content type",
			domainerr.FieldViolation{Path: "content-type", Code: "invalid_value", Message: "expected application/json"})
	}
	return nil
}

func decodeError(err error) error {
	if err == nil {
		return nil
	}
	var maxErr *http.MaxBytesError
	if errors.As(err, &maxErr) {
		return domainerr.ValidationFailed("request body too large",
			domainerr.FieldViolation{Path: "", Code: "document_too_large", Message: "request body exceeds the management limit"})
	}
	msg := "invalid JSON"
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
		msg = "request body is required"
	}
	return domainerr.ValidationFailed(msg,
		domainerr.FieldViolation{Path: "", Code: "invalid_value", Message: "request body is not valid JSON"})
}

func bytesTrimSpace(b []byte) []byte {
	i, j := 0, len(b)
	for i < j && (b[i] == ' ' || b[i] == '\n' || b[i] == '\r' || b[i] == '\t') {
		i++
	}
	for j > i && (b[j-1] == ' ' || b[j-1] == '\n' || b[j-1] == '\r' || b[j-1] == '\t') {
		j--
	}
	return b[i:j]
}

func pageFromQuery(r *http.Request) (app.Page, error) {
	p := app.Page{Cursor: r.URL.Query().Get("cursor")}
	if raw := r.URL.Query().Get("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 0 {
			return p, domainerr.ValidationFailed("invalid limit",
				domainerr.FieldViolation{Path: "limit", Code: "invalid_value", Message: "limit must be a non-negative integer"})
		}
		p.Limit = n
	}
	return p, nil
}

func expectedRevision(r *http.Request, body string) string {
	if body != "" {
		return body
	}
	if v := strings.Trim(r.Header.Get(headerIfMatch), `"`); v != "" && !strings.EqualFold(v, "*") {
		return v
	}
	return strings.TrimSpace(r.Header.Get(headerExpected))
}

func idempotencyKey(r *http.Request, body string) string {
	if body != "" {
		return body
	}
	return strings.TrimSpace(r.Header.Get(headerIdempotency))
}
