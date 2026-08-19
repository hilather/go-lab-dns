package rest

import (
	"net/http"
	"time"

	"github.com/hilather/go-lab-dns/internal/audit"
	"github.com/hilather/go-lab-dns/internal/auth"
	"github.com/hilather/go-lab-dns/internal/domainerr"
	"github.com/hilather/go-lab-dns/internal/observability"
)

func (s *Server) handleSessionCreate(w http.ResponseWriter, r *http.Request, instance string, actor auth.Actor) {
	var (
		sess *auth.Session
		err  error
	)
	if _, ok := auth.BearerToken(r.Header.Get("Authorization")); ok {
		sess, err = s.sessions.Create(actor)
	} else if c, cerr := r.Cookie(auth.CookieName); cerr == nil && c.Value != "" {
		if _, ok := s.sessions.Lookup(c.Value); ok {
			sess, err = s.sessions.Rotate(c.Value)
		} else {
			s.writeProblem(w, r, instance, domainerr.Unauthenticated("authentication required"))
			return
		}
	} else {
		sess, err = s.sessions.Create(actor)
	}
	if err != nil {
		s.writeProblem(w, r, instance, asDomain(err))
		return
	}
	setSessionCookie(w, r, sess.ID, 0)
	w.Header().Set("Cache-Control", "no-store")
	s.sessionAudit(r, sess.Actor, audit.ResultOK, "")
	s.sessionLog(r, sess.Actor, "ok")
	s.writeJSON(w, http.StatusOK, sessionResponse{
		CSRF:  sess.CSRF,
		Actor: sessionActorJSONFrom(sess.Actor),
	})
}

func (s *Server) handleSessionGet(w http.ResponseWriter, r *http.Request, instance string) {
	sess, err := s.liveSession(r)
	if err != nil {
		s.writeProblem(w, r, instance, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	s.writeJSON(w, http.StatusOK, sessionResponse{
		CSRF:  sess.CSRF,
		Actor: sessionActorJSONFrom(sess.Actor),
	})
}

func (s *Server) handleSessionDelete(w http.ResponseWriter, r *http.Request, instance string) {
	sess, err := s.liveSession(r)
	if err != nil {
		s.writeProblem(w, r, instance, err)
		return
	}
	s.sessions.Delete(sess.ID)
	setSessionCookie(w, r, "", -1)
	w.Header().Set("Cache-Control", "no-store")
	s.sessionAudit(r, sess.Actor, audit.ResultOK, "")
	s.sessionLog(r, sess.Actor, "ok")
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) liveSession(r *http.Request) (*auth.Session, error) {
	c, err := r.Cookie(auth.CookieName)
	if err != nil || c.Value == "" {
		return nil, domainerr.Unauthenticated("authentication required")
	}
	sess, ok := s.sessions.Lookup(c.Value)
	if !ok {
		return nil, domainerr.Unauthenticated("authentication required")
	}
	return sess, nil
}

func setSessionCookie(w http.ResponseWriter, r *http.Request, value string, maxAge int) {
	http.SetCookie(w, &http.Cookie{
		Name:     auth.CookieName,
		Value:    value,
		Path:     "/",
		MaxAge:   maxAge,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   r.TLS != nil,
	})
}

func (s *Server) sessionAudit(r *http.Request, actor auth.Actor, result, code string) {
	s.emitAudit(r.Context(), audit.Event{
		Time:       time.Now().UTC(),
		ActorID:    actor.ID,
		ActorClass: auth.ClassUISession,
		Transport:  "rest",
		Capability: "session",
		Result:     result,
		ErrorCode:  code,
	})
}

func (s *Server) sessionLog(r *http.Request, actor auth.Actor, result string) {
	if s.logger == nil {
		return
	}
	s.logger.Log(observability.Record{
		Event:      observability.EventUISession,
		Component:  "rest",
		RequestID:  observability.RequestIDFrom(r.Context()),
		Capability: "session",
		Transport:  "rest",
		Result:     result,
		ActorID:    actor.ID,
	})
}

func sessionActorJSONFrom(a auth.Actor) sessionActorJSON {
	return sessionActorJSON{
		ID:     a.ID,
		Class:  a.Class,
		Role:   a.Role,
		Scopes: a.EffectiveScopes(),
		Groups: append([]string(nil), a.Groups...),
	}
}
