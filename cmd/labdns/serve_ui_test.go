package main

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestServeWiresEmbeddedUI(t *testing.T) {
	cfg := writeLocalConfig(t, "127.0.0.1:0", "127.0.0.1:0")
	rt, err := serveFromConfig(context.Background(), serveFlags{Config: cfg})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = rt.Shutdown(context.Background()) })
	base := "http://" + rt.mgmtLn.Addr().String()

	html := getHTML(t, base+"/")
	if html.status != http.StatusOK {
		t.Fatalf("GET / status=%d body=%s", html.status, html.body)
	}
	if !strings.Contains(html.contentType, "text/html") {
		t.Fatalf("GET / content-type=%q", html.contentType)
	}
	if !strings.Contains(html.body, "LabDNS") {
		t.Fatalf("GET / missing HTML: %s", html.body)
	}

	login := getHTML(t, base+"/login")
	if login.status != http.StatusOK {
		t.Fatalf("GET /login status=%d body=%s", login.status, login.body)
	}
	if !strings.Contains(login.body, "LabDNS") {
		t.Fatalf("GET /login missing HTML: %s", login.body)
	}

	// Loopback Identify still authorizes /v1; non-loopback 401 is in rest session tests.
	state := getHTML(t, base+"/v1/state")
	if state.status != http.StatusOK {
		t.Fatalf("GET /v1/state status=%d body=%s", state.status, state.body)
	}
}

type htmlGET struct {
	status      int
	contentType string
	body        string
}

func getHTML(t *testing.T, url string) htmlGET {
	t.Helper()
	var last error
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := http.Get(url)
		if err != nil {
			last = err
			time.Sleep(20 * time.Millisecond)
			continue
		}
		raw, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		return htmlGET{
			status:      resp.StatusCode,
			contentType: resp.Header.Get("Content-Type"),
			body:        string(raw),
		}
	}
	t.Fatalf("GET %s: %v", url, last)
	return htmlGET{}
}
