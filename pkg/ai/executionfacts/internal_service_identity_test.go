package executionfacts

import (
	"testing"

	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/proxytest"
	"github.com/higress-group/wasm-go/pkg/ai/balanceidentity"
	"github.com/higress-group/wasm-go/pkg/ai/serviceidentity"
)

func TestInternalCallReferenceRequiresActualAccountlessAuthenticationAndOriginalCall(t *testing.T) {
	_, reset := proxytest.NewHostEmulator(proxytest.NewEmulatorOption())
	defer reset()
	identity := serviceidentity.Snapshot{Service: "ai-memory", APIKeyID: "20000000-0000-4000-8000-000000000009", CredentialGeneration: "9007199254740993", TenantID: "30000000-0000-4000-8000-000000000009", PrincipalID: "40000000-0000-4000-8000-000000000009"}
	call := [2]string{InternalCallReferenceHeader, "50000000-0000-4000-8000-000000000009"}
	headers := append(identity.Headers(), call)
	if _, err := ReadInternalCallReference(headers); err == nil {
		t.Fatal("header-only service spoof accepted")
	}
	if err := serviceidentity.Publish(identity); err != nil {
		t.Fatal(err)
	}
	actual, err := ReadInternalCallReference(headers)
	if err != nil || actual.APIKeyID != identity.APIKeyID || actual.CredentialGeneration != identity.CredentialGeneration || actual.Identity != identity {
		t.Fatal("actual authenticated service identity lost", err)
	}
	for _, invalid := range [][][2]string{identity.Headers(), append(headers, call), append(headers, [2]string{balanceidentity.AccountHeader, "10000000-0000-4000-8000-000000000009"})} {
		if _, err := ReadInternalCallReference(invalid); err == nil {
			t.Fatal("missing/replayed reference or customer account accepted")
		}
	}
}
