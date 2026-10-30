package upstreamhealth

import (
	"errors"
	"time"
)

// ModelOperationScopes returns only the original registered model/operation
// domains for a 429 without stronger provider scope evidence. It does not turn
// a rate limit into a whole-account or credential denial. Keep this typed
// projection with the actual attempt so a later callback never rereads scope.
func (s *Snapshot) ModelOperationScopes(target TargetReference, now time.Time) ([]string, map[string]Revision, error) {
	if s.current(now) != ReadReady || !validTarget(target) {
		return nil, nil, errors.New("original health scope is unavailable")
	}
	declared := map[string]bool{}
	for _, id := range target.ScopeIDs {
		declared[id] = true
	}
	var ids []string
	revisions := map[string]Revision{}
	for _, index := range s.accountScopes[target.ProviderAccountID] {
		scope := s.wire.Scopes[index]
		if scope.Identity.Kind == "model_operation" && declared[scope.Identity.ScopeID] && scopeApplies(scope.Identity, target) {
			ids = append(ids, scope.Identity.ScopeID)
			revisions[scope.Identity.ScopeID] = scope.Revision
		}
	}
	if len(ids) == 0 {
		return nil, nil, errors.New("original model operation rate-limit scope is unavailable")
	}
	return ids, revisions, nil
}
