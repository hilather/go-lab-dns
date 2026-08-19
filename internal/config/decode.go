package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"strings"
	"time"

	"github.com/hilather/go-lab-dns/internal/domainerr"
	"github.com/hilather/go-lab-dns/internal/model"
	"gopkg.in/yaml.v3"
)

// Decode auto-detects YAML vs JSON and rejects unknown fields. It does not
// normalize or validate. ClientGroup.allowForward and UISpec.enabled omitted
// in the document are materialized to true here because the Go bool zero
// value is false.
func Decode(data []byte) (*model.State, error) {
	if len(data) > MaxDocumentBytes {
		return nil, domainerr.ValidationFailed("document exceeds size limit",
			domainerr.FieldViolation{Path: "", Code: violationTooLarge, Message: fmt.Sprintf("document is %d bytes; max is %d", len(data), MaxDocumentBytes)})
	}
	data = stripBOM(bytes.TrimSpace(data))
	if len(data) == 0 {
		return nil, domainerr.ValidationFailed("empty document",
			domainerr.FieldViolation{Path: "", Code: violationRequired, Message: "document is empty"})
	}
	if looksLikeJSON(data) {
		return decodeJSON(data)
	}
	return decodeYAML(data)
}

// DecodeYAML decodes a YAML document with unknown-field rejection.
func DecodeYAML(data []byte) (*model.State, error) {
	if len(data) > MaxDocumentBytes {
		return nil, domainerr.ValidationFailed("document exceeds size limit",
			domainerr.FieldViolation{Path: "", Code: violationTooLarge, Message: "document exceeds size limit"})
	}
	return decodeYAML(stripBOM(data))
}

// DecodeJSON decodes a JSON document with unknown-field rejection.
func DecodeJSON(data []byte) (*model.State, error) {
	if len(data) > MaxDocumentBytes {
		return nil, domainerr.ValidationFailed("document exceeds size limit",
			domainerr.FieldViolation{Path: "", Code: violationTooLarge, Message: "document exceeds size limit"})
	}
	return decodeJSON(stripBOM(data))
}

func decodeYAML(data []byte) (*model.State, error) {
	var raw any
	dec := yaml.NewDecoder(bytes.NewReader(data))
	if err := dec.Decode(&raw); err != nil {
		if err == io.EOF {
			return nil, domainerr.ValidationFailed("empty document",
				domainerr.FieldViolation{Path: "", Code: violationRequired, Message: "document is empty"})
		}
		return nil, domainerr.ValidationFailed("YAML decode failed",
			domainerr.FieldViolation{Path: "", Code: violationInvalidValue, Message: err.Error()})
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		return nil, domainerr.ValidationFailed("trailing YAML document",
			domainerr.FieldViolation{Path: "", Code: violationInvalidValue, Message: "document contains more than one YAML value"})
	}
	raw = stringifyKeys(raw)
	return decodeRaw(raw)
}

func decodeJSON(data []byte) (*model.State, error) {
	var raw any
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	if err := dec.Decode(&raw); err != nil {
		return nil, domainerr.ValidationFailed("JSON decode failed",
			domainerr.FieldViolation{Path: "", Code: violationInvalidValue, Message: err.Error()})
	}
	if dec.More() {
		return nil, domainerr.ValidationFailed("trailing JSON value",
			domainerr.FieldViolation{Path: "", Code: violationInvalidValue, Message: "document contains more than one JSON value"})
	}
	return decodeRaw(raw)
}

func decodeRaw(raw any) (*model.State, error) {
	applyDecodeDefaults(raw)
	if vs := convertDurations(raw, ""); len(vs) > 0 {
		return nil, domainerr.ValidationFailed("invalid durations", vs...)
	}
	if vs := unknownFields(raw, reflect.TypeOf(model.State{}), ""); len(vs) > 0 {
		return nil, domainerr.ValidationFailed("unknown fields", vs...)
	}
	b, err := json.Marshal(raw)
	if err != nil {
		return nil, domainerr.ValidationFailed("re-encode failed",
			domainerr.FieldViolation{Path: "", Code: violationInvalidValue, Message: err.Error()})
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	var st model.State
	if err := dec.Decode(&st); err != nil {
		return nil, mapJSONDecodeError(err)
	}
	return &st, nil
}

func mapJSONDecodeError(err error) error {
	msg := err.Error()
	if strings.Contains(msg, "unknown field") {
		field := msg
		if i := strings.Index(msg, `"`); i >= 0 {
			if j := strings.LastIndex(msg, `"`); j > i {
				field = msg[i+1 : j]
			}
		}
		return domainerr.ValidationFailed("unknown fields",
			domainerr.FieldViolation{Path: field, Code: violationUnknownField, Message: "unknown field"})
	}
	return domainerr.ValidationFailed("JSON decode failed",
		domainerr.FieldViolation{Path: "", Code: violationInvalidValue, Message: msg})
}

func looksLikeJSON(data []byte) bool {
	data = bytes.TrimSpace(data)
	return len(data) > 0 && (data[0] == '{' || data[0] == '[')
}

func stripBOM(data []byte) []byte {
	return bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF})
}

