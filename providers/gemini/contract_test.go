package gemini_test

import (
	"testing"

	"github.com/AhmedAburady/imagine-cli/providers/providertest"
)

func TestContract(t *testing.T) {
	providertest.Contract(t, "gemini")
}
