package rest

import (
	"net/http"
	"testing"
)

func TestOptionalMutationBodyRejectsTrailingJSON(t *testing.T) {
	for _, path := range []string{"/v1/state:reset", "/v1/cache:flush", "/v1/chaos:emergency-disable"} {
		for _, body := range []string{`{} {}`, `{} ]`, `{} null`} {
			t.Run(path+body, func(t *testing.T) {
				s, svc := newTestServer(t)
				before := svc.Store().Load()
				rec := doLoopback(t, s.Handler(), http.MethodPost, path, body)
				requireProblem(t, rec, http.StatusBadRequest, "validation_failed")
				if svc.Store().Load() != before {
					t.Fatal("invalid JSON mutated active snapshot")
				}
			})
		}
	}
}
