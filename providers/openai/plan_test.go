package openai

import (
	"encoding/base64"
	"encoding/json"
	"testing"
)

func fakeJWT(t *testing.T, claims map[string]any) string {
	t.Helper()
	b, err := json.Marshal(claims)
	if err != nil {
		t.Fatal(err)
	}
	return "x." + base64.RawURLEncoding.EncodeToString(b) + ".sig"
}

func TestDriverFor_ByPlan(t *testing.T) {
	cases := map[string]string{
		"free":       driverModelLite,
		"go":         driverModelLite,
		"GO":         driverModelLite,
		"plus":       driverModel,
		"pro":        driverModel,
		"business":   driverModel,
		"enterprise": driverModel,
	}
	for plan, want := range cases {
		tok := fakeJWT(t, map[string]any{authClaim: map[string]any{"chatgpt_plan_type": plan}})
		if got := driverFor(tok, ""); got != want {
			t.Errorf("plan %q in id_token: got %q, want %q", plan, got, want)
		}
		if got := driverFor("", tok); got != want {
			t.Errorf("plan %q: got %q, want %q", plan, got, want)
		}
	}
}

func TestDriverFor_UnreadableTokenUsesPaidModel(t *testing.T) {
	for _, tok := range []string{"", "not-a-jwt", fakeJWT(t, map[string]any{"exp": 1})} {
		if got := driverFor(tok, tok); got != driverModel {
			t.Errorf("token %q: got %q, want %q", tok, got, driverModel)
		}
	}
}

func TestSignInVisionModel(t *testing.T) {
	paid := fakeJWT(t, map[string]any{authClaim: map[string]any{"chatgpt_plan_type": "plus"}})
	free := fakeJWT(t, map[string]any{authClaim: map[string]any{"chatgpt_plan_type": "free"}})
	cases := []struct {
		requested, configured, token string
		want, retired                string
	}{
		{"", "", paid, driverModel, ""},
		{"", "", free, driverModelLite, ""},
		{"", "gpt-6.1-terra", free, "gpt-6.1-terra", ""},
		{"", "gpt-5.5", paid, driverModel, "gpt-5.5"},
		{"", "gpt-5.5", free, driverModelLite, "gpt-5.5"},
		{"gpt-5.5", "", paid, "gpt-5.5", ""}, // explicit -m is never replaced
	}
	for _, c := range cases {
		got, retired := signInVisionModel(c.requested, c.configured, c.token, "")
		if got != c.want || retired != c.retired {
			t.Errorf("(%q, %q): got (%q, %q), want (%q, %q)", c.requested, c.configured, got, retired, c.want, c.retired)
		}
	}
}

// The id_token wins when both carry a plan; the access token fills a gap.
func TestDriverFor_IDTokenFirst(t *testing.T) {
	free := fakeJWT(t, map[string]any{authClaim: map[string]any{"chatgpt_plan_type": "free"}})
	plus := fakeJWT(t, map[string]any{authClaim: map[string]any{"chatgpt_plan_type": "plus"}})
	if got := driverFor(free, plus); got != driverModelLite {
		t.Errorf("id_token free, access plus: got %q, want %q", got, driverModelLite)
	}
	if got := driverFor("", free); got != driverModelLite {
		t.Errorf("no id_token, access free: got %q, want %q", got, driverModelLite)
	}
}
