package upstreamhealth

import "time"

func deny(reason string, status ReadStatus) Decision {
	return Decision{Kind: Denied, Reason: reason, ControlStatus: status}
}

func (r ReadResult) DecideNewCall(candidate Candidate, now time.Time) Decision {
	if r.Status != ReadReady || r.Snapshot == nil {
		return deny("health_control_unavailable", r.Status)
	}
	return r.Snapshot.DecideNewCall(candidate, now)
}

// FilterNewCallCandidates preserves input order and one result per candidate.
// No candidate is selected, weighted, retried or silently substituted here.
func (r ReadResult) FilterNewCallCandidates(candidates []Candidate, now time.Time) []Decision {
	decisions := make([]Decision, len(candidates))
	for index, candidate := range candidates {
		decisions[index] = r.DecideNewCall(candidate, now)
	}
	return decisions
}

// DecideAcceptedOperation cannot turn a read snapshot into permission to query,
// consume or recreate an existing operation. That decision belongs to the
// original operation authority, even when generation traffic is cooling down.
func (r ReadResult) DecideAcceptedOperation(originalOperationID string) Decision {
	if !identity(originalOperationID) {
		return deny("original_operation_missing", "")
	}
	return Decision{Kind: ExternalOperationAuthorityRequired, Reason: "original_operation_authority_required"}
}

// DecideNewCall mirrors upstream_health::target_decision after validating the
// complete fresh control view. Cooling expiry requests an external recovery
// claim and never clears a Console row or allocates a local recovery lease.
func (s *Snapshot) DecideNewCall(candidate Candidate, now time.Time) Decision {
	if status := s.current(now); status != ReadReady {
		return deny("health_control_unavailable", status)
	}
	target := candidate.Target
	if !validTarget(target) {
		return deny("health_scope_missing_or_misbound", ReadReady)
	}
	if candidate.PendingLocalVeto {
		return deny("local_health_failure_pending", ReadReady)
	}
	control, found := s.Credential(target.CredentialRef)
	if !found {
		return deny("credential_control_missing", ReadReady)
	}
	if !control.Enabled || control.Identity.ProviderAccountID != target.ProviderAccountID || control.Generation != target.CredentialGeneration {
		return deny("credential_unavailable", ReadReady)
	}
	if candidate.ExpectedCredentialControlRevision != nil {
		if _, err := ParseRevision(string(*candidate.ExpectedCredentialControlRevision)); err != nil || *candidate.ExpectedCredentialControlRevision != control.Revision {
			return deny("credential_control_revision_mismatch", ReadReady)
		}
	}
	for _, id := range target.ScopeIDs {
		index, found := s.scopeIndex[id]
		if !found || !scopeApplies(s.wire.Scopes[index].Identity, target) {
			return deny("health_scope_missing_or_misbound", ReadReady)
		}
	}
	decision := Decision{Kind: Eligible, ControlStatus: ReadReady, AuthorityID: s.metadata.AuthorityID,
		SnapshotRevision: s.metadata.Revision, CredentialGeneration: control.Generation, CredentialControlRevision: control.Revision,
		ScopeVersions: make(map[string]ScopeVersion), ScopeEpochs: make(map[string]Revision)}
	var deadline time.Time
	for _, index := range s.accountScopes[target.ProviderAccountID] {
		scope := s.wire.Scopes[index]
		if !scopeApplies(scope.Identity, target) {
			continue
		}
		decision.ScopeVersions[scope.Identity.ScopeID] = ScopeVersion{Revision: scope.Revision, FailureGeneration: scope.FailureGeneration, RecoveryEpoch: scope.RecoveryEpoch}
		if scope.PermanentReason != nil {
			decision.Kind, decision.Reason = Denied, "scope_permanently_blocked"
			return decision
		}
		if scope.BlockedUntil != nil {
			blocked, _ := utc(*scope.BlockedUntil) // Strict snapshot validation already checked it.
			if blocked.After(now) {
				if blocked.After(deadline) {
					deadline = blocked
				}
			} else {
				decision.ScopeEpochs[scope.Identity.ScopeID] = scope.RecoveryEpoch
			}
		}
	}
	if !deadline.IsZero() {
		seconds := deadline.Unix() - now.Unix()
		if deadline.Nanosecond() > now.Nanosecond() {
			seconds++
		}
		if seconds < 1 {
			seconds = 1
		}
		wait := uint64(seconds)
		decision.Kind, decision.Reason, decision.RetryAfterSeconds = Denied, "scope_cooling_down", &wait
		return decision
	}
	if len(decision.ScopeEpochs) != 0 {
		decision.Kind = RecoveryRequired
	}
	return decision
}

func scopeApplies(scope ScopeIdentity, target TargetReference) bool {
	if scope.ProviderAccountID != target.ProviderAccountID || scope.RegionEndpointRef != nil && *scope.RegionEndpointRef != target.RegionEndpointRef ||
		scope.CredentialRef != nil && *scope.CredentialRef != target.CredentialRef || scope.CredentialGeneration != nil && *scope.CredentialGeneration != target.CredentialGeneration ||
		scope.UpstreamModel != nil && *scope.UpstreamModel != target.UpstreamModel || scope.OperationKind != nil && *scope.OperationKind != target.OperationKind {
		return false
	}
	if scope.Kind == "quota_group" {
		for _, id := range target.ScopeIDs {
			if id == scope.ScopeID {
				return true
			}
		}
		return false
	}
	return true
}

func validTarget(target TargetReference) bool {
	for _, value := range []string{target.TargetID, target.BindingID, target.ProviderAccountID, target.RegionEndpointRef, target.UpstreamModel, target.OperationKind, target.CredentialRef} {
		if !identity(value) {
			return false
		}
	}
	if _, err := ParseRevision(string(target.CredentialGeneration)); err != nil || len(target.ScopeIDs) == 0 || len(target.ScopeIDs) > 128 {
		return false
	}
	seen := make(map[string]bool, len(target.ScopeIDs))
	for _, id := range target.ScopeIDs {
		if !identity(id) || seen[id] {
			return false
		}
		seen[id] = true
	}
	return true
}
