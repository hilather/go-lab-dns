package auth

import (
	"testing"

	"github.com/hilather/go-lab-dns/internal/domainerr"
	"github.com/hilather/go-lab-dns/internal/model"
)

func TestAuthorizeTargetUIRequiresAdmin(t *testing.T) {
	op := model.Operation{
		Op:     model.OpUpdate,
		Target: model.Target{Kind: model.TargetUI},
		Value:  []byte(`{"enabled":false}`),
	}
	editor := Actor{ID: "e", Class: ClassToken, Role: RoleDNSEditor}
	if err := AuthorizeChange(editor, []model.Operation{op}, &model.State{}); err == nil {
		t.Fatal("editor authorized TargetUI")
	} else if de, ok := domainerr.As(err); !ok || de.Code != domainerr.CodeForbidden {
		t.Fatalf("want forbidden, got %v", err)
	}
	admin := Actor{ID: "a", Class: ClassToken, Role: RoleAdministrator}
	if err := AuthorizeChange(admin, []model.Operation{op}, &model.State{}); err != nil {
		t.Fatal(err)
	}
	need := RequiredPermissions([]model.Operation{op}, &model.State{})
	found := false
	for _, s := range need {
		if s == ScopeDNSAdmin {
			found = true
		}
	}
	if !found {
		t.Fatalf("RequiredPermissions=%v, want %s", need, ScopeDNSAdmin)
	}
}
