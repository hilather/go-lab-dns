package deploypolicy

import (
	"fmt"
	"net/netip"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/hilather/go-lab-dns/internal/model"
	"gopkg.in/yaml.v3"
)

const policyAPIVersion = "labdns.dev/policy/v1alpha1"

// RequiredKinds are always enforced when --policies is set. Omitting a
// file used to skip that gate; missing kinds are now fail-closed.
var RequiredKinds = []string{
	"AllowedUpstreams",
	"AllowedClientNetworks",
	"AllowedAlternateAddresses",
	"ProtectedNames",
	"ChaosSafety",
}

// ImageDigest is a digest-pinned container reference.
// Tags and untagged names are rejected so GitOps cannot float on :latest.
var ImageDigest = regexp.MustCompile(`(?i)^[a-z0-9._\-/]+@sha256:[0-9a-f]{64}$`)

// Set is the union of policy files in a directory.
type Set struct {
	Upstreams           []string
	ClientNetworks      []netip.Prefix
	AlternateAddresses  []netip.Prefix
	ProtectedNames      []string
	MaxDelay            time.Duration
	MaxConcurrent       int
	MaxDropProbability  float64
	MaxActiveHighImpact int
	HaveUpstreams       bool
	HaveClients         bool
	HaveAlternates      bool
	HaveProtected       bool
	HaveChaos           bool
	// haveProtectedFile is true only when a ProtectedNames document was
	// loaded. ChaosSafety.requireProtectedNames must not satisfy that gate.
	haveProtectedFile bool
}

type fileHeader struct {
	APIVersion string `yaml:"apiVersion"`
	Kind       string `yaml:"kind"`
}

type stringListFile struct {
	fileHeader `yaml:",inline"`
	Endpoints  []string `yaml:"endpoints"`
	Networks   []string `yaml:"networks"`
	CIDRs      []string `yaml:"cidrs"`
	Names      []string `yaml:"names"`
}

type chaosSafetyFile struct {
	fileHeader            `yaml:",inline"`
	MaxDelay              string   `yaml:"maxDelay"`
	MaxConcurrentDelayed  int      `yaml:"maxConcurrentDelayed"`
	MaxDropProbability    float64  `yaml:"maxDropProbability"`
	MaxActiveHighImpact   int      `yaml:"maxActiveHighImpactPolicies"`
	RequireProtectedNames []string `yaml:"requireProtectedNames"`
}

// LoadDir reads every *.yaml / *.yml file in dir. Unknown kinds are errors
// so a typo cannot silently disable a gate.
func LoadDir(dir string) (*Set, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	out := &Set{}
	n := 0
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		ext := strings.ToLower(filepath.Ext(name))
		if ext != ".yaml" && ext != ".yml" {
			continue
		}
		path := filepath.Join(dir, name)
		if err := loadFile(path, out); err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		n++
	}
	if n == 0 {
		return nil, fmt.Errorf("no policy yaml files in %s", dir)
	}
	if missing := out.missingKinds(); len(missing) > 0 {
		return nil, fmt.Errorf("missing required policy kinds: %s", strings.Join(missing, ", "))
	}
	return out, nil
}

func (s *Set) missingKinds() []string {
	if s == nil {
		return append([]string(nil), RequiredKinds...)
	}
	var missing []string
	if !s.HaveUpstreams {
		missing = append(missing, "AllowedUpstreams")
	}
	if !s.HaveClients {
		missing = append(missing, "AllowedClientNetworks")
	}
	if !s.HaveAlternates {
		missing = append(missing, "AllowedAlternateAddresses")
	}
	if !s.haveProtectedFile {
		missing = append(missing, "ProtectedNames")
	}
	if !s.HaveChaos {
		missing = append(missing, "ChaosSafety")
	}
	return missing
}

