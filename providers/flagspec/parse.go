package flagspec

import (
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"

	"github.com/AhmedAburady/imagine-cli/providers"
)

// Parse populates a new *T from a map[string]any (e.g. parsed YAML or JSON
// for one batch entry). It mirrors Read's semantics — defaults, enum/range
// validation, @models resolution, optional Normalize() and Validate(Info)
// hooks — but reads from the map instead of cobra. Unknown keys produce
// an error listing valid names; missing keys fall back to the field's
// `default:"..."` tag (or the type's zero value).
//
// Type coercion accepts what gopkg.in/yaml.v3 and encoding/json typically
// produce: a string field tolerates incoming int/float/bool (rendered
// to text); an int field tolerates float64 (when whole) and numeric
// strings; a bool field requires a real bool (no string-to-bool magic).
func Parse(prototype any, values map[string]any, info providers.Info) (any, error) {
	t := structType(prototype)
	outPtr := reflect.New(t)
	out := outPtr.Elem()

	consumed := make(map[string]bool, len(values))
	knownNames := make([]string, 0, t.NumField())

	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		name, _, skip := parseFlagTag(f)
		if skip {
			continue
		}
		knownNames = append(knownNames, name)

		defTag := f.Tag.Get("default")
		enumTag := f.Tag.Get("enum")
		rangeTag := f.Tag.Get("range")

		raw, present := values[name]
		if present {
			consumed[name] = true
		}

		switch f.Type.Kind() {
		case reflect.String:
			var v string
			switch {
			case present:
				s, err := coerceString(raw, name)
				if err != nil {
					return nil, err
				}
				v = strings.TrimSpace(s)
			case defTag != "":
				v = defTag
			}

			switch {
			case enumTag == "@models":
				resolved, err := info.ResolveModel(v)
				if err != nil {
					return nil, err
				}
				v = resolved
			case enumTag != "" && v != "":
				canonical, ok := matchEnum(v, enumTag)
				if !ok {
					return nil, fmt.Errorf("invalid --%s %q (valid: %s)", name, v, humanEnum(enumTag))
				}
				v = canonical
			}
			out.Field(i).SetString(v)

		case reflect.Bool:
			var v bool
			switch {
			case present:
				b, err := coerceBool(raw, name)
				if err != nil {
					return nil, err
				}
				v = b
			case defTag != "":
				parsed, err := strconv.ParseBool(defTag)
				if err == nil {
					v = parsed
				}
			}
			out.Field(i).SetBool(v)

		case reflect.Int:
			var v int
			switch {
			case present:
				n, err := coerceInt(raw, name)
				if err != nil {
					return nil, err
				}
				v = n
			case defTag != "":
				parsed, err := strconv.Atoi(defTag)
				if err == nil {
					v = parsed
				}
			}
			if rangeTag != "" {
				min, max, _ := parseRange(rangeTag) // already validated in Bind
				if v < min || v > max {
					return nil, fmt.Errorf("--%s must be %d-%d (got %d)", name, min, max, v)
				}
			}
			out.Field(i).SetInt(int64(v))
		}
	}

	if len(values) > 0 {
		var unknown []string
		for k := range values {
			if !consumed[k] {
				unknown = append(unknown, k)
			}
		}
		if len(unknown) > 0 {
			sort.Strings(unknown)
			sort.Strings(knownNames)
			return nil, fmt.Errorf("unknown key(s) %v (valid: %v)", unknown, knownNames)
		}
	}

	// Optional Normalize() hook.
	if m := outPtr.MethodByName("Normalize"); m.IsValid() {
		mt := m.Type()
		if mt.NumIn() == 0 && mt.NumOut() == 0 {
			m.Call(nil)
		}
	}

	// Optional Validate(providers.Info) error hook.
	if m := outPtr.MethodByName("Validate"); m.IsValid() {
		mt := m.Type()
		if mt.NumIn() == 1 && mt.NumOut() == 1 &&
			mt.In(0) == reflect.TypeFor[providers.Info]() &&
			mt.Out(0) == reflect.TypeFor[error]() {
			results := m.Call([]reflect.Value{reflect.ValueOf(info)})
			if !results[0].IsNil() {
				return nil, results[0].Interface().(error)
			}
		}
	}

	return outPtr.Interface(), nil
}

func coerceString(v any, name string) (string, error) {
	switch x := v.(type) {
	case nil:
		return "", nil
	case string:
		return x, nil
	case bool:
		return strconv.FormatBool(x), nil
	case int:
		return strconv.Itoa(x), nil
	case int64:
		return strconv.FormatInt(x, 10), nil
	case float64:
		// Render integral floats as integers ("1024" not "1024.0") so
		// raw WxH dimensions in JSON-decoded numerics still match.
		if x == float64(int64(x)) {
			return strconv.FormatInt(int64(x), 10), nil
		}
		return strconv.FormatFloat(x, 'f', -1, 64), nil
	default:
		return "", fmt.Errorf("--%s must be a string (got %T)", name, v)
	}
}

func coerceBool(v any, name string) (bool, error) {
	if b, ok := v.(bool); ok {
		return b, nil
	}
	return false, fmt.Errorf("--%s must be true or false (got %T)", name, v)
}

func coerceInt(v any, name string) (int, error) {
	switch x := v.(type) {
	case int:
		return x, nil
	case int64:
		return int(x), nil
	case float64:
		if x != float64(int64(x)) {
			return 0, fmt.Errorf("--%s must be an integer (got %v)", name, x)
		}
		return int(x), nil
	case string:
		n, err := strconv.Atoi(strings.TrimSpace(x))
		if err != nil {
			return 0, fmt.Errorf("--%s must be an integer (got string %q)", name, x)
		}
		return n, nil
	default:
		return 0, fmt.Errorf("--%s must be an integer (got %T)", name, v)
	}
}
