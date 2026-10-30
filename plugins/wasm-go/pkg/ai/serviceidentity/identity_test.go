package serviceidentity

import (
	"encoding/json"
	"testing"

	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/proxytest"
	"github.com/higress-group/wasm-go/pkg/ai/balanceidentity"
)

func original() Snapshot {
	return Snapshot{Service: "ai-memory", APIKeyID: "20000000-0000-4000-8000-000000000009", CredentialGeneration: "9007199254740993", TenantID: "30000000-0000-4000-8000-000000000009", PrincipalID: "40000000-0000-4000-8000-000000000009"}
}

func TestNativeServiceRequiresAuthenticationStateNotClientHeaders(t *testing.T) {
	host, reset := proxytest.NewHostEmulator(proxytest.NewEmulatorOption())
	defer reset()
	snapshot := original()
	if _, err := Read(snapshot.Headers()); err == nil {
		t.Fatal("headers alone became service authentication")
	}
	if err := Publish(snapshot); err != nil {
		t.Fatal(err)
	}
	actual, err := Read(snapshot.Headers())
	if err != nil || actual != snapshot {
		t.Fatal("original authenticated service changed", err)
	}
	if _, err := balanceidentity.Read(snapshot.Headers()); err == nil {
		t.Fatal("accountless service became a customer wallet")
	}
	for _, name := range []string{ServiceHeader, balanceidentity.APIKeyHeader, balanceidentity.GenerationHeader, balanceidentity.TenantHeader, balanceidentity.PrincipalHeader} {
		headers := snapshot.Headers()
		for i := range headers {
			if headers[i][0] == name {
				headers[i][1] = "forged"
			}
		}
		if _, err := Read(headers); err == nil {
			t.Fatal("changed authentication field accepted", name)
		}
	}
	if _, err := Read(append(snapshot.Headers(), [2]string{balanceidentity.AccountHeader, ""})); err == nil {
		t.Fatal("customer account field accepted")
	}
	raw, _ := json.Marshal(snapshot)
	if err := host.SetProperty([]string{Property}, raw); err != nil {
		t.Fatal(err)
	}
	if err := Clear(); err != nil {
		t.Fatal(err)
	}
	cleared, err := proxywasm.GetProperty([]string{Property})
	if err != nil || len(cleared) == 0 || string(cleared) == string(raw) {
		t.Fatal("clear did not replace the original state with nonempty invalid data", err)
	}
	if _, err := Read(snapshot.Headers()); err == nil {
		t.Fatal("cleared state was reused")
	}
	if err := Clear(); err != nil {
		t.Fatal("repeated clear failed", err)
	}
	next := snapshot
	next.Service = "ai-cache"
	next.APIKeyID = "20000000-0000-4000-8000-000000000008"
	if err := Publish(next); err != nil {
		t.Fatal("new authentication could not replace invalid state", err)
	}
	actual, err = Read(next.Headers())
	if err != nil || actual != next {
		t.Fatal("new authenticated identity was not retained", err)
	}
	if _, err := Read(snapshot.Headers()); err == nil {
		t.Fatal("new authentication retained old identity")
	}
}

func TestNativeServiceStateIsClosedAndGenerationIsExact(t *testing.T) {
	host, reset := proxytest.NewHostEmulator(proxytest.NewEmulatorOption())
	defer reset()
	for _, value := range []string{"0", "01", "-1", "1.0", "9223372036854775808"} {
		snapshot := original()
		snapshot.CredentialGeneration = value
		if snapshot.Validate() == nil {
			t.Fatal("invalid generation accepted", value)
		}
	}
	snapshot := original()
	raw, _ := json.Marshal(snapshot)
	var value map[string]interface{}
	_ = json.Unmarshal(raw, &value)
	value["account_id"] = nil
	corrupt, _ := json.Marshal(value)
	_ = host.SetProperty([]string{Property}, corrupt)
	if _, err := Read(snapshot.Headers()); err == nil {
		t.Fatal("service state accepted a wallet field")
	}
	_ = host.SetProperty([]string{Property}, []byte(`{"service":"ai-memory","service":"ai-cache"}`))
	if _, err := Read(snapshot.Headers()); err == nil {
		t.Fatal("duplicate state fields accepted")
	}
}