func loadFile(path string, out *Set) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var hdr fileHeader
	if err := yaml.Unmarshal(b, &hdr); err != nil {
		return err
	}
	if hdr.APIVersion != "" && hdr.APIVersion != policyAPIVersion {
		return fmt.Errorf("unsupported apiVersion %q", hdr.APIVersion)
	}
	switch hdr.Kind {
	case "AllowedUpstreams":
		var f stringListFile
		if err := yaml.Unmarshal(b, &f); err != nil {
			return err
		}
		out.Upstreams = append(out.Upstreams, f.Endpoints...)
		out.HaveUpstreams = true
	case "AllowedClientNetworks":
		var f stringListFile
		if err := yaml.Unmarshal(b, &f); err != nil {
			return err
		}
		pfxs, err := parsePrefixes(f.Networks)
		if err != nil {
			return err
		}
		out.ClientNetworks = append(out.ClientNetworks, pfxs...)
		out.HaveClients = true
	case "AllowedAlternateAddresses":
		var f stringListFile
		if err := yaml.Unmarshal(b, &f); err != nil {
			return err
		}
		pfxs, err := parsePrefixes(f.CIDRs)
		if err != nil {
			return err
		}
		out.AlternateAddresses = append(out.AlternateAddresses, pfxs...)
		out.HaveAlternates = true
	case "ProtectedNames":
		var f stringListFile
		if err := yaml.Unmarshal(b, &f); err != nil {
			return err
		}
		for _, n := range f.Names {
			out.ProtectedNames = append(out.ProtectedNames, canonName(n))
		}
		out.HaveProtected = true
		out.haveProtectedFile = true
	case "ChaosSafety":
		var f chaosSafetyFile
		if err := yaml.Unmarshal(b, &f); err != nil {
			return err
		}
		if strings.TrimSpace(f.MaxDelay) != "" {
			d, err := time.ParseDuration(f.MaxDelay)
			if err != nil {
				return fmt.Errorf("maxDelay: %w", err)
			}
			out.MaxDelay = d
		}
		out.MaxConcurrent = f.MaxConcurrentDelayed
		out.MaxDropProbability = f.MaxDropProbability
		out.MaxActiveHighImpact = f.MaxActiveHighImpact
		for _, n := range f.RequireProtectedNames {
			out.ProtectedNames = append(out.ProtectedNames, canonName(n))
		}
		out.HaveChaos = true
		if len(f.RequireProtectedNames) > 0 {
			out.HaveProtected = true
		}
	case "":
		return fmt.Errorf("missing kind")
	default:
		return fmt.Errorf("unknown policy kind %q", hdr.Kind)
	}
	return nil
}

// CheckImage rejects mutable tags and unpinned names.
func CheckImage(ref string) error {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return fmt.Errorf("image reference is empty")
	}
	if strings.Contains(ref, "\n") || strings.Contains(ref, " ") {
		return fmt.Errorf("image reference %q is not a single token", ref)
	}
	if !ImageDigest.MatchString(ref) {
		return fmt.Errorf("image %q is not digest-pinned (require name@sha256:<64 hex>, not a tag)", ref)
	}
	return nil
}

// ParseImageEnv reads KEY=value assignments and returns LABDNS_IMAGE.
func ParseImageEnv(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	var image string
	for i, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			return "", fmt.Errorf("%s:%d: expected KEY=value", path, i+1)
		}
		key = strings.TrimSpace(key)
		val = strings.Trim(strings.TrimSpace(val), `"'`)
		if key == "LABDNS_IMAGE" {
			image = val
		}
	}
	if image == "" {
		return "", fmt.Errorf("%s: LABDNS_IMAGE is required", path)
	}
	return image, nil
}

type kustomizeImages struct {
	Images []kustomizeImage `yaml:"images"`
}

type kustomizeImage struct {
	Name    string `yaml:"name"`
	NewName string `yaml:"newName"`
	NewTag  string `yaml:"newTag"`
	Digest  string `yaml:"digest"`
}

// CheckKustomizeImage requires kustomize images[] to be the same digest pin
// as imageRef (LABDNS_IMAGE). A tag rewrite is fail-closed.
func CheckKustomizeImage(kustomizePath, imageRef string) error {
	if err := CheckImage(imageRef); err != nil {
		return err
	}
	b, err := os.ReadFile(kustomizePath)
	if err != nil {
		return err
	}
	var doc kustomizeImages
	if err := yaml.Unmarshal(b, &doc); err != nil {
		return fmt.Errorf("%s: %w", kustomizePath, err)
	}
	wantName, wantDigest, err := splitImageDigest(imageRef)
	if err != nil {
		return err
	}
	var matched bool
	for _, img := range doc.Images {
		name := strings.TrimSpace(img.Name)
		if img.NewName != "" {
			name = strings.TrimSpace(img.NewName)
		}
		if name != wantName {
			continue
		}
		matched = true
		if strings.TrimSpace(img.NewTag) != "" {
			return fmt.Errorf("%s: image %s uses newTag %q (digest pin required)", kustomizePath, name, img.NewTag)
		}
		got := strings.TrimSpace(img.Digest)
		if got == "" {
			return fmt.Errorf("%s: image %s has no digest", kustomizePath, name)
		}
		if !strings.Contains(got, ":") {
			got = "sha256:" + got
		}
		if !strings.EqualFold(got, wantDigest) {
			return fmt.Errorf("%s: image %s digest %s != %s from image pin", kustomizePath, name, got, wantDigest)
		}
	}
	if !matched {
		return fmt.Errorf("%s: no images[] entry for %s", kustomizePath, wantName)
	}
	return nil
}

func splitImageDigest(ref string) (name, digest string, err error) {
	ref = strings.TrimSpace(ref)
	name, digest, ok := strings.Cut(ref, "@")
	if !ok || name == "" || digest == "" {
		return "", "", fmt.Errorf("image %q is not name@digest", ref)
	}
	return name, digest, nil
}

