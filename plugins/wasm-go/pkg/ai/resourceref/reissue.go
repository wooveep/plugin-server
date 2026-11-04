package resourceref

import "github.com/higress-group/wasm-go/pkg/ai/clientresponse"

// Reissue gives a replay fresh IDs with the original resource's provenance.
// A cached alias is resolved once; no alias-to-alias chain is persisted.
func Reissue(client Client, scope Scope, aliases []clientresponse.ResourceAlias, done func(error)) {
	var previous []clientresponse.ResourceAlias
	var next []clientresponse.ResourceAlias
	for _, alias := range aliases {
		if IsAlias(alias.Kind, alias.Original) {
			previous = append(previous, clientresponse.ResourceAlias{Kind: alias.Kind, Public: alias.Original})
			next = append(next, alias)
		}
	}
	if len(previous) == 0 {
		done(nil)
		return
	}
	Lookup(client, scope, previous, func(records []Record, err error) {
		if err != nil {
			done(err)
			return
		}
		for i := range records {
			records[i].Alias.Public = next[i].Public
		}
		Save(client, scope, records, done)
	})
}
