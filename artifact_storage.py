"""Durable immutable artifact storage for a single shared POSIX filesystem.

flock serializes threads/processes; SQLite and fsync preserve upload intent.
The final directory is published by one rename, and HTTP visibility is gated by
the committed database row. Recovery finishes a renamed commit after a crash.
This does not claim coordination across independent volumes or remote replicas.
"""
from __future__ import annotations

import fcntl
import os
import re
import shutil
import sqlite3
import uuid
from contextlib import contextmanager
from datetime import datetime, timezone
from pathlib import Path
from typing import Callable
from urllib.parse import quote

from artifact_contract import (ArtifactError, ValidationPolicy, digest, json_bytes,
                               safe_path, validate_document, validate_package)
from mf_canonical import ContractError, parse_json
from artifact_retention import validate_decision

SEGMENT = re.compile(r"[A-Za-z0-9][A-Za-z0-9._+-]{0,199}\Z")
UPLOAD_ID = re.compile(r"[A-Za-z0-9][A-Za-z0-9._:-]{0,199}\Z")


class StorageConflict(Exception):
    def __init__(self, code: str):
        super().__init__(code)
        self.code = code


class StorageUnavailable(Exception):
    pass


def fsync_directory(path: Path) -> None:
    fd = os.open(path, os.O_RDONLY | os.O_DIRECTORY)
    try:
        os.fsync(fd)
    finally:
        os.close(fd)


def write_durable(path: Path, raw: bytes) -> None:
    with path.open("xb") as stream:
        stream.write(raw)
        stream.flush()
        os.fsync(stream.fileno())


