package deploytest

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/hilather/go-lab-dns/internal/app"
	"github.com/hilather/go-lab-dns/internal/auth"
	"github.com/hilather/go-lab-dns/internal/compiler"
	"github.com/hilather/go-lab-dns/internal/config"
	"github.com/hilather/go-lab-dns/internal/deploypolicy"
	"github.com/hilather/go-lab-dns/internal/snapshot"
)

func TestEnvironmentsValidateAndVerify(t *testing.T) {
	root := repoRoot(t)
	bin := buildLabDNS(t, root)
	for _, env := range []string{"main-lab", "test-lab"} {
		dir := filepath.Join(root, "examples", "labdns-deploy", "environments", env)
		args := []string{"verify",
			"--config", filepath.Join(dir, "dns.yaml"),
			"--probes", filepath.Join(dir, "probes.yaml"),
			"--policies", filepath.Join(root, "examples", "labdns-deploy", "policies"),
			"--image-env", filepath.Join(dir, "image.env"),
		}
		kust := filepath.Join(dir, "k8s", "kustomization.yaml")
		if _, err := os.Stat(kust); err == nil {
			args = append(args, "--kustomize", kust)
		}
		cmd := exec.Command(bin, args...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("%s verify: %v\n%s", env, err, out)
		}
		text := string(out)
		for _, want := range []string{
			"ok policies", "ok image digest pin", "ok exact-ns1", "ok wildcard-tools",
			"ok authoritative-miss", "ok overlay-hit", "ok unknown-client-local",
			"ok unknown-client-forward-only", "ok chaos-simulation",
		} {
			if !strings.Contains(text, want) {
				t.Errorf("%s missing %q\n%s", env, want, text)
			}
		}
		if env == "main-lab" && !strings.Contains(text, "skip live-exact-ns1") {
			t.Errorf("%s should skip live probe without --server\n%s", env, text)
		}
	}
}

func TestPolicyNegativesAndUnpinnedImage(t *testing.T) {
	root := repoRoot(t)
	bin := buildLabDNS(t, root)
	deploy := filepath.Join(root, "examples", "labdns-deploy")
	probes := filepath.Join(deploy, "environments", "main-lab", "probes.yaml")
	policies := filepath.Join(deploy, "policies")
	cases := []struct {
		name string
		args []string
		want string
	}{
		{"latest", []string{"verify", "--config", filepath.Join(deploy, "environments", "main-lab", "dns.yaml"), "--probes", probes, "--image", "ghcr.io/hilather/labdns:latest"}, "digest"},
		{"image-env", []string{"verify", "--config", filepath.Join(deploy, "environments", "main-lab", "dns.yaml"), "--probes", probes, "--image-env", filepath.Join(deploy, "testdata", "invalid", "unpinned.image.env")}, "digest"},
		{"broad", []string{"verify", "--config", filepath.Join(deploy, "testdata", "invalid", "broad-client.yaml"), "--probes", probes, "--policies", policies}, "allowed-client-networks"},
		{"upstream", []string{"verify", "--config", filepath.Join(deploy, "testdata", "invalid", "bad-upstream.yaml"), "--probes", probes, "--policies", policies}, "allowed-upstreams"},
		{"chaos", []string{"verify", "--config", filepath.Join(deploy, "testdata", "invalid", "unsafe-chaos.yaml"), "--probes", probes, "--policies", policies}, "maxDelay"},
		{"kustomize-mismatch", []string{"verify", "--config", filepath.Join(deploy, "environments", "main-lab", "dns.yaml"), "--probes", probes, "--image-env", filepath.Join(deploy, "environments", "main-lab", "image.env"), "--kustomize", filepath.Join(deploy, "testdata", "invalid", "kustomization-mismatch.yaml")}, "digest"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cmd := exec.Command(bin, tc.args...)
			out, err := cmd.CombinedOutput()
			if err == nil {
				t.Fatalf("expected failure\n%s", out)
			}
			if !strings.Contains(strings.ToLower(string(out)), strings.ToLower(tc.want)) {
				t.Fatalf("want %q in\n%s", tc.want, out)
			}
		})
	}
}

