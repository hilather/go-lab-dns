package rest

import (
	"github.com/hilather/go-lab-dns/internal/model"
	"net/netip"
	"reflect"
	"strings"
	"time"
)

// Resolution output schemas are derived from the shared model rather than a
// second field list, including the additive modeled-chaos explanation fields.
func resolutionSchemas() map[string]any {
	types := map[reflect.Type]string{
		reflect.TypeOf(model.Query{}): "DNSQuery", reflect.TypeOf(model.RR{}): "DNSRecord",
		reflect.TypeOf(model.Result{}): "DNSResult", reflect.TypeOf(model.Explanation{}): "ResolutionExplanation",
		reflect.TypeOf(model.ChaosDecision{}): "ChaosDecision", reflect.TypeOf(model.EDE{}): "ExtendedDNSError",
	}
	var schema func(reflect.Type) map[string]any
	schema = func(kind reflect.Type) map[string]any {
		if kind.Kind() == reflect.Pointer {
			return schema(kind.Elem())
		}
		if name, ok := types[kind]; ok {
			return map[string]any{"$ref": "#/components/schemas/" + name}
		}
		if kind == reflect.TypeOf(time.Duration(0)) || kind == reflect.TypeOf(netip.Addr{}) {
			return map[string]any{"type": "string"}
		}
		switch kind.Kind() {
		case reflect.String:
			return map[string]any{"type": "string"}
		case reflect.Bool:
			return map[string]any{"type": "boolean"}
		case reflect.Slice:
			return map[string]any{"type": "array", "items": schema(kind.Elem())}
		default:
			return map[string]any{"type": "integer"}
		}
	}
	result := map[string]any{}
	for kind, name := range types {
		properties := map[string]any{}
		required := []string{}
		for i := 0; i < kind.NumField(); i++ {
			field := kind.Field(i)
			tag := field.Tag.Get("json")
			property := strings.Split(tag, ",")[0]
			if property == "" || property == "-" {
				continue
			}
			properties[property] = schema(field.Type)
			if !strings.Contains(tag, ",omitempty") {
				required = append(required, property)
			}
		}
		result[name] = map[string]any{"type": "object", "properties": properties, "required": required}
	}
	for _, name := range []string{"ResolveOut", "ExplainOut"} {
		properties := map[string]any{"result": schema(reflect.TypeOf(model.Result{}))}
		required := []string{"result"}
		if name == "ExplainOut" {
			properties["explanation"] = schema(reflect.TypeOf(model.Explanation{}))
			required = append(required, "explanation")
		}
		result[name] = map[string]any{"type": "object", "properties": properties, "required": required}
	}
	return result
}
