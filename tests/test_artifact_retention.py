import unittest
from datetime import datetime, timezone

from artifact_retention import REFERENCE_KINDS, retention_preflight


class ArtifactRetentionTests(unittest.TestCase):
    def test_default_explicit_and_all_reference_kinds_retain(self):
        self.assertEqual(retention_preflight("artifact")["action"], "retain")
        decision = {"decision_id": "d1", "artifact_id": "artifact", "revision": "3", "reference_snapshot_digest": "sha256:" + "0" * 64,
                    "references": [], "action": "delete_allowed", "decided_at": "2026-10-25T00:00:00Z", "expires_at": "2026-10-27T00:00:00Z", "decided_by": "Console"}
        now = datetime(2026, 10, 26, tzinfo=timezone.utc)
        for kind in REFERENCE_KINDS:
            decision["references"] = [{"kind": kind, "ref_id": "old-version", "retention_reason": "still needed"}]
            self.assertEqual(retention_preflight("artifact", decision, current_revision="3", now=now)["reason"], "referenced_artifact")
        decision["references"] = []
        self.assertEqual(retention_preflight("artifact", decision, current_revision="2", now=now)["reason"], "stale_or_expired_decision")
        self.assertEqual(retention_preflight("artifact", decision, current_revision="3", now=now)["reason"], "reference_coordinator_reservation_required")
        self.assertEqual(retention_preflight("artifact", decision, current_revision="3", now=datetime(2026, 10, 28, tzinfo=timezone.utc))["action"], "retain")
