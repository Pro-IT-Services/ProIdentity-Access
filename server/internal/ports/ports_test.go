package ports

import (
	"reflect"
	"testing"
)

func TestNormalizeStripsRepeatedLabels(t *testing.T) {
	got := Normalize("ports: ports: 53, 80,135-139")
	want := "53,80,135-139"
	if got != want {
		t.Fatalf("Normalize() = %q, want %q", got, want)
	}
}

func TestNormalizeAllPortsIsEmpty(t *testing.T) {
	if got := Normalize("all ports"); got != "" {
		t.Fatalf("Normalize() = %q, want empty", got)
	}
}

func TestParseSpecsConvertsRanges(t *testing.T) {
	got, err := ParseSpecs("ports: 1-3387,3390-65535")
	if err != nil {
		t.Fatalf("ParseSpecs returned error: %v", err)
	}
	want := []string{"1:3387", "3390:65535"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ParseSpecs() = %#v, want %#v", got, want)
	}
}

func TestValidateRejectsInvalidPorts(t *testing.T) {
	for _, tc := range []string{"0", "65536", "3389-1", "abc", "1-2-3"} {
		if err := Validate(tc); err == nil {
			t.Fatalf("Validate(%q) returned nil error", tc)
		}
	}
}
