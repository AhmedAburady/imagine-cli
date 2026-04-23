// Package flagspec provides a reflection-based DSL for declaring a provider's
// private flags as struct tags on a typed Options struct. A provider that
// opts in writes one tagged struct and gets Cobra flag binding, enum/range
// validation, defaults, model-alias resolution, and automatic ownership-gate
// registration for free.
//
// A provider that needs validation the DSL can't express (cross-field checks,
// custom parsers) implements optional methods on *Options:
//
//	func (o *Options) Normalize()                            // trim, uppercase, …
//	func (o *Options) Validate(info providers.Info) error    // cross-field checks
//
// Both are called by Read after the reflection-based population.
//
// # Struct-tag grammar
//
//	flag:"<name>[,<shorthand>]" — register the field as a CLI flag.
//	                              Use flag:"-" to skip a field.
//	desc:"<help text>"          — flag description shown in --help.
//	default:"<value>"           — default value (string form, parsed per type).
//	enum:"<a>,<b>,<c>"          — allowed values, case-insensitive match,
//	                              canonicalised to the listed form. Empty
//	                              input (and no default) skips the check.
//	enum:"@models"              — allowed values are the provider's Info.Models
//	                              IDs + aliases; resolved to the canonical ID.
//	range:"<min>:<max>"         — numeric range check (int fields only).
//
// # Supported field kinds
//
// string, bool, int. These cover every parameter the shipped providers need.
package flagspec

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/AhmedAburady/imagine-cli/providers"
)

// Bind registers Cobra flags derived from prototype's struct tags on cmd.
// Idempotent: if a flag with the same name already exists on cmd.Flags(),
// Bind skips it — providers that share flag names (historically Gemini and
// Vertex) can both call Bind against their own tagged structs safely.
//
// Bind panics on malformed tags (unsupported field kind, non-numeric range
// for int, range on a non-int field, etc.). These are programmer errors
// discoverable at init() time — a panic is louder than an error the caller
// can accidentally swallow, and nothing useful is recoverable from them.
func Bind(cmd *cobra.Command, prototype any) {
	t := structType(prototype)
	flags := cmd.Flags()

	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		name, short, skip := parseFlagTag(f)
		if skip {
			continue
		}
		if flags.Lookup(name) != nil {
			continue // idempotent
		}
		desc := f.Tag.Get("desc")
		defTag := f.Tag.Get("default")
		rangeTag := f.Tag.Get("range")

		// Validate tag combinations before registering anything — a
		// malformed field must not leave a half-configured flag behind.
		if rangeTag != "" && f.Type.Kind() != reflect.Int {
			panic(fmt.Sprintf("flagspec: field %s: range tag only valid on int fields", f.Name))
		}
		if rangeTag != "" {
			if _, _, err := parseRange(rangeTag); err != nil {
				panic(fmt.Sprintf("flagspec: field %s: %v", f.Name, err))
			}
		}

		switch f.Type.Kind() {
		case reflect.String:
			if short != "" {
				flags.StringP(name, short, defTag, desc)
			} else {
				flags.String(name, defTag, desc)
			}

		case reflect.Bool:
			var def bool
			if defTag != "" {
				parsed, err := strconv.ParseBool(defTag)
				if err != nil {
					panic(fmt.Sprintf("flagspec: field %s: invalid bool default %q: %v", f.Name, defTag, err))
				}
				def = parsed
			}
			if short != "" {
				flags.BoolP(name, short, def, desc)
			} else {
				flags.Bool(name, def, desc)
			}

		case reflect.Int:
			var def int
			if defTag != "" {
				parsed, err := strconv.Atoi(defTag)
				if err != nil {
					panic(fmt.Sprintf("flagspec: field %s: invalid int default %q: %v", f.Name, defTag, err))
				}
				def = parsed
			}
			if short != "" {
				flags.IntP(name, short, def, desc)
			} else {
				flags.Int(name, def, desc)
			}

		default:
			panic(fmt.Sprintf("flagspec: field %s: unsupported kind %s (want string, bool, int)", f.Name, f.Type.Kind()))
		}
	}
}

