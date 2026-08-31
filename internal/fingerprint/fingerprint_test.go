package fingerprint

import (
	"testing"

	"github.com/InfraGuard-Labs/logquiet/internal/severity"
)

func TestSameInputSameFingerprint(t *testing.T) {
	a := Of(severity.Info, "User [NUM] connected from [IP]")
	b := Of(severity.Info, "User [NUM] connected from [IP]")
	if a != b {
		t.Fatalf("identical (severity, template) pairs produced different fingerprints")
	}
}

func TestDifferentSeveritySameTextDiffers(t *testing.T) {
	a := Of(severity.Info, "connection pool active")
	b := Of(severity.Error, "connection pool active")
	if a == b {
		t.Fatalf("same template at different severities should not collide")
	}
}

func TestDifferentTemplateDiffers(t *testing.T) {
	a := Of(severity.Info, "User [NUM] connected from [IP]")
	b := Of(severity.Info, "User [NUM] disconnected from [IP]")
	if a == b {
		t.Fatalf("different templates collided")
	}
}
