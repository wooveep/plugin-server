"""Default-retain policy and deletion preflight owned by ART-05.

Physical pruning requires Console's current reference coordinator and reservation.
Until that adapter is wired, a preflight can never authorize local deletion.
"""
from __future__ import annotations

from datetime import datetime, timezone
from typing import Any

from artifact_contract import ArtifactError

REFERENCE_KINDS = frozenset(("active_release", "inflight_call", "media_operation", "historical_fact", "rollback"))


def retention_preflight(artifact_id: str, decision: dict[str, Any] | None = None, *,
                        current_revision: str | None = None, now: datetime | None = None) -> dict[str, str]:
    if not artifact_id:
        raise ValueError("artifact_id is required")
    if decision is None:
        return {"artifact_id": artifact_id, "action": "retain", "reason": "default_retain"}
    required = {"decision_id", "artifact_id", "revision", "reference_snapshot_digest", "references",
                "action", "decided_at", "expires_at", "decided_by"}
    if set(decision) != required or decision["artifact_id"] != artifact_id:
        raise ArtifactError("ART_RETENTION_DECISION", "invalid retention decision")
    if decision["action"] not in ("retain", "delete_allowed") or not isinstance(decision["references"], list):
        raise ArtifactError("ART_RETENTION_DECISION", "invalid retention action/reference set")
    for ref in decision["references"]:
        if not isinstance(ref, dict) or set(ref) != {"kind", "ref_id", "retention_reason"} or ref["kind"] not in REFERENCE_KINDS or not ref["ref_id"] or not ref["retention_reason"]:
            raise ArtifactError("ART_RETENTION_DECISION", "invalid retention reference")
    try:
        expiry = datetime.fromisoformat(decision["expires_at"].replace("Z", "+00:00"))
        decided = datetime.fromisoformat(decision["decided_at"].replace("Z", "+00:00"))
        if expiry.tzinfo is None or decided.tzinfo is None or expiry <= decided:
            raise ValueError
    except (ValueError, TypeError, AttributeError) as exc:
        raise ArtifactError("ART_RETENTION_DECISION", "invalid retention lifetime") from exc
    now = now or datetime.now(timezone.utc)
    if current_revision is None or decision["revision"] != current_revision or expiry <= now:
        reason = "stale_or_expired_decision"
    elif decision["references"]:
        reason = "referenced_artifact"
    elif decision["action"] == "retain":
        reason = "explicit_retain"
    else:
        reason = "reference_coordinator_reservation_required"
    return {"artifact_id": artifact_id, "action": "retain", "reason": reason}
