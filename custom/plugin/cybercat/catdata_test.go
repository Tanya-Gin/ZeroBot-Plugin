package cybercat

import "testing"

func TestGetCatAPIKeyReadsProtectedEnvironmentVariable(t *testing.T) {
	t.Setenv("THECATAPI_KEY", "test-key-from-env")
	if got := getCatAPIKey(); got != "test-key-from-env" {
		t.Fatalf("getCatAPIKey() = %q, want value from THECATAPI_KEY", got)
	}
}

func TestGetCatAPIKeyIsEmptyWhenEnvironmentVariableIsUnset(t *testing.T) {
	t.Setenv("THECATAPI_KEY", "")
	if got := getCatAPIKey(); got != "" {
		t.Fatalf("getCatAPIKey() = %q, want empty value when THECATAPI_KEY is unset", got)
	}
}