func TestMissingPolicyKindFailsClosed(t *testing.T) {
	root := repoRoot(t)
	bin := buildLabDNS(t, root)
	src := filepath.Join(root, "examples", "labdns-deploy", "policies")
	dir := t.TempDir()
	ents, err := os.ReadDir(src)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range ents {
		if e.Name() == "allowed-client-networks.yaml" {
			continue
		}
		b, err := os.ReadFile(filepath.Join(src, e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, e.Name()), b, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	cfg := filepath.Join(root, "examples", "labdns-deploy", "environments", "main-lab", "dns.yaml")
	probes := filepath.Join(root, "examples", "labdns-deploy", "environments", "main-lab", "probes.yaml")
	cmd := exec.Command(bin, "verify", "--config", cfg, "--probes", probes, "--policies", dir)
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected missing kind to fail\n%s", out)
	}
	if !strings.Contains(string(out), "AllowedClientNetworks") {
		t.Fatalf("want missing kind in\n%s", out)
	}
}

func TestScriptsFailClosedAndNoBypass(t *testing.T) {
	root := repoRoot(t)
	scripts := filepath.Join(root, "examples", "labdns-deploy", "scripts")
	for _, name := range []string{"validate.sh", "test-config.sh", "deploy.sh", "live-probe.sh", "rollback.sh"} {
		body, err := os.ReadFile(filepath.Join(scripts, name))
		if err != nil {
			t.Fatal(err)
		}
		text := string(body)
		if !strings.Contains(text, "set -euo pipefail") {
			t.Errorf("%s missing set -euo pipefail", name)
		}
		if strings.Contains(text, "|| true") {
			t.Errorf("%s bypasses failure with || true", name)
		}
	}
}

func TestTestConfigScriptPositivesAndNegatives(t *testing.T) {
	root := repoRoot(t)
	bin := buildLabDNS(t, root)
	script := filepath.Join(root, "examples", "labdns-deploy", "scripts", "test-config.sh")
	cmd := exec.Command(script, "main-lab")
	cmd.Env = append(os.Environ(), "LABDNS="+bin)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("test-config.sh: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "test-config ok") {
		t.Fatalf("output:\n%s", out)
	}
}

func TestRollbackRestoresPriorDesiredState(t *testing.T) {
	root := repoRoot(t)
	bin := buildLabDNS(t, root)
	src := filepath.Join(root, "examples", "labdns-deploy")
	tmp := t.TempDir()
	for _, rel := range []string{
		"scripts/lib.sh",
		"scripts/validate.sh",
		"scripts/deploy.sh",
		"scripts/rollback.sh",
		"policies/allowed-upstreams.yaml",
		"policies/allowed-client-networks.yaml",
		"policies/allowed-alternate-addresses.yaml",
		"policies/protected-names.yaml",
		"policies/chaos-safety.yaml",
		"environments/main-lab/dns.yaml",
		"environments/main-lab/probes.yaml",
		"environments/main-lab/image.env",
		"environments/main-lab/compose.yaml",
	} {
		dst := filepath.Join(tmp, rel)
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			t.Fatal(err)
		}
		b, err := os.ReadFile(filepath.Join(src, rel))
		if err != nil {
			t.Fatal(err)
		}
		mode := os.FileMode(0o644)
		if strings.HasSuffix(rel, ".sh") {
			mode = 0o755
		}
		if err := os.WriteFile(dst, b, mode); err != nil {
			t.Fatal(err)
		}
	}
	stub := filepath.Join(tmp, "bin")
	if err := os.MkdirAll(stub, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(stub, "docker"), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	envDir := filepath.Join(tmp, "environments", "main-lab")
	orig, err := os.ReadFile(filepath.Join(envDir, "dns.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	runDeploy := func(t *testing.T, script string) {
		t.Helper()
		cmd := exec.Command(filepath.Join(tmp, "scripts", script), "main-lab", "compose")
		cmd.Dir = tmp
		cmd.Env = append(os.Environ(),
			"LABDNS="+bin,
			"PATH="+stub+string(os.PathListSeparator)+os.Getenv("PATH"),
			"LABDNS_TOKEN_FILE="+writeToken(t, tmp),
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%s: %v\n%s", script, err, out)
		}
	}
	runDeploy(t, "deploy.sh")
	// Mutate a non-probe field so the second deploy still verifies.
	changed := bytes.Replace(orig, []byte("environment: main-lab"), []byte("environment: main-lab-next"), 1)
	if !bytes.Contains(changed, []byte("environment: main-lab-next")) {
		t.Fatal("label replace failed")
	}
	if err := os.WriteFile(filepath.Join(envDir, "dns.yaml"), changed, 0o644); err != nil {
		t.Fatal(err)
	}
	runDeploy(t, "deploy.sh")
	runDeploy(t, "rollback.sh")
	got, err := os.ReadFile(filepath.Join(envDir, "dns.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(got, []byte("environment: main-lab\n")) || bytes.Contains(got, []byte("environment: main-lab-next")) {
		t.Fatalf("rollback did not restore prior dns.yaml")
	}
}

func TestRecreationResetsRuntimeDrift(t *testing.T) {
	root := repoRoot(t)
	src := filepath.Join(root, "examples", "labdns-deploy", "environments", "main-lab", "dns.yaml")
	path := filepath.Join(t.TempDir(), "dns.yaml")
	b, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, b, 0o600); err != nil {
		t.Fatal(err)
	}
	st, err := config.LoadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	snap, err := compiler.Compile(context.Background(), st, compiler.CompileOpts{})
	if err != nil {
		t.Fatal(err)
	}
	store := snapshot.NewStore()
	store.InstallBootstrap(snap)
	svc := app.New(app.Options{Store: store, BootstrapPath: path})
	ctx := context.Background()
	actor := auth.Actor{ID: "test", Class: "token", Role: "administrator"}
	boot := svc.Store().Load()
	if _, err := svc.ActivateChaos(ctx, actor, app.ActivationIn{
		PolicyID: "slow-tools", ExpectedRevision: boot.Revision, Reason: "ephemeral",
	}); err != nil {
		t.Fatal(err)
	}
	if svc.Store().Load().Revision == boot.Revision {
		t.Fatal("expected drift")
	}
	if _, err := svc.Reset(ctx, actor, app.ResetIn{Reason: "recreate"}); err != nil {
		t.Fatal(err)
	}
	if svc.Store().Load().Revision != boot.Revision {
		t.Fatal("reset did not restore bootstrap")
	}
}

func TestComposeAndK8sIsolateManagementAndPinDigest(t *testing.T) {
	root := repoRoot(t)
	compose := read(t, filepath.Join(root, "examples", "labdns-deploy", "environments", "main-lab", "compose.yaml"))
	for _, want := range []string{
		"127.0.0.1:8080:8080/tcp",
		"read_only: true",
		"cap_drop:",
		"no-new-privileges:true",
		"65532:65532",
		"labdns-token",
		"${LABDNS_IMAGE",
	} {
		if !strings.Contains(compose, want) {
			t.Errorf("compose missing %q", want)
		}
	}
	dep := read(t, filepath.Join(root, "examples", "labdns-deploy", "environments", "main-lab", "k8s", "deployment.yaml"))
	for _, want := range []string{"runAsUser: 65532", "readOnlyRootFilesystem: true", "secretName: labdns-token", "drop:"} {
		if !strings.Contains(dep, want) {
			t.Errorf("deployment missing %q", want)
		}
	}
	np := read(t, filepath.Join(root, "examples", "labdns-deploy", "environments", "main-lab", "k8s", "networkpolicy.yaml"))
	if !strings.Contains(np, "8080") || !strings.Contains(np, "labdns.dev/management") {
		t.Fatal("networkpolicy must isolate management")
	}
	svcYAML := read(t, filepath.Join(root, "examples", "labdns-deploy", "environments", "main-lab", "k8s", "service.yaml"))
	if !strings.Contains(svcYAML, "externalTrafficPolicy: Local") {
		t.Fatal("DNS Service must set externalTrafficPolicy: Local so refuse-forward sees the real client IP")
	}
	if !strings.Contains(read(t, filepath.Join(root, "examples", "labdns-deploy", "environments", "main-lab", "k8s", "management-namespace.yaml")), `labdns.dev/management: "true"`) {
		t.Fatal("missing management namespace sample")
	}
	kust := read(t, filepath.Join(root, "examples", "labdns-deploy", "environments", "main-lab", "k8s", "kustomization.yaml"))
	img, err := deploypolicy.ParseImageEnv(filepath.Join(root, "examples", "labdns-deploy", "environments", "main-lab", "image.env"))
	if err != nil {
		t.Fatal(err)
	}
	digest := strings.TrimPrefix(img, "ghcr.io/hilather/labdns@")
	if !strings.Contains(kust, digest) {
		t.Fatalf("kustomize digest %s != image.env", digest)
	}
}

func TestConfigMapGeneratorUsesDnsYAML(t *testing.T) {
	root := repoRoot(t)
	kust := read(t, filepath.Join(root, "examples", "labdns-deploy", "environments", "main-lab", "k8s", "kustomization.yaml"))
	if !strings.Contains(kust, "config.yaml=../dns.yaml") {
		t.Fatal("kustomization must generate the ConfigMap from dns.yaml (single source of truth)")
	}
	if _, err := os.Stat(filepath.Join(root, "examples", "labdns-deploy", "environments", "main-lab", "k8s", "configmap.yaml")); err == nil {
		t.Fatal("checked-in configmap.yaml would drift from dns.yaml; use configMapGenerator")
	}
}

func TestNoSecretsInTree(t *testing.T) {
	root := filepath.Join(repoRoot(t), "examples", "labdns-deploy")
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && (d.Name() == ".previous" || d.Name() == ".last") {
			return filepath.SkipDir
		}
		if d.IsDir() {
			return nil
		}
		if d.Name() == "labdns-token" {
			t.Errorf("secret file committed: %s", path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

var (
	labdnsOnce sync.Once
	labdnsBin  string
	labdnsErr  error
)

func buildLabDNS(t *testing.T, root string) string {
	t.Helper()
	labdnsOnce.Do(func() {
		dir, err := os.MkdirTemp("", "labdns-gitops-")
		if err != nil {
			labdnsErr = err
			return
		}
		out := filepath.Join(dir, "labdns")
		cmd := exec.Command("go", "build", "-o", out, "./cmd/labdns")
		cmd.Dir = root
		if b, err := cmd.CombinedOutput(); err != nil {
			labdnsErr = err
			labdnsBin = string(b)
			return
		}
		labdnsBin = out
	})
	if labdnsErr != nil {
		t.Fatalf("go build: %v\n%s", labdnsErr, labdnsBin)
	}
	return labdnsBin
}

func writeToken(t *testing.T, dir string) string {
	t.Helper()
	p := filepath.Join(dir, "secrets", "labdns-token")
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte("test-token\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func read(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found")
		}
		dir = parent
	}
}

func TestKubernetesDeployRestartsUnchangedDesiredState(t *testing.T) {
	for _, failRestart := range []bool{false, true} {
		t.Run(fmt.Sprint(failRestart), func(t *testing.T) {
			root := repoRoot(t)
			tmp := t.TempDir()
			scripts := filepath.Join(tmp, "scripts")
			envDir := filepath.Join(tmp, "environments", "main-lab")
			bin := filepath.Join(tmp, "bin")
			for _, dir := range []string{scripts, filepath.Join(envDir, "k8s"), bin} {
				if err := os.MkdirAll(dir, 0755); err != nil {
					t.Fatal(err)
				}
			}
			write := func(path, body string) {
				t.Helper()
				if err := os.WriteFile(path, []byte(body), 0755); err != nil {
					t.Fatal(err)
				}
			}
			for _, name := range []string{"deploy.sh", "lib.sh"} {
				write(filepath.Join(scripts, name), read(t, filepath.Join(root, "examples", "labdns-deploy", "scripts", name)))
			}
			write(filepath.Join(scripts, "validate.sh"), "#!/bin/sh\nexit 0\n")
			write(filepath.Join(envDir, "dns.yaml"), "same desired state\n")
			write(filepath.Join(envDir, "image.env"), "LABDNS_IMAGE=ghcr.io/hilather/labdns@sha256:abc\n")
			log := filepath.Join(tmp, "kubectl.log")
			write(filepath.Join(bin, "kubectl"), `#!/bin/sh
printf '%s\n' "$*" >> "$KUBECTL_LOG"
if [ "$FAIL_RESTART" = true ] && [ "$1 $2" = "rollout restart" ]; then exit 1; fi
`)
			run := func() ([]byte, error) {
				cmd := exec.Command(filepath.Join(scripts, "deploy.sh"), "main-lab", "k8s")
				cmd.Env = append(os.Environ(), "PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"), "KUBECTL_LOG="+log, "FAIL_RESTART="+fmt.Sprint(failRestart))
				return cmd.CombinedOutput()
			}
			attempts := 2
			if failRestart {
				attempts = 1
			}
			for i := 0; i < attempts; i++ {
				out, err := run()
				if failRestart {
					if err == nil {
						t.Fatalf("failed restart reported success: %s", out)
					}
				} else if err != nil {
					t.Fatalf("deploy: %v %s", err, out)
				}
			}
			calls := strings.Split(strings.TrimSpace(read(t, log)), "\n")
			want := []string{"apply -k " + filepath.Join(envDir, "k8s"), "rollout restart deployment/labdns -n labdns-main-lab"}
			if !failRestart {
				want = append(want, "rollout status deployment/labdns -n labdns-main-lab --timeout=120s")
				want = append(want, want...)
			}
			if !reflect.DeepEqual(calls, want) {
				t.Fatalf("calls=%q want=%q", calls, want)
			}
			_, err := os.Stat(filepath.Join(envDir, ".last", "dns.yaml"))
			if failRestart && !os.IsNotExist(err) {
				t.Fatal("failed deployment recorded success snapshot")
			}
			if !failRestart && err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestKubernetesRollbackRestoresImagePinAndKustomization(t *testing.T) {
	root := repoRoot(t)
	tmp := t.TempDir()
	scripts := filepath.Join(tmp, "scripts")
	envDir := filepath.Join(tmp, "environments", "main-lab")
	bin := filepath.Join(tmp, "bin")
	for _, dir := range []string{scripts, filepath.Join(envDir, "k8s"), bin} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}
	write := func(path, body string) {
		t.Helper()
		if err := os.WriteFile(path, []byte(body), 0755); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{"deploy.sh", "lib.sh", "rollback.sh"} {
		write(filepath.Join(scripts, name), read(t, filepath.Join(root, "examples", "labdns-deploy", "scripts", name)))
	}
	write(filepath.Join(scripts, "validate.sh"), `#!/bin/sh
set -eu
root=$(dirname "$0")/..
pin=$(cat "$root/environments/main-lab/image.env")
kustomization=$(cat "$root/environments/main-lab/k8s/kustomization.yaml")
[ "$pin" = "$kustomization" ]
`)
	write(filepath.Join(bin, "kubectl"), "#!/bin/sh\nexit 0\n")
	write(filepath.Join(envDir, "dns.yaml"), "desired state\n")
	image := filepath.Join(envDir, "image.env")
	kustomization := filepath.Join(envDir, "k8s", "kustomization.yaml")
	run := func(script string) {
		t.Helper()
		cmd := exec.Command(filepath.Join(scripts, script), "main-lab", "k8s")
		cmd.Env = append(os.Environ(), "PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"))
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%s: %v %s", script, err, out)
		}
	}
	oldPin := "LABDNS_IMAGE=ghcr.io/hilather/labdns@sha256:old\n"
	newPin := "LABDNS_IMAGE=ghcr.io/hilather/labdns@sha256:new\n"
	write(image, oldPin)
	write(kustomization, oldPin)
	run("deploy.sh")
	write(image, newPin)
	write(kustomization, newPin)
	run("deploy.sh")
	run("rollback.sh")
	if got := read(t, image); got != oldPin {
		t.Fatalf("image pin=%s want=%s", got, oldPin)
	}
	if got := read(t, kustomization); got != oldPin {
		t.Fatalf("kustomization pin=%s want=%s", got, oldPin)
	}
}

func TestFailedKubernetesDeploymentPreservesSuccessfulSnapshots(t *testing.T) {
	for _, failAction := range []string{"apply", "rollout restart", "rollout status"} {
		t.Run(failAction, func(t *testing.T) {
			root := repoRoot(t)
			tmp := t.TempDir()
			scripts := filepath.Join(tmp, "scripts")
			envDir := filepath.Join(tmp, "environments", "main-lab")
			bin := filepath.Join(tmp, "bin")
			for _, dir := range []string{scripts, filepath.Join(envDir, "k8s"), bin} {
				if err := os.MkdirAll(dir, 0755); err != nil {
					t.Fatal(err)
				}
			}
			write := func(path, body string) {
				t.Helper()
				if err := os.WriteFile(path, []byte(body), 0755); err != nil {
					t.Fatal(err)
				}
			}
			for _, name := range []string{"deploy.sh", "lib.sh", "rollback.sh"} {
				write(filepath.Join(scripts, name), read(t, filepath.Join(root, "examples", "labdns-deploy", "scripts", name)))
			}
			write(filepath.Join(scripts, "validate.sh"), "#!/bin/sh\nexit 0\n")
			write(filepath.Join(bin, "kubectl"), `#!/bin/sh
if [ -n "$FAIL_ACTION" ]; then
    case "$*" in "$FAIL_ACTION"*) exit 1 ;; esac
fi
exit 0
`)
			run := func(script, failure string) ([]byte, error) {
				cmd := exec.Command(filepath.Join(scripts, script), "main-lab", "k8s")
				cmd.Env = append(os.Environ(), "PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"), "FAIL_ACTION="+failure)
				return cmd.CombinedOutput()
			}
			contents := func(name string) map[string]string {
				return map[string]string{
					"dns.yaml":           "desired " + name + "\n",
					"image.env":          "LABDNS_IMAGE=ghcr.io/hilather/labdns@sha256:" + name + "\n",
					"kustomization.yaml": "image " + name + "\n",
				}
			}
			candidate := func(name string) {
				for file, body := range contents(name) {
					dir := envDir
					if file == "kustomization.yaml" {
						dir = filepath.Join(dir, "k8s")
					}
					write(filepath.Join(dir, file), body)
				}
			}
			assertSnapshot := func(dir, name string) {
				t.Helper()
				for file, want := range contents(name) {
					if got := read(t, filepath.Join(envDir, dir, file)); got != want {
						t.Fatalf("%s/%s=%q want %q", dir, file, got, want)
					}
				}
			}
			for _, name := range []string{"A", "B"} {
				candidate(name)
				if out, err := run("deploy.sh", ""); err != nil {
					t.Fatalf("deploy %s: %v %s", name, err, out)
				}
			}
			assertSnapshot(".last", "B")
			assertSnapshot(".previous", "A")
			candidate("C")
			if out, err := run("deploy.sh", failAction); err == nil {
				t.Fatalf("failed deployment reported success: %s", out)
			}
			assertSnapshot(".last", "B")
			assertSnapshot(".previous", "A")
			if out, err := run("rollback.sh", ""); err != nil {
				t.Fatalf("rollback: %v %s", err, out)
			}
			assertSnapshot(".last", "A")
			assertSnapshot(".previous", "B")
			for file, want := range contents("A") {
				dir := envDir
				if file == "kustomization.yaml" {
					dir = filepath.Join(dir, "k8s")
				}
				if got := read(t, filepath.Join(dir, file)); got != want {
					t.Fatalf("rollback %s=%q want %q", file, got, want)
				}
			}
		})
	}
}
