"""Default-retain policy and deletion preflight owned by ART-05.

Physical pruning requires Console's authenticated reference coordinator and
reservation; the standalone preflight never authorizes local deletion.
"""
from __future__ import annotations

from datetime import datetime, timezone
from typing import Any
import re

from artifact_contract import ArtifactError

REFERENCE_KINDS = frozenset(("active_release", "inflight_call", "media_operation", "historical_fact", "rollback"))


def validate_decision(decision: dict[str, Any]) -> tuple[datetime, datetime]:
    """Validate the original IF07 decision; this does not authenticate its writer."""
    required = {"decision_id", "artifact_id", "revision", "reference_snapshot_digest", "references",
                "action", "decided_at", "expires_at", "decided_by"}
    if not isinstance(decision, dict) or set(decision) != required:
        raise ArtifactError("ART_RETENTION_DECISION", "invalid retention decision")
    for field in ("decision_id", "artifact_id", "decided_by"):
        value = decision[field]
        if not isinstance(value, str) or not 1 <= len(value.encode()) <= 200 or value.strip() != value or any(ord(c) < 32 for c in value):
            raise ArtifactError("ART_RETENTION_DECISION", "invalid retention identity")
    if not isinstance(decision["revision"], str) or not re.fullmatch(r"0|[1-9][0-9]{0,199}", decision["revision"]):
        raise ArtifactError("ART_RETENTION_DECISION", "invalid retention revision")
    if not isinstance(decision["reference_snapshot_digest"], str) or not re.fullmatch(r"sha256:[0-9a-f]{64}", decision["reference_snapshot_digest"]):
        raise ArtifactError("ART_RETENTION_DECISION", "invalid reference snapshot")
    if decision["action"] not in ("retain", "delete_allowed") or not isinstance(decision["references"], list):
        raise ArtifactError("ART_RETENTION_DECISION", "invalid retention action/reference set")
    for ref in decision["references"]:
        if not isinstance(ref, dict) or set(ref) != {"kind", "ref_id", "retention_reason"} or not isinstance(ref["kind"], str) or ref["kind"] not in REFERENCE_KINDS or not all(isinstance(ref[key], str) and ref[key].strip() for key in ("ref_id", "retention_reason")):
            raise ArtifactError("ART_RETENTION_DECISION", "invalid retention reference")
    try:
        expiry = datetime.fromisoformat(decision["expires_at"].replace("Z", "+00:00"))
        decided = datetime.fromisoformat(decision["decided_at"].replace("Z", "+00:00"))
        if expiry.tzinfo is None or decided.tzinfo is None or expiry <= decided:
            raise ValueError
    except (ValueError, TypeError, AttributeError) as exc:
        raise ArtifactError("ART_RETENTION_DECISION", "invalid retention lifetime") from exc
    return decided, expiry


def retention_preflight(artifact_id: str, decision: dict[str, Any] | None = None, *,
                        current_revision: str | None = None, now: datetime | None = None) -> dict[str, str]:
    if not artifact_id:
        raise ValueError("artifact_id is required")
    if decision is None:
        return {"artifact_id": artifact_id, "action": "retain", "reason": "default_retain"}
    _, expiry = validate_decision(decision)
    if decision["artifact_id"] != artifact_id:
        raise ArtifactError("ART_RETENTION_DECISION", "invalid retention artifact")
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
