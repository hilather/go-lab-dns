package capabilities

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// designTableHeading is the implementation-design freeze the harness diffs against.
const designTableHeading = "### Capability list (REST ↔ MCP)"

// docs22UIMapHeading is the Capability → UI map the harness diffs against.
const docs22UIMapHeading = "## Capability → UI map"

type tableRow struct {
	Title     string
	REST      []string
	Tools     []string
	Resources []string
	RESTOnly  bool
	Scopes    []string
}

func TestParityHarnessMatchesDesignTable(t *testing.T) {
	rows := parseDesignCapabilityTable(t)
	if len(rows) != TableRowCount {
		t.Fatalf("design table rows=%d want %d (row missing or added)", len(rows), TableRowCount)
	}
	got := All()
	if len(got) != len(rows) {
		t.Fatalf("registry rows=%d design=%d", len(got), len(rows))
	}
	for i, row := range rows {
		c := got[i]
		if c.Title != row.Title {
			t.Errorf("row %d title=%q design=%q (renamed?)", i, c.Title, row.Title)
			continue
		}
		gotREST := restRefs(c)
		if !stringSlicesEqual(gotREST, row.REST) {
			t.Errorf("%s REST=%v design=%v", c.ID, gotREST, row.REST)
		}
		var tools, resources []string
		if c.MCP != nil {
			tools = append([]string(nil), c.MCP.Tools...)
			resources = append([]string(nil), c.MCP.Resources...)
		}
		if !stringSlicesEqual(tools, row.Tools) {
			t.Errorf("%s tools=%v design=%v (renamed?)", c.ID, tools, row.Tools)
		}
		if !stringSlicesEqual(resources, row.Resources) {
			t.Errorf("%s resources=%v design=%v (renamed?)", c.ID, resources, row.Resources)
		}
		if c.RESTOnly != row.RESTOnly {
			t.Errorf("%s RESTOnly=%v design=%v", c.ID, c.RESTOnly, row.RESTOnly)
		}
		if !stringSlicesEqual(c.RequiredScopes, row.Scopes) {
			t.Errorf("%s scopes=%v design=%v", c.ID, c.RequiredScopes, row.Scopes)
		}
		if _, ok := Lookup(c.ID); !ok {
			t.Errorf("Lookup(%s) missing", c.ID)
		}
		for _, ref := range gotREST {
			method, path, _ := strings.Cut(ref, " ")
			hit, ok := LookupREST(method, path)
			if !ok || hit.ID != c.ID {
				t.Errorf("LookupREST(%s) = %+v ok=%v", ref, hit, ok)
			}
		}
	}
}

func TestParityWritesHaveMCPTools(t *testing.T) {
	for _, c := range All() {
		if c.RESTOnly {
			continue
		}
		write := false
		for _, b := range c.REST {
			if b.Method != "GET" {
				write = true
			}
		}
		if (write || c.Mutating) && (c.MCP == nil || len(c.MCP.Tools) == 0) {
			t.Errorf("%s is a write without an MCP tool", c.ID)
		}
	}
}

func TestParityMCPMutationsHaveREST(t *testing.T) {
	for _, c := range All() {
		if !c.Mutating {
			continue
		}
		if len(c.REST) == 0 {
			t.Errorf("%s mutating without REST", c.ID)
		}
		if c.MCP == nil || len(c.MCP.Tools) == 0 {
			t.Errorf("%s mutating without MCP tool", c.ID)
		}
	}
}

func TestParityUIBindingsComplete(t *testing.T) {
	want := frozenUIBindings()
	if len(want) != TableRowCount {
		t.Fatalf("frozen UI map=%d want %d", len(want), TableRowCount)
	}
	docs := parseDocs22UIMap(t)
	if len(docs) != TableRowCount {
		t.Fatalf("docs/22 UI map rows=%d want %d (row missing or added)", len(docs), TableRowCount)
	}
	got := All()
	if len(got) != TableRowCount {
		t.Fatalf("catalog rows=%d want %d", len(got), TableRowCount)
	}
	for i, c := range got {
		wantDisp := DispositionParityRequired
		if c.RESTOnly {
			wantDisp = DispositionRESTOnlyProtocol
		}
		if gotDisp := c.Disposition(); gotDisp != wantDisp {
			t.Errorf("%s Disposition()=%s want %s", c.ID, gotDisp, wantDisp)
		}
		if c.UI == nil {
			t.Errorf("%s missing UI binding", c.ID)
			continue
		}
		frozen, ok := want[c.ID]
		if !ok {
			t.Errorf("%s missing from frozen UI map", c.ID)
		} else if c.UI.Route != frozen.Route || c.UI.Action != frozen.Action {
			t.Errorf("%s UI={%s %s} frozen={%s %s}", c.ID, c.UI.Route, c.UI.Action, frozen.Route, frozen.Action)
		}
		row := docs[i]
		if c.Title != row.Title {
			t.Errorf("row %d title=%q docs/22=%q (renamed?)", i, c.Title, row.Title)
			continue
		}
		if c.UI.Route != row.Route || c.UI.Action != row.Action {
			t.Errorf("%s UI={%s %s} docs/22={%s %s}", c.ID, c.UI.Route, c.UI.Action, row.Route, row.Action)
		}
	}
}