class ArtifactStore:
    def __init__(self, plugins_dir: Path, *, state_dir: Path | None = None,
                 public_base_url: str = "http://localhost:8080", policy: ValidationPolicy | None = None,
                 fault: Callable[[str], None] | None = None):
        self.root = Path(plugins_dir).resolve()
        self.state = Path(state_dir).resolve() if state_dir else self.root.parent / ("." + self.root.name + "-artifact-state")
        if self.state == self.root or self.state.is_relative_to(self.root) or self.root.is_relative_to(self.state):
            raise ValueError("artifact state/staging and downloadable files must be disjoint")
        if not re.fullmatch(r"https?://[^\s]+", public_base_url):
            raise ValueError("public_base_url must be an explicit HTTP(S) URL")
        self.base_url = public_base_url.rstrip("/")
        self.policy = policy or ValidationPolicy()
        self.fault = fault or (lambda phase: None)
        self.root.mkdir(parents=True, exist_ok=True)
        self.state.mkdir(parents=True, exist_ok=True, mode=0o700)
        self.staging = self.state / "staging"
        self.staging.mkdir(exist_ok=True, mode=0o700)
        if self.root.stat().st_dev != self.staging.stat().st_dev:
            raise ValueError("atomic commit requires staging and payloads on the same filesystem")
        with self._locked() as db:
            db.executescript("""
                CREATE TABLE IF NOT EXISTS artifacts (
                    name TEXT NOT NULL, version TEXT NOT NULL, artifact_id TEXT UNIQUE NOT NULL,
                    content_digest TEXT NOT NULL, manifest BLOB NOT NULL,
                    PRIMARY KEY(name, version));
                CREATE TABLE IF NOT EXISTS uploads (
                    upload_id TEXT PRIMARY KEY, name TEXT, version TEXT,
                    content_digest TEXT NOT NULL, state TEXT NOT NULL,
                    error TEXT, artifact_id TEXT);
            """)
            self._recover(db)

            db.executescript("""
                CREATE TABLE IF NOT EXISTS retention_commands (
                    decision_id TEXT PRIMARY KEY, artifact_id TEXT NOT NULL,
                    original BLOB NOT NULL, receipt BLOB NOT NULL, state TEXT NOT NULL,
                    quarantine TEXT NOT NULL);
            """)

    @contextmanager
    def _locked(self):
        with (self.state / "store.lock").open("a+b") as lock:
            fcntl.flock(lock, fcntl.LOCK_EX)
            db = sqlite3.connect(self.state / "state.sqlite3", timeout=30, isolation_level=None)
            db.row_factory = sqlite3.Row
            try:
                db.execute("PRAGMA synchronous=FULL")
                yield db
            finally:
                db.close()
                fcntl.flock(lock, fcntl.LOCK_UN)

    def _recover(self, db):
        """Only persisted intents can authorize completing a renamed commit."""
        for row in db.execute("SELECT * FROM uploads WHERE state='validating'").fetchall():
            final = self.root / row["name"] / row["version"]
            if not final.exists():
                db.execute("UPDATE uploads SET state='failed',error='storage_unavailable' WHERE upload_id=?", (row["upload_id"],))
                continue
            try:
                manifest_raw = (final / "manifest.json").read_bytes()
                manifest = validate_document(manifest_raw, "manifest")["document"]
                if (manifest["plugin_name"], manifest["plugin_version"], manifest["content_digest"]) != (row["name"], row["version"], row["content_digest"]):
                    raise StorageUnavailable("recovery identity mismatch")
                # The upload was validated before its durable intent and atomic
                # rename. Recovery completes that intent from registered metadata;
                # ordinary reads still check the requested member's digest.
                self._record_committed(db, row["upload_id"], manifest_raw)
            except (OSError, ContractError) as exc:
                raise StorageUnavailable("persisted artifact could not be recovered") from exc

    @staticmethod
    def _record_committed(db, upload_id, manifest_raw):
        manifest = parse_json(manifest_raw)
        db.execute("BEGIN IMMEDIATE")
        try:
            db.execute("INSERT OR IGNORE INTO artifacts VALUES (?,?,?,?,?)", (manifest["plugin_name"], manifest["plugin_version"], manifest["artifact_id"], manifest["content_digest"], manifest_raw))
            existing = db.execute("SELECT content_digest,manifest FROM artifacts WHERE name=? AND version=?", (manifest["plugin_name"], manifest["plugin_version"])).fetchone()
            if existing["content_digest"] != manifest["content_digest"] or existing["manifest"] != manifest_raw:
                raise StorageConflict("version_conflict")
            db.execute("UPDATE uploads SET state='committed',error=NULL,artifact_id=? WHERE upload_id=?", (manifest["artifact_id"], upload_id))
            db.execute("COMMIT")
        except BaseException:
            db.execute("ROLLBACK")
            raise

    def commit(self, upload_id: str, raw: bytes, *, created_by: str = "artifact-service",
               expected_identity: tuple[str, str] | None = None) -> dict:
        if not UPLOAD_ID.fullmatch(upload_id) or not created_by or len(created_by) > 200:
            raise ArtifactError("ART_UPLOAD_ID", "invalid upload/principal identity")
        try:
            validated = validate_package(raw, self.policy)
        except ContractError as exc:
            # Determinate invalid requests have durable, content-bound query results.
            with self._locked() as db:
                previous = db.execute("SELECT * FROM uploads WHERE upload_id=?", (upload_id,)).fetchone()
                if previous is not None and previous["content_digest"] != digest(raw):
                    raise StorageConflict("idempotency_conflict") from exc
                if previous is None:
                    db.execute("INSERT INTO uploads VALUES (?,NULL,NULL,?,'rejected',?,NULL)", (upload_id, digest(raw), exc.code))
            raise
        desc = validated.descriptor
        name, version = desc["plugin_name"], desc["plugin_version"]
        if expected_identity is not None and expected_identity != (name, version):
            raise ArtifactError("ART_IDENTITY", "upload URL and package identity differ")
        if not SEGMENT.fullmatch(name) or not SEGMENT.fullmatch(version):
            raise ArtifactError("ART_PATH", "plugin identity exceeds supported path budget")
        stage = None
        renamed = False
        try:
            with self._locked() as db:
                self._recover(db)
                previous = db.execute("SELECT * FROM uploads WHERE upload_id=?", (upload_id,)).fetchone()
                if previous is not None and previous["content_digest"] != validated.content_digest:
                    raise StorageConflict("idempotency_conflict")
                if previous is not None and previous["state"] == "rejected":
                    raise StorageConflict(previous["error"] or "idempotency_conflict")
                existing = db.execute("SELECT * FROM artifacts WHERE name=? AND version=?", (name, version)).fetchone()
                if existing is not None:
                    if db.execute("SELECT 1 FROM retention_commands WHERE artifact_id=? AND state IN ('unknown','deleted')", (existing["artifact_id"],)).fetchone():
                        raise StorageConflict("artifact_retention_reserved")
                    if existing["content_digest"] != validated.content_digest:
                        db.execute("INSERT INTO uploads VALUES (?,?,?,?,'rejected','version_conflict',NULL) ON CONFLICT(upload_id) DO UPDATE SET state='rejected',error='version_conflict'", (upload_id, name, version, validated.content_digest))
                        raise StorageConflict("version_conflict")
                    db.execute("INSERT INTO uploads VALUES (?,?,?,?,'committed',NULL,?) ON CONFLICT(upload_id) DO UPDATE SET state='committed',error=NULL,artifact_id=excluded.artifact_id", (upload_id, name, version, validated.content_digest, existing["artifact_id"]))
                    return self._receipt(db, upload_id)
                final = self.root / name / version
                if final.exists():
                    # Legacy versions are retained. A new full package cannot replace them.
                    db.execute("INSERT INTO uploads VALUES (?,?,?,?,'rejected','version_conflict',NULL) ON CONFLICT(upload_id) DO UPDATE SET state='rejected',error='version_conflict'", (upload_id, name, version, validated.content_digest))
                    raise StorageConflict("version_conflict")
                if previous is None:
                    db.execute("INSERT INTO uploads VALUES (?,?,?,?,'validating',NULL,NULL)", (upload_id, name, version, validated.content_digest))
                else:
                    db.execute("UPDATE uploads SET state='validating',error=NULL WHERE upload_id=?", (upload_id,))
                stage = self.staging / str(uuid.uuid4())
                stage.mkdir(mode=0o700)
                manifest = {**desc, "artifact_id": str(uuid.uuid4()), "content_digest": validated.content_digest,
                            "archive_digest": validated.archive_digest, "created_at": datetime.now(timezone.utc).isoformat().replace("+00:00", "Z"),
                            "created_by": created_by, "download_url": self.base_url + "/plugins/" + quote(name, safe="") + "/" + quote(version, safe="") + "/" + quote(desc["wasm_path"], safe="/")}
                manifest_raw = json_bytes(manifest)
                validate_document(manifest_raw, "manifest")
                for path, content in validated.payload.items():
                    target = stage / path
                    target.parent.mkdir(parents=True, exist_ok=True)
                    write_durable(target, content)
                write_durable(stage / "manifest.json", manifest_raw)
                write_durable(stage / "archive.tgz", raw)
                # Preserve legacy metadata reads without participating in content identity.
                wasm = validated.payload[desc["wasm_path"]]
                write_durable(stage / "metadata.txt", f"Plugin Name: {name}\nVersion: {version}\nSize: {len(wasm)} bytes\nSHA256: {digest(wasm)}\n".encode())
                for directory, _, _ in os.walk(stage, topdown=False):
                    fsync_directory(Path(directory))
                self.fault("before_rename")
                final.parent.mkdir(parents=True, exist_ok=True)
                fsync_directory(self.root)
                os.rename(stage, final)
                renamed = True
                fsync_directory(final.parent)
                fsync_directory(self.staging)
                self.fault("after_rename")
                self._record_committed(db, upload_id, manifest_raw)
                self.fault("after_commit")
                return self._receipt(db, upload_id)
        except (OSError, sqlite3.Error) as exc:
            # A renamed package may already be durable. Never declare it failed:
            # query/replay of the same intent completes recovery under the lock.
            raise StorageUnavailable("commit outcome must be queried with the same upload_id") from exc
        finally:
            if stage is not None and not renamed:
                shutil.rmtree(stage, ignore_errors=True)

    def _receipt(self, db, upload_id):
        row = db.execute("SELECT * FROM uploads WHERE upload_id=?", (upload_id,)).fetchone()
        if row is None:
            return None
        result = {"upload_id": upload_id, "state": row["state"]}
        for key in ("artifact_id", "content_digest", "error"):
            if key == "content_digest" and row["name"] is None:
                continue
            if row[key] is not None:
                result[key] = row[key]
        if row["state"] == "committed":
            artifact = db.execute("SELECT manifest FROM artifacts WHERE artifact_id=?", (row["artifact_id"],)).fetchone()
            result["manifest"] = parse_json(artifact["manifest"])
        return result

    def query(self, upload_id: str) -> dict | None:
        if not UPLOAD_ID.fullmatch(upload_id):
            raise ArtifactError("ART_UPLOAD_ID", "invalid upload identity")
        with self._locked() as db:
            self._recover(db)
            return self._receipt(db, upload_id)

    def manifest(self, name: str, version: str) -> dict | None:
        if not SEGMENT.fullmatch(name) or not SEGMENT.fullmatch(version):
            return None
        with self._locked() as db:
            self._recover(db)
            row = db.execute("SELECT manifest FROM artifacts a WHERE name=? AND version=? AND NOT EXISTS(SELECT 1 FROM retention_commands r WHERE r.artifact_id=a.artifact_id AND r.state IN ('unknown','deleted'))", (name, version)).fetchone()
            return parse_json(row["manifest"]) if row else None

    def read_file(self, name: str, version: str, path: str) -> bytes | None:
        if not safe_path(path):
            return None
        manifest = self.manifest(name, version)
        if manifest is None:
            return None
        return self._read_member(manifest, path)

    def _read_member(self, manifest: dict, path: str) -> bytes | None:
        allowed = {entry["path"]: entry for entry in manifest["files"]}
        if path not in allowed and path not in ("package.json", "metadata.txt", "archive.tgz"):
            return None
        target = self.root / manifest["plugin_name"] / manifest["plugin_version"] / path
        if target.is_symlink() or not target.resolve().is_relative_to(self.root):
            raise StorageUnavailable("unexpected file alias")
        try:
            raw = target.read_bytes()
        except OSError as exc:
            raise StorageUnavailable("committed artifact member is unavailable") from exc
        if path in allowed and (digest(raw) != allowed[path]["sha256"] or str(len(raw)) != allowed[path]["size_bytes"]):
            raise StorageUnavailable("committed payload bytes differ")
        if path == "archive.tgz" and digest(raw) != manifest["archive_digest"]:
            raise StorageUnavailable("committed archive bytes differ")
        return raw

    def list_manifests(self) -> list[dict]:
        """Read committed versions for a producer inventory; never select latest."""
        with self._locked() as db:
            self._recover(db)
            return [parse_json(row["manifest"]) for row in db.execute("SELECT manifest FROM artifacts a WHERE NOT EXISTS(SELECT 1 FROM retention_commands r WHERE r.artifact_id=a.artifact_id AND r.state IN ('unknown','deleted')) ORDER BY name,version")]

    def list_metadata(self) -> list[dict]:
        """Batch registered metadata without reading Wasm or package archives."""
        result = []
        for manifest in self.list_manifests():
            schema = parse_json(self._read_member(manifest, manifest["config_schema_path"]))
            result.append({"manifest": manifest, "metadata": schema.get("x-higress")})
        return result

    def seed_registered(self, source_root: Path, manifests: list[dict]) -> dict:
        """Copy an already validated local store, preserving published identities.

        This operator-only port consumes a trusted image/store snapshot. New HTTP
        uploads must always use commit(), which performs full package validation.
        """
        copied, retained = [], []
        source_root = Path(source_root).resolve()
        for manifest in manifests:
            name, version = manifest["plugin_name"], manifest["plugin_version"]
            manifest_raw = json_bytes(manifest)
            stage = None
            renamed = False
            try:
                with self._locked() as db:
                    self._recover(db)
                    existing = db.execute("SELECT * FROM artifacts WHERE name=? AND version=?", (name, version)).fetchone()
                    if existing is not None:
                        if existing["content_digest"] != manifest["content_digest"]:
                            raise StorageConflict("version_conflict")
                        if db.execute("SELECT 1 FROM retention_commands WHERE artifact_id=? AND state IN ('unknown','deleted')", (existing["artifact_id"],)).fetchone():
                            raise StorageConflict("artifact_retention_reserved")
                        retained.append(existing["artifact_id"])
                        continue
                    final = self.root / name / version
                    if final.exists():
                        raise StorageConflict("version_conflict")
                    upload_id = "seed-" + manifest["artifact_id"]
                    previous = db.execute("SELECT content_digest FROM uploads WHERE upload_id=?", (upload_id,)).fetchone()
                    if previous is not None and previous["content_digest"] != manifest["content_digest"]:
                        raise StorageConflict("idempotency_conflict")
                    db.execute("INSERT INTO uploads VALUES (?,?,?,?,'validating',NULL,NULL) "
                               "ON CONFLICT(upload_id) DO UPDATE SET state='validating',error=NULL",
                               (upload_id, name, version, manifest["content_digest"]))
                    stage = self.staging / str(uuid.uuid4())
                    stage.mkdir(mode=0o700)
                    for path in [*(entry["path"] for entry in manifest["files"]),
                                 "package.json", "archive.tgz", "metadata.txt"]:
                        target = stage / path
                        target.parent.mkdir(parents=True, exist_ok=True)
                        write_durable(target, (source_root / name / version / path).read_bytes())
                    write_durable(stage / "manifest.json", manifest_raw)
                    for directory, _, _ in os.walk(stage, topdown=False):
                        fsync_directory(Path(directory))
                    self.fault("before_rename")
                    final.parent.mkdir(parents=True, exist_ok=True)
                    fsync_directory(self.root)
                    os.rename(stage, final)
                    renamed = True
                    fsync_directory(final.parent)
                    fsync_directory(self.staging)
                    self.fault("after_rename")
                    self._record_committed(db, upload_id, manifest_raw)
                    copied.append(manifest["artifact_id"])
            except (OSError, sqlite3.Error) as exc:
                raise StorageUnavailable("registered store seed must be resumed from its persisted metadata") from exc
            finally:
                if stage is not None and not renamed:
                    shutil.rmtree(stage, ignore_errors=True)
        return {"copied": copied, "retained": retained}

    def retain_or_delete(self, decision: dict) -> dict:
        """Consume one decision from the authenticated Console coordinator only.

        The caller owns the live reference reservation. This local journal never
        grants that authority. Replays only read the first original intent; they
        do not retry a filesystem deletion after unknown.
        """
        decided, expiry = validate_decision(decision)
        raw = json_bytes(decision)
        with self._locked() as db:
            previous = db.execute("SELECT * FROM retention_commands WHERE decision_id=?", (decision["decision_id"],)).fetchone()
            if previous is not None:
                if previous["original"] != raw:
                    raise StorageConflict("retention_idempotency_conflict")
                return self._retention_readback(db, previous)
            row = db.execute("SELECT * FROM artifacts WHERE artifact_id=?", (decision["artifact_id"],)).fetchone()
            if row is None:
                raise StorageConflict("retention_artifact_not_found")
            manifest = parse_json(row["manifest"])
            receipt = {"decision_id": decision["decision_id"], "artifact_id": row["artifact_id"],
                       "plugin_name": row["name"], "plugin_version": row["version"],
                       "content_digest": row["content_digest"], "decision_digest": digest(raw),
                       "state": "retained", "reason": "explicit_retain"}
            now = datetime.now(timezone.utc)
            other = db.execute("SELECT original,state FROM retention_commands WHERE artifact_id=?", (row["artifact_id"],)).fetchall()
            if expiry <= now or decided > now:
                receipt.update(state="refused", reason="stale_or_expired_decision")
            elif any(item["state"] in ("unknown", "deleted") or int(parse_json(item["original"])["revision"]) >= int(decision["revision"]) for item in other):
                receipt.update(state="refused", reason="stale_or_reserved_decision")
            elif decision["references"]:
                receipt["reason"] = "referenced_artifact"
            elif decision["action"] == "delete_allowed":
                receipt.update(state="unknown", reason="delete_outcome_unknown")
            quarantine = digest(decision["decision_id"].encode()).removeprefix("sha256:")
            db.execute("INSERT INTO retention_commands VALUES (?,?,?,?,?,?)", (decision["decision_id"], row["artifact_id"], raw, json_bytes(receipt), receipt["state"], quarantine))
            if receipt["state"] != "unknown":
                return receipt
            # The immutable row remains a tombstone: neither uploads nor seeding
            # can revive this exact artifact after the reserved public removal.
            final = self.root / row["name"] / row["version"]
            tombstone = self.state / ("deleted-" + quarantine)
            try:
                if final.is_symlink() or not final.resolve().is_relative_to(self.root) or not final.is_dir() or (final / "manifest.json").read_bytes() != row["manifest"]:
                    receipt.update(state="refused", reason="original_manifest_differs")
                    self._save_retention(db, receipt)
                    return receipt
                for entry in manifest["files"]:
                    member = final / entry["path"]
                    if member.is_symlink() or not member.resolve().is_relative_to(final.resolve()) or digest(member.read_bytes()) != entry["sha256"]:
                        raise StorageUnavailable("original artifact member differs")
                if digest((final / "archive.tgz").read_bytes()) != manifest["archive_digest"]:
                    raise StorageUnavailable("original archive bytes differ")
                self.fault("retention_before_rename")
                os.rename(final, tombstone)
                fsync_directory(final.parent)
                fsync_directory(self.state)
                self.fault("retention_after_rename")
                shutil.rmtree(tombstone)
                fsync_directory(self.state)
                self.fault("retention_after_delete")
            except (OSError, StorageUnavailable) as exc:
                raise StorageUnavailable("retention outcome must be queried with its original decision_id") from exc
            receipt.update(state="deleted", reason="original_files_absent")
            self._save_retention(db, receipt)
            return receipt

    @staticmethod
    def _save_retention(db, receipt):
        db.execute("UPDATE retention_commands SET receipt=?,state=? WHERE decision_id=?", (json_bytes(receipt), receipt["state"], receipt["decision_id"]))

    def _retention_readback(self, db, row) -> dict:
        receipt = parse_json(row["receipt"])
        if row["state"] != "unknown":
            return receipt
        artifact = db.execute("SELECT * FROM artifacts WHERE artifact_id=?", (row["artifact_id"],)).fetchone()
        final = self.root / artifact["name"] / artifact["version"]
        quarantine = self.state / ("deleted-" + row["quarantine"])
        # A query observes the original locations under the same process lock;
        # it never issues another rename/rmtree or changes the original intent.
        if not final.exists() and not final.is_symlink() and not quarantine.exists() and not quarantine.is_symlink():
            receipt.update(state="deleted", reason="original_files_absent")
        elif final.is_dir() and not final.is_symlink() and not quarantine.exists() and (final / "manifest.json").read_bytes() == artifact["manifest"]:
            receipt.update(state="refused", reason="original_files_still_present")
        self._save_retention(db, receipt)
        return receipt

    def retention_query(self, decision_id: str) -> dict | None:
        with self._locked() as db:
            row = db.execute("SELECT * FROM retention_commands WHERE decision_id=?", (decision_id,)).fetchone()
            return self._retention_readback(db, row) if row else None
