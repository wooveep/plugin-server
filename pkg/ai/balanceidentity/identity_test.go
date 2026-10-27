package balanceidentity

import "testing"

func TestReadRequiresAnExactUniqueNativeIdentity(t *testing.T) {
	valid := Snapshot{AccountID: "00000000-0000-0000-0000-000000000002", APIKeyID: "00000000-0000-0000-0000-000000000003", CredentialGeneration: "9007199254740993"}
	actual, err := Read(valid.Headers())
	if err != nil || actual != valid {
		t.Fatalf("native identity lost: %v", err)
	}
	for _, generation := range []string{"", "0", "-1", "+1", "01", "1.0", "9223372036854775808"} {
		invalid := valid
		invalid.CredentialGeneration = generation
		if invalid.Validate() == nil {
			t.Errorf("accepted invalid generation %q", generation)
		}
	}
	for _, id := range []string{"", "00000000-0000-0000-0000-000000000000", "CLIENT/WALLET", "00000000-0000-0000-0000-00000000000A"} {
		invalid := valid
		invalid.AccountID = id
		if invalid.Validate() == nil {
			t.Errorf("accepted invalid account %q", id)
		}
	}
	if _, err := Read(append(valid.Headers(), [2]string{AccountHeader, valid.AccountID})); err == nil {
		t.Fatal("accepted ambiguous duplicate native identity")
	}
}

func TestNativeCallerRequiresPairedOriginalIdentities(t *testing.T) {
	original := Snapshot{AccountID: "00000000-0000-0000-0000-000000000002", APIKeyID: "00000000-0000-0000-0000-000000000003", CredentialGeneration: "9007199254740993", TenantID: "00000000-0000-0000-0000-000000000005", PrincipalID: "00000000-0000-0000-0000-000000000006"}
	actual, err := Read(original.Headers())
	if err != nil || actual != original {
		t.Fatalf("caller identity changed: %v", err)
	}
	if _, err := Read(append(original.Headers(), [2]string{PrincipalHeader, original.PrincipalID})); err == nil {
		t.Fatal("duplicate principal accepted")
	}
	invalid := original
	invalid.PrincipalID = ""
	if invalid.Validate() == nil {
		t.Fatal("unpaired caller identity accepted")
	}
}