func TestDocs05TableCoversFrozenTitles(t *testing.T) {
	body := readRepoFile(t, "docs", "05-control-plane-and-parity.md")
	if strings.Contains(body, "Handler          Handler") || strings.Contains(body, "REST             *RESTBinding") {
		t.Fatal("docs/05 still shows the pack Handler/*RESTBinding sketch; adapters must follow capability.go")
	}
	for _, c := range All() {
		if !strings.Contains(body, c.Title) {
			t.Errorf("docs/05 missing capability title %q", c.Title)
		}
		if c.MCP == nil {
			continue
		}
		for _, tool := range c.MCP.Tools {
			if !strings.Contains(body, tool) {
				t.Errorf("docs/05 missing tool %s", tool)
			}
		}
	}
}

func TestDocs07ListsEveryResource(t *testing.T) {
	body := readRepoFile(t, "docs", "07-mcp-api.md")
	for _, uri := range Resources() {
		if !strings.Contains(body, uri) {
			t.Errorf("docs/07 missing resource %s", uri)
		}
	}
}

func TestDocs06UsesFrozenChaosTemplates(t *testing.T) {
	body := readRepoFile(t, "docs", "06-rest-api.md")
	for _, path := range []string{
		"/v1/chaos/policies/{policyId}",
		"/v1/chaos/policies/{id}:activate",
		"/v1/chaos/policies/{id}:deactivate",
		"/v1/chaos/policies/{id}:expire",
	} {
		if !strings.Contains(body, path) {
			t.Errorf("docs/06 missing frozen path %s", path)
		}
	}
	if strings.Contains(body, "/v1/chaos/policies/{policyId}:activate") ||
		strings.Contains(body, "/v1/chaos/policies/{policyId}:deactivate") {
		t.Fatal("docs/06 still uses {policyId} on activate/deactivate; catalog spelling is {id}")
	}
}

type uiMapRow struct {
	Title  string
	Route  string
	Action string
}

func frozenUIBindings() map[ID]UIBinding {
	return map[ID]UIBinding{
		HealthLive:         {Route: "/", Action: "view"},
		HealthReady:        {Route: "/", Action: "view"},
		Version:            {Route: "/", Action: "view"},
		CapabilitiesID:     {Route: "/capabilities", Action: "view"},
		Status:             {Route: "/", Action: "view"},
		SchemaConfig:       {Route: "/schema", Action: "view"},
		StateGet:           {Route: "/state", Action: "view"},
		StateValidate:      {Route: "/state", Action: "mutate"},
		ChangePlan:         {Route: "/changes", Action: "mutate"},
		ChangeApply:        {Route: "/changes", Action: "mutate"},
		StateExport:        {Route: "/state", Action: "view"},
		StateReset:         {Route: "/reset", Action: "mutate"},
		Zones:              {Route: "/zones", Action: "view"},
		Records:            {Route: "/zones/:zoneId", Action: "view"},
		Resolve:            {Route: "/resolve", Action: "view"},
		ResolveExplain:     {Route: "/resolve", Action: "view"},
		ForwardingPolicies: {Route: "/forwarding", Action: "view"},
		UpstreamPools:      {Route: "/forwarding", Action: "view"},
		UpstreamsStatus:    {Route: "/forwarding", Action: "view"},
		CacheStatus:        {Route: "/cache", Action: "view"},
		CacheFlush:         {Route: "/cache", Action: "mutate"},
		ChaosStatus:        {Route: "/chaos", Action: "view"},
		ChaosPolicies:      {Route: "/chaos", Action: "view"},
		ChaosSimulate:      {Route: "/chaos", Action: "view"},
		ChaosActivate:      {Route: "/chaos/:policyId", Action: "mutate"},
		ChaosSetExpiry:     {Route: "/chaos/:policyId", Action: "mutate"},
		ChaosEmergency:     {Route: "/", Action: "mutate"},
		AuditList:          {Route: "/audit", Action: "view"},
		AuditGet:           {Route: "/audit/:eventId", Action: "view"},
		DocsDNSSemantics:   {Route: "/docs/:id", Action: "view"},
		DocsChaosSafety:    {Route: "/docs/:id", Action: "view"},
	}
}