// Check compares a compiled-ready desired state against the allowlists.
// Every required kind is evaluated. An empty allowlist is deny-all, not a skip.
func Check(st *model.State, pol *Set) error {
	if st == nil || pol == nil {
		return fmt.Errorf("config and policies are required")
	}
	if missing := pol.missingKinds(); len(missing) > 0 {
		return fmt.Errorf("missing required policy kinds: %s", strings.Join(missing, ", "))
	}
	var errs []string
	errs = append(errs, checkUpstreams(st, pol)...)
	errs = append(errs, checkClients(st, pol)...)
	errs = append(errs, checkAlternates(st, pol)...)
	errs = append(errs, checkProtected(st, pol)...)
	errs = append(errs, checkChaos(st, pol)...)
	if len(errs) == 0 {
		return nil
	}
	return fmt.Errorf("policy:\n  %s", strings.Join(errs, "\n  "))
}

func checkUpstreams(st *model.State, pol *Set) []string {
	allowed := map[string]bool{}
	for _, e := range pol.Upstreams {
		allowed[strings.TrimSpace(e)] = true
	}
	var errs []string
	for _, pool := range st.Spec.Forwarding.Pools {
		for _, up := range pool.Upstreams {
			if !allowed[up.Endpoint] {
				errs = append(errs, fmt.Sprintf("upstream %s (%s) is not in allowed-upstreams", up.ID, up.Endpoint))
			}
		}
	}
	return errs
}

func checkClients(st *model.State, pol *Set) []string {
	var errs []string
	for _, g := range st.Spec.Access.ClientGroups {
		for _, c := range g.CIDRs {
			pfx, err := netip.ParsePrefix(c)
			if err != nil {
				errs = append(errs, fmt.Sprintf("client group %s: invalid CIDR %s", g.ID, c))
				continue
			}
			if !containedInAny(pfx, pol.ClientNetworks) {
				errs = append(errs, fmt.Sprintf("client group %s CIDR %s is outside allowed-client-networks", g.ID, c))
			}
		}
	}
	return errs
}

func checkAlternates(st *model.State, pol *Set) []string {
	var errs []string
	for i, c := range st.Spec.Chaos.Safety.AllowedAddressCIDRs {
		pfx, err := netip.ParsePrefix(c)
		if err != nil {
			errs = append(errs, fmt.Sprintf("allowedAddressCIDRs[%d]: invalid CIDR %s", i, c))
			continue
		}
		if !containedInAny(pfx, pol.AlternateAddresses) {
			errs = append(errs, fmt.Sprintf("allowedAddressCIDRs %s is outside allowed-alternate-addresses", c))
		}
	}
	return errs
}

func checkProtected(st *model.State, pol *Set) []string {
	have := map[string]bool{}
	for _, n := range st.Spec.Chaos.Safety.ProtectedNames {
		have[canonName(string(n))] = true
	}
	var errs []string
	for _, n := range pol.ProtectedNames {
		if !have[n] {
			errs = append(errs, fmt.Sprintf("protected name %s is missing from spec.chaos.safety.protectedNames", n))
		}
	}
	return errs
}

func checkChaos(st *model.State, pol *Set) []string {
	var errs []string
	s := st.Spec.Chaos.Safety
	// Zero caps are deny (not "unset / skip"). A missing numeric field
	// therefore rejects any positive configured value.
	if s.MaxDelay > pol.MaxDelay {
		errs = append(errs, fmt.Sprintf("maxDelay %s exceeds policy cap %s", s.MaxDelay, pol.MaxDelay))
	}
	if s.MaxConcurrentDelayed > pol.MaxConcurrent {
		errs = append(errs, fmt.Sprintf("maxConcurrentDelayed %d exceeds policy cap %d", s.MaxConcurrentDelayed, pol.MaxConcurrent))
	}
	if s.MaxDropProbability > pol.MaxDropProbability+1e-9 {
		errs = append(errs, fmt.Sprintf("maxDropProbability %g exceeds policy cap %g", s.MaxDropProbability, pol.MaxDropProbability))
	}
	if s.MaxActiveHighImpactPolicies > pol.MaxActiveHighImpact {
		errs = append(errs, fmt.Sprintf("maxActiveHighImpactPolicies %d exceeds policy cap %d", s.MaxActiveHighImpactPolicies, pol.MaxActiveHighImpact))
	}
	return errs
}

func parsePrefixes(ss []string) ([]netip.Prefix, error) {
	var out []netip.Prefix
	for _, s := range ss {
		p, err := netip.ParsePrefix(strings.TrimSpace(s))
		if err != nil {
			return nil, fmt.Errorf("invalid CIDR %s: %w", s, err)
		}
		out = append(out, p)
	}
	return out, nil
}

func containedInAny(inner netip.Prefix, outer []netip.Prefix) bool {
	for _, o := range outer {
		if covers(o, inner) {
			return true
		}
	}
	return false
}

func covers(outer, inner netip.Prefix) bool {
	o := outer.Masked()
	in := inner.Masked()
	if o.Addr().BitLen() != in.Addr().BitLen() {
		return false
	}
	if o.Bits() > in.Bits() {
		return false
	}
	return o.Contains(in.Addr())
}

func canonName(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" {
		return s
	}
	if !strings.HasSuffix(s, ".") {
		s += "."
	}
	return s
}