// Read allocates a new *T (where T is prototype's struct type), populates
// its fields from cmd's flag values, enforces enum/range/model-resolution,
// then calls optional Normalize() and Validate(providers.Info) error hooks
// on the pointer. Returns *T as any for Request.Options.
func Read(cmd *cobra.Command, prototype any, info providers.Info) (any, error) {
	t := structType(prototype)
	outPtr := reflect.New(t)
	out := outPtr.Elem()
	flags := cmd.Flags()

	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		name, _, skip := parseFlagTag(f)
		if skip {
			continue
		}
		enumTag := f.Tag.Get("enum")
		rangeTag := f.Tag.Get("range")

		switch f.Type.Kind() {
		case reflect.String:
			v, _ := flags.GetString(name)
			v = strings.TrimSpace(v)

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
			v, _ := flags.GetBool(name)
			out.Field(i).SetBool(v)

		case reflect.Int:
			v, _ := flags.GetInt(name)
			if rangeTag != "" {
				min, max, _ := parseRange(rangeTag) // already validated in Bind
				if v < min || v > max {
					return nil, fmt.Errorf("--%s must be %d-%d (got %d)", name, min, max, v)
				}
			}
			out.Field(i).SetInt(int64(v))
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
			mt.In(0) == reflect.TypeOf(providers.Info{}) &&
			mt.Out(0) == reflect.TypeOf((*error)(nil)).Elem() {
			results := m.Call([]reflect.Value{reflect.ValueOf(info)})
			if !results[0].IsNil() {
				return nil, results[0].Interface().(error)
			}
		}
	}

	return outPtr.Interface(), nil
}

// FieldNames returns the flag names declared by prototype's struct tags, in
// declaration order. Used to populate Bundle.SupportedFlags automatically.
func FieldNames(prototype any) []string {
	t := structType(prototype)
	var names []string
	for i := 0; i < t.NumField(); i++ {
		name, _, skip := parseFlagTag(t.Field(i))
		if skip {
			continue
		}
		names = append(names, name)
	}
	return names
}

// --- internal helpers -------------------------------------------------------

func structType(prototype any) reflect.Type {
	t := reflect.TypeOf(prototype)
	if t == nil {
		panic("flagspec: nil prototype")
	}
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		panic(fmt.Sprintf("flagspec: prototype must be a struct, got %s", t.Kind()))
	}
	return t
}

// parseFlagTag pulls the name and optional shorthand out of `flag:"..."`.
// Returns skip=true for fields with no flag tag or flag:"-".
func parseFlagTag(f reflect.StructField) (name, short string, skip bool) {
	tag := f.Tag.Get("flag")
	if tag == "" || tag == "-" {
		return "", "", true
	}
	parts := strings.SplitN(tag, ",", 2)
	name = strings.TrimSpace(parts[0])
	if len(parts) == 2 {
		short = strings.TrimSpace(parts[1])
	}
	return name, short, false
}

func matchEnum(input, enumTag string) (string, bool) {
	for _, raw := range strings.Split(enumTag, ",") {
		v := strings.TrimSpace(raw)
		if strings.EqualFold(v, input) {
			return v, true
		}
	}
	return "", false
}

func humanEnum(enumTag string) string {
	parts := strings.Split(enumTag, ",")
	for i, p := range parts {
		parts[i] = strings.TrimSpace(p)
	}
	return strings.Join(parts, ", ")
}

func parseRange(tag string) (min, max int, err error) {
	parts := strings.SplitN(tag, ":", 2)
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("invalid range %q, want min:max", tag)
	}
	min, err = strconv.Atoi(strings.TrimSpace(parts[0]))
	if err != nil {
		return 0, 0, fmt.Errorf("invalid range min %q", parts[0])
	}
	max, err = strconv.Atoi(strings.TrimSpace(parts[1]))
	if err != nil {
		return 0, 0, fmt.Errorf("invalid range max %q", parts[1])
	}
	if min > max {
		return 0, 0, fmt.Errorf("invalid range %q: min > max", tag)
	}
	return min, max, nil
}
