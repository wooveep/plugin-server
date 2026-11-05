// Package resourceref restores platform aliases without disclosing native IDs.
package resourceref

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"

	"github.com/higress-group/wasm-go/pkg/ai/clientresponse"
	"github.com/higress-group/wasm-go/pkg/wrapper"
	"github.com/tidwall/resp"
)

// RetentionSeconds is the user-approved default mapping lifetime of 30 days.
const RetentionSeconds = 30 * 24 * 60 * 60
const schema = "modelfusion-public-resources.v1"

// A native parallel function batch needs both item and call IDs, plus its
// response ID. Full histories must not be truncated at the tool count itself.
const maxReferences = 4096

type Scope struct {
	Tenant, Principal, SalesAsset string
	Passthrough                   bool
}

// Owner is a native resource boundary, independent of a publication revision.
// It deliberately contains no credential or credential generation.
type Owner struct {
	Provider string `json:"provider"`
	Account  string `json:"account"`
	Region   string `json:"region"`
	Model    string `json:"model"`
	Protocol string `json:"protocol"`
}

type Record struct {
	Schema      string                       `json:"schema"`
	Alias       clientresponse.ResourceAlias `json:"alias"`
	Owner       Owner                        `json:"owner"`
	Native      bool                         `json:"native"`
	Passthrough bool                         `json:"passthrough,omitempty"`
}

func NewRecord(alias clientresponse.ResourceAlias, owner Owner, native bool) Record {
	return Record{Schema: schema, Alias: alias, Owner: owner, Native: native}
}

type Client interface {
	Command([]interface{}, wrapper.RedisResponseCallback) error
}

func IsAlias(kind, value string) bool {
	prefix := map[string]string{"response": "resp_pf_", "message": "msg_pf_", "chat": "chatcmpl_pf_", "item": "item_pf_", "call": "call_pf_", "conversation": "conv_pf_"}[kind]
	if prefix == "" || !strings.HasPrefix(value, prefix) || len(value) != len(prefix)+32 {
		return false
	}
	decoded, err := hex.DecodeString(value[len(prefix):])
	return err == nil && len(decoded) == 16 && strings.ToLower(value) == value
}

func key(scope Scope, alias clientresponse.ResourceAlias) (string, error) {
	validID := IsAlias(alias.Kind, alias.Public)
	if scope.Passthrough {
		validID = nativeID(alias.Kind, alias.Public)
	}
	if scope.Tenant == "" || scope.Principal == "" || scope.SalesAsset == "" || !validID {
		return "", errors.New("invalid resource reference scope")
	}
	raw, _ := json.Marshal([]string{scope.Tenant, scope.Principal, scope.SalesAsset})
	digest := sha256.Sum256(raw)
	if scope.Passthrough {
		id := sha256.Sum256([]byte(alias.Kind + ":" + alias.Public))
		return "modelfusion:native-resources:v1:{" + hex.EncodeToString(digest[:]) + "}:" + hex.EncodeToString(id[:]), nil
	}
	return "modelfusion:public-resources:v1:{" + hex.EncodeToString(digest[:]) + "}:" + alias.Public, nil
}

func valid(record Record) bool {
	o := record.Owner
	validID := IsAlias(record.Alias.Kind, record.Alias.Public)
	if record.Passthrough {
		validID = record.Native && record.Alias.Public == record.Alias.Original && nativeID(record.Alias.Kind, record.Alias.Public)
	}
	return record.Schema == schema && validID && record.Alias.Original != "" && len(record.Alias.Original) <= 4096 && o.Provider != "" && o.Account != "" && o.Region != "" && o.Model != "" && o.Protocol != ""
}

func nativeID(kind, value string) bool {
	return (kind == "response" || kind == "item" || kind == "call" || kind == "conversation") && value != "" && len(value) <= 4096
}

// Save is atomic and acknowledged before delivery. A random collision never
// overwrites a different resource, and a failed batch never writes half a graph.
func Save(client Client, scope Scope, records []Record, done func(error)) {
	if len(records) == 0 {
		done(nil)
		return
	}
	if len(records) > maxReferences {
		done(errors.New("resource reference budget exceeded"))
		return
	}
	const script = `-- modelfusion-public-resource-write.v1
for i,k in ipairs(KEYS) do local old=redis.call('GET',k); if old and (ARGV[#KEYS+2] ~= 'native' or old ~= ARGV[i+1]) then return 0 end end
for i,k in ipairs(KEYS) do redis.call('SET',k,ARGV[i+1],'EX',ARGV[1]) end
return 1`
	command := []interface{}{"EVAL", script, len(records)}
	values := []interface{}{RetentionSeconds}
	for _, record := range records {
		k, err := key(scope, record.Alias)
		if err != nil || !valid(record) || record.Passthrough != scope.Passthrough {
			done(errors.New("invalid resource mapping"))
			return
		}
		raw, _ := json.Marshal(record)
		command = append(command, k)
		values = append(values, string(raw))
	}
	mode := "public"
	if scope.Passthrough {
		mode = "native"
	}
	values = append(values, mode)
	command = append(command, values...)
	if err := client.Command(command, func(value resp.Value) {
		if value.Type() != resp.Integer || value.Integer() != 1 {
			done(errors.New("resource mapping not acknowledged"))
			return
		}
		done(nil)
	}); err != nil {
		done(errors.New("resource mapping unavailable"))
	}
}

// Lookup does not refresh expiry and cannot reveal a foreign scope's records.
func Lookup(client Client, scope Scope, aliases []clientresponse.ResourceAlias, done func([]Record, error)) {
	if len(aliases) == 0 || len(aliases) > maxReferences {
		done(nil, errors.New("invalid resource reference count"))
		return
	}
	command := []interface{}{"MGET"}
	for _, alias := range aliases {
		k, err := key(scope, alias)
		if err != nil {
			done(nil, err)
			return
		}
		command = append(command, k)
	}
	if err := client.Command(command, func(value resp.Value) {
		if value.Type() != resp.Array || len(value.Array()) != len(aliases) {
			done(nil, errors.New("resource reference lookup unavailable"))
			return
		}
		result := make([]Record, len(aliases))
		for i, raw := range value.Array() {
			if raw.IsNull() || len(raw.String()) > 16384 || json.Unmarshal([]byte(raw.String()), &result[i]) != nil || !valid(result[i]) || result[i].Passthrough != scope.Passthrough || result[i].Alias.Kind != aliases[i].Kind || result[i].Alias.Public != aliases[i].Public {
				done(nil, errors.New("resource reference unavailable"))
				return
			}
		}
		done(result, nil)
	}); err != nil {
		done(nil, errors.New("resource reference lookup unavailable"))
	}
}