func stringifyKeys(v any) any {
	switch x := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(x))
		for k, child := range x {
			out[k] = stringifyKeys(child)
		}
		return out
	case map[any]any:
		out := make(map[string]any, len(x))
		for k, child := range x {
			out[fmt.Sprint(k)] = stringifyKeys(child)
		}
		return out
	case []any:
		for i, child := range x {
			x[i] = stringifyKeys(child)
		}
		return x
	default:
		return v
	}
}

// applyDecodeDefaults injects defaults that cannot be recovered from a Go zero
// value after unmarshal. UI.Enabled and AllowForward must be handled here.
func applyDecodeDefaults(v any) {
	root, ok := v.(map[string]any)
	if !ok {
		return
	}
	spec, _ := root["spec"].(map[string]any)
	if spec == nil {
		return
	}
	// UI defaulting is spec-level and must run before the access early-return.
	if rawUI, exists := spec["ui"]; !exists || rawUI == nil {
		spec["ui"] = map[string]any{"enabled": model.DefaultUIEnabled}
	} else if ui, ok := rawUI.(map[string]any); ok {
		if _, has := ui["enabled"]; !has {
			ui["enabled"] = model.DefaultUIEnabled
		}
	}
	access, _ := spec["access"].(map[string]any)
	if access == nil {
		return
	}
	groups, _ := access["clientGroups"].([]any)
	for _, g := range groups {
		gm, ok := g.(map[string]any)
		if !ok {
			continue
		}
		if _, exists := gm["allowForward"]; !exists {
			gm["allowForward"] = model.DefaultAllowForward
		}
	}
}

func unknownFields(val any, typ reflect.Type, path string) []domainerr.FieldViolation {
	if val == nil || typ == nil {
		return nil
	}
	for typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}
	switch typ.Kind() {
	case reflect.Struct:
		if typ == reflect.TypeOf(time.Time{}) {
			return nil
		}
		obj, ok := val.(map[string]any)
		if !ok {
			return nil
		}
		fields := jsonFieldMap(typ)
		var vs []domainerr.FieldViolation
		for k, child := range obj {
			ft, ok := fields[k]
			if !ok {
				vs = append(vs, domainerr.FieldViolation{
					Path:    joinPath(path, k),
					Code:    violationUnknownField,
					Message: fmt.Sprintf("unknown field %q", k),
				})
				continue
			}
			vs = append(vs, unknownFields(child, ft, joinPath(path, k))...)
		}
		return vs
	case reflect.Slice, reflect.Array:
		arr, ok := val.([]any)
		if !ok {
			return nil
		}
		var vs []domainerr.FieldViolation
		for i, child := range arr {
			vs = append(vs, unknownFields(child, typ.Elem(), indexPath(path, i))...)
		}
		return vs
	case reflect.Map:
		return nil
	default:
		return nil
	}
}

func jsonFieldMap(typ reflect.Type) map[string]reflect.Type {
	out := make(map[string]reflect.Type)
	for i := 0; i < typ.NumField(); i++ {
		f := typ.Field(i)
		if f.PkgPath != "" {
			continue
		}
		tag := f.Tag.Get("json")
		name, _, _ := strings.Cut(tag, ",")
		if name == "" || name == "-" {
			continue
		}
		out[name] = f.Type
	}
	return out
}

func joinPath(base, key string) string {
	if base == "" {
		return key
	}
	return base + "." + key
}

func indexPath(base string, i int) string {
	return fmt.Sprintf("%s[%d]", base, i)
}
