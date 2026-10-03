package config

import (
	"encoding/json"
	"math"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/hilather/go-lab-dns/internal/domainerr"
	"github.com/hilather/go-lab-dns/internal/model"
)

// walkDurationFields follows model JSON fields, leaving arbitrary maps such
// as labels untouched. Input and export must use the same type-aware traversal.
func walkDurationFields(v any, typ reflect.Type, path string, visit func(map[string]any, string, any, string)) {
	for typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}
	switch typ.Kind() {
	case reflect.Struct:
		obj, ok := v.(map[string]any)
		if !ok {
			return
		}
		for i := 0; i < typ.NumField(); i++ {
			field := typ.Field(i)
			key, _, _ := strings.Cut(field.Tag.Get("json"), ",")
			child, exists := obj[key]
			if !exists || key == "" || key == "-" {
				continue
			}
			p := joinPath(path, key)
			if field.Type == reflect.TypeFor[time.Duration]() {
				visit(obj, key, child, p)
			} else {
				walkDurationFields(child, field.Type, p, visit)
			}
		}
	case reflect.Slice, reflect.Array:
		children, ok := v.([]any)
		if !ok {
			return
		}
		for i, child := range children {
			walkDurationFields(child, typ.Elem(), indexPath(path, i), visit)
		}
	}
}

// convertDurations rewrites actual duration strings to nanoseconds for
// encoding/json. Non-string numbers except literal zero are rejected.
func convertDurations(v any, path string) []domainerr.FieldViolation {
	var vs []domainerr.FieldViolation
	walkDurationFields(v, reflect.TypeFor[model.State](), path, func(obj map[string]any, key string, child any, p string) {
		if violation, ok := coerceDurationField(obj, key, child, p); !ok {
			vs = append(vs, violation)
		}
	})
	return vs
}

func coerceDurationField(obj map[string]any, key string, child any, path string) (domainerr.FieldViolation, bool) {
	if child == nil {
		return domainerr.FieldViolation{}, true
	}
	switch n := child.(type) {
	case string:
		d, err := time.ParseDuration(n)
		if err != nil {
			return domainerr.FieldViolation{
				Path:    path,
				Code:    violationInvalidValue,
				Message: "duration must use Go time.ParseDuration syntax (for example 30s)",
			}, false
		}
		obj[key] = int64(d)
		return domainerr.FieldViolation{}, true
	default:
		if d, ok := jsonNumberDuration(n); ok {
			if d == 0 {
				obj[key] = int64(0)
				return domainerr.FieldViolation{}, true
			}
		}
		return domainerr.FieldViolation{
			Path:    path,
			Code:    violationInvalidValue,
			Message: "duration must be a string such as 30s, not a bare number",
		}, false
	}
}

func convertDurationNumbersToStrings(v any) {
	walkDurationFields(v, reflect.TypeFor[model.State](), "", func(obj map[string]any, key string, child any, _ string) {
		if d, ok := jsonNumberDuration(child); ok {
			obj[key] = FormatDuration(d)
		}
	})
}

func jsonNumberDuration(v any) (time.Duration, bool) {
	switch n := v.(type) {
	case json.Number:
		i, err := n.Int64()
		if err != nil {
			return 0, false
		}
		return time.Duration(i), true
	case int64:
		return time.Duration(n), true
	case float64:
		return time.Duration(n), true
	case int:
		return time.Duration(n), true
	default:
		return 0, false
	}
}

// FormatDuration is the canonical duration spelling used in export and hashes.
// It prefers a single whole unit (24h, 5m, 30s, 100ms) over Go's "1h0m0s".
func FormatDuration(d time.Duration) string {
	if d == 0 {
		return "0s"
	}
	if d == time.Duration(math.MinInt64) {
		return strconv.FormatInt(int64(d), 10) + "ns"
	}
	if d < 0 {
		return "-" + FormatDuration(-d)
	}
	if d%time.Hour == 0 {
		return strconv.FormatInt(int64(d/time.Hour), 10) + "h"
	}
	if d%time.Minute == 0 {
		return strconv.FormatInt(int64(d/time.Minute), 10) + "m"
	}
	if d%time.Second == 0 {
		return strconv.FormatInt(int64(d/time.Second), 10) + "s"
	}
	if d%time.Millisecond == 0 {
		return strconv.FormatInt(int64(d/time.Millisecond), 10) + "ms"
	}
	if d%time.Microsecond == 0 {
		return strconv.FormatInt(int64(d/time.Microsecond), 10) + "us"
	}
	return strconv.FormatInt(int64(d), 10) + "ns"
}
