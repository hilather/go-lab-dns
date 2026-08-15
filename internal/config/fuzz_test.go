package config

import "testing"

func FuzzDecode(f *testing.F) {
	f.Add([]byte(`{"apiVersion":"labdns.dev/v1alpha1","kind":"LabDNS","metadata":{"name":"x"},"spec":{}}`))
	f.Add([]byte("apiVersion: labdns.dev/v1alpha1\nkind: LabDNS\nmetadata:\n  name: x\nspec: {}\n"))
	f.Add([]byte(""))
	f.Add([]byte("{"))
	f.Add([]byte("[:"))
	f.Add([]byte("apiVersion: labdns.dev/v1beta1\nkind: LabDNS\n"))
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 64*1024 {
			data = data[:64*1024]
		}
		_, _ = Decode(data)
		_, _ = Load(data)
		_ = PeekAPIVersion(data)
	})
}

func TestFuzzDecodeSmoke(t *testing.T) {
	seeds := [][]byte{
		[]byte(`{"apiVersion":"labdns.dev/v1alpha1","kind":"LabDNS","metadata":{"name":"x"},"spec":{}}`),
		[]byte("apiVersion: labdns.dev/v1alpha1\nkind: LabDNS\nmetadata:\n  name: x\nspec: {}\n"),
		[]byte(""),
		[]byte("{"),
		[]byte(mustLoad(t, "valid", "empty-client-groups.yaml")),
		[]byte(mustLoad(t, "invalid", "unknown-field.yaml")),
	}
	for _, s := range seeds {
		_, _ = Decode(s)
		_, _ = Load(s)
	}
}
