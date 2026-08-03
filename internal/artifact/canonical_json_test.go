package artifact

import (
	"encoding/json"
	"testing"
)

func TestCanonicalJSONHasOneIdentityForEquivalentValues(t *testing.T) {
	first, err := CanonicalJSON([]byte(` { "b": 1.0, "a": [true, null] } `))
	if err != nil {
		t.Fatal(err)
	}
	second, err := CanonicalJSON([]byte(`{"a":[true,null],"b":1}`))
	if err != nil {
		t.Fatal(err)
	}
	if string(first) != string(second) || string(first) != `{"a":[true,null],"b":1}` {
		t.Fatalf("canonical values differ: %s / %s", first, second)
	}
}

func TestCanonicalJSONPreservesTypedIntegerDecoding(t *testing.T) {
	encoded, err := CanonicalJSON([]byte(`{"size":16.0}`))
	if err != nil {
		t.Fatal(err)
	}
	var value struct {
		Size int64 `json:"size"`
	}
	if err := json.Unmarshal(encoded, &value); err != nil || value.Size != 16 {
		t.Fatalf("typed canonical integer = %#v, %v, bytes=%s", value, err, encoded)
	}
}

func TestCanonicalJSONRejectsAmbiguousInput(t *testing.T) {
	for _, input := range []string{`{"a":1,"a":2}`, `{"a":1} trailing`} {
		if _, err := CanonicalJSON([]byte(input)); err == nil {
			t.Fatalf("accepted ambiguous input %q", input)
		}
	}
}

func TestCanonicalJSONNumbersAreExactAtArbitraryMagnitude(t *testing.T) {
	values := []string{`10000000000000000000000000000000000000000`, `1e40`, `1000e37`}
	var canonical string
	for _, value := range values {
		encoded, err := CanonicalJSON([]byte(value))
		if err != nil {
			t.Fatal(err)
		}
		if canonical == "" {
			canonical = string(encoded)
		} else if string(encoded) != canonical {
			t.Fatalf("%s canonicalized to %s, want %s", value, encoded, canonical)
		}
	}
	if canonical != "1e40" {
		t.Fatalf("canonical number = %s", canonical)
	}
}
