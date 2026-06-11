package providers

import (
	"slices"
	"sort"
	"testing"

	"github.com/spf13/cobra"
)

func TestUnusedSecretKeys(t *testing.T) {
	const name = "multiauth_test"
	Register(name, Bundle{
		Factory:   func(Auth) (Provider, error) { return nil, nil },
		BindFlags: func(*cobra.Command) {},
		AuthMethods: []AuthMethod{
			{Key: "api_key", Fields: []ConfigField{{Key: "api_key"}, {Key: "vision_model"}}},
			{Key: "subscription"},
		},
	})

	cases := []struct {
		name string
		raw  map[string]string
		want []string
	}{
		{"subscription skips api_key", map[string]string{"auth_method": "subscription", "api_key": "op://x"}, []string{"api_key", "vision_model"}},
		{"api_key skips nothing", map[string]string{"auth_method": "api_key", "api_key": "op://x"}, nil},
		{"no auth_method resolves all", map[string]string{"api_key": "op://x"}, nil},
		{"unknown auth_method resolves all", map[string]string{"auth_method": "bogus", "api_key": "op://x"}, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := UnusedSecretKeys(name, c.raw)
			sort.Strings(got)
			want := append([]string(nil), c.want...)
			sort.Strings(want)
			if !slices.Equal(got, want) {
				t.Errorf("UnusedSecretKeys = %v, want %v", got, want)
			}
		})
	}

	if UnusedSecretKeys("unregistered", map[string]string{"auth_method": "x"}) != nil {
		t.Error("unregistered provider should resolve all (nil)")
	}
}