func parseDocs22UIMap(t *testing.T) []uiMapRow {
	t.Helper()
	body := readRepoFile(t, "docs", "22-web-ui.md")
	idx := strings.Index(body, docs22UIMapHeading)
	if idx < 0 {
		t.Fatal("docs/22-web-ui.md: missing Capability → UI map heading")
	}
	lines := strings.Split(body[idx:], "\n")
	var rows []uiMapRow
	headerSeen := false
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "|") {
			if headerSeen && len(rows) > 0 {
				break
			}
			continue
		}
		cells := splitTableRow(line)
		if len(cells) < 3 {
			continue
		}
		if cells[0] == "Capability" {
			headerSeen = true
			continue
		}
		if strings.HasPrefix(cells[0], "---") {
			continue
		}
		toks := backticks(cells[2])
		if len(toks) < 2 {
			t.Fatalf("docs/22 %q: UI column %q want `route` `action`", cells[0], cells[2])
		}
		rows = append(rows, uiMapRow{Title: cells[0], Route: toks[0], Action: toks[1]})
	}
	if len(rows) == 0 {
		t.Fatal("parsed zero UI map rows from docs/22-web-ui.md")
	}
	return rows
}

func parseDesignCapabilityTable(t *testing.T) []tableRow {
	t.Helper()
	body := readRepoFile(t, "docs", "implementation-design.md")
	idx := strings.Index(body, designTableHeading)
	if idx < 0 {
		t.Fatal("docs/implementation-design.md: missing capability table heading")
	}
	rest := body[idx:]
	lines := strings.Split(rest, "\n")
	var rows []tableRow
	headerSeen := false
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "|") {
			if headerSeen && len(rows) > 0 {
				break
			}
			continue
		}
		cells := splitTableRow(line)
		if len(cells) < 4 {
			continue
		}
		if cells[0] == "Capability" {
			headerSeen = true
			continue
		}
		if strings.HasPrefix(cells[0], "---") {
			continue
		}
		row := tableRow{Title: cells[0]}
		row.REST = expandRESTTokens(backticks(cells[1]))
		mcp := cells[2]
		// Version row mentions dns_capabilities_get only as an embedded payload, not a second tool.
		if i := strings.Index(strings.ToLower(mcp), "embedded"); i >= 0 {
			mcp = mcp[:i]
		}
		if strings.Contains(strings.ToLower(mcp), "not a tool") {
			row.RESTOnly = true
		} else {
			for _, tok := range backticks(mcp) {
				switch {
				case strings.HasPrefix(tok, "labdns://"):
					row.Resources = append(row.Resources, tok)
				case strings.HasPrefix(tok, "dns_"):
					row.Tools = append(row.Tools, tok)
				}
			}
		}
		if cells[3] != "none" {
			row.Scopes = backticks(cells[3])
		}
		rows = append(rows, row)
	}
	if len(rows) == 0 {
		t.Fatal("parsed zero capability rows from implementation-design.md")
	}
	return rows
}

func splitTableRow(line string) []string {
	line = strings.TrimPrefix(line, "|")
	line = strings.TrimSuffix(line, "|")
	parts := strings.Split(line, "|")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		out = append(out, strings.TrimSpace(p))
	}
	return out
}

var tickRE = regexp.MustCompile("`([^`]+)`")

func backticks(s string) []string {
	ms := tickRE.FindAllStringSubmatch(s, -1)
	var out []string
	for _, m := range ms {
		out = append(out, m[1])
	}
	return out
}

func expandRESTTokens(tokens []string) []string {
	var last string
	out := make([]string, 0, len(tokens))
	for _, tok := range tokens {
		if strings.HasPrefix(tok, ":") && last != "" {
			if i := strings.LastIndex(last, ":"); i > strings.LastIndex(last, "/") {
				tok = last[:i] + tok
			}
		}
		last = tok
		out = append(out, tok)
	}
	return out
}

func restRefs(c Capability) []string {
	out := make([]string, 0, len(c.REST))
	for _, b := range c.REST {
		out = append(out, b.RESTRef())
	}
	return out
}

func stringSlicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func readRepoFile(t *testing.T, elem ...string) string {
	t.Helper()
	root := repoRoot(t)
	b, err := os.ReadFile(filepath.Join(append([]string{root}, elem...)...))
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
