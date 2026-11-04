package reniec

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"
)

func TestSharedSyntheticIdentities(t *testing.T) {
	var cases struct {
		Records []Identity `json:"identity_records"`
		Cases   []struct {
			ID, DNI        string
			Found, Invalid bool
		} `json:"identity_cases"`
	}
	body, err := os.ReadFile("../internal/conformance/cases.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(body, &cases); err != nil {
		t.Fatal(err)
	}
	service, err := NewMockIdentityService(cases.Records)
	if err != nil {
		t.Fatal(err)
	}
	var _ IdentityService = service
	for _, c := range cases.Cases {
		t.Run(c.ID, func(t *testing.T) {
			result, err := service.Lookup(context.Background(), IdentityQuery{DNI: c.DNI})
			if c.Invalid {
				if err == nil {
					t.Fatal("invalid DNI accepted")
				}
				return
			}
			if !c.Found {
				if !errors.Is(err, ErrNotFound) {
					t.Fatalf("got %v", err)
				}
				return
			}
			if err != nil || result != cases.Records[0] {
				t.Fatalf("wrong synthetic identity: %#v, %v", result, err)
			}
		})
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := service.Lookup(ctx, IdentityQuery{DNI: "00000001"}); !errors.Is(err, context.Canceled) {
		t.Fatal("cancellation ignored")
	}
	if _, err := NewMockIdentityService(append(cases.Records, cases.Records[0])); err == nil {
		t.Fatal("duplicate record accepted")
	}
}
