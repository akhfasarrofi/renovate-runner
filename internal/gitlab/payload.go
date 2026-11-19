package gitlab

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
)

// GitLab payloads are decoded into map[string]any with json.Number. These helpers read
// fields leniently: absent or falsy values count as empty, wrong types never panic.

// ErrBadPayload marks a webhook body that is JSON but not a GitLab event shape.
var ErrBadPayload = errors.New("bad payload")

// object returns an empty map for absent/falsy values and ErrBadPayload for non-objects.
func object(v any) (map[string]any, error) {
	if !truthy(v) {
		return map[string]any{}, nil
	}
	m, ok := v.(map[string]any)
	if !ok {
		return nil, ErrBadPayload
	}
	return m, nil
}

func integer(v any) (int64, bool) {
	n, ok := v.(json.Number)
	if !ok {
		return 0, false
	}
	i, err := strconv.ParseInt(n.String(), 10, 64)
	return i, err == nil
}

func truthy(v any) bool {
	switch x := v.(type) {
	case nil:
		return false
	case bool:
		return x
	case string:
		return x != ""
	case json.Number:
		f, err := x.Float64()
		return err != nil || f != 0
	case map[string]any:
		return len(x) > 0
	case []any:
		return len(x) > 0
	}
	return true
}

func text(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	if v == nil {
		return ""
	}
	return fmt.Sprint(v)
}

// textOr returns v as text, or fallback when v is absent or falsy.
func textOr(v any, fallback string) string {
	if truthy(v) {
		return text(v)
	}
	return fallback
}

// cut truncates by characters, not bytes, so multi-byte titles stay valid UTF-8.
func cut(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}
