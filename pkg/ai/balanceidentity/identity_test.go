package balanceidentity

import "testing"

func TestReadRequiresAnExactUniqueNativeIdentity(t *testing.T) {
	valid := Snapshot{"00000000-0000-0000-0000-000000000002", "00000000-0000-0000-0000-000000000003", "9007199254740993"}
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
