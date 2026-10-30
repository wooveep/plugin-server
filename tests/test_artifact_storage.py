import gzip
import multiprocessing
import os
import tempfile
import threading
import unittest
from unittest.mock import patch
from concurrent.futures import ThreadPoolExecutor
from pathlib import Path

from artifact_contract import ArtifactError, validate_document
from artifact_storage import ArtifactStore, StorageConflict, StorageUnavailable
from test_artifact_contract import example_archive, example_payload


def process_commit(root, upload_id, raw, barrier, queue, crash_phase=None):
    def fault(phase):
        if phase == crash_phase:
            os._exit(71)
    try:
        store = ArtifactStore(Path(root), fault=fault)
        if barrier:
            barrier.wait(timeout=10)
        result = store.commit(upload_id, raw)
        if queue:
            queue.put(("committed", result["artifact_id"]))
    except StorageConflict as exc:
        if queue:
            queue.put((exc.code, None))


class ArtifactStorageTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.root = Path(self.temp.name) / "plugins"
        self.store = ArtifactStore(self.root, public_base_url="http://127.0.0.1:8080")

    def tearDown(self):
        self.temp.cleanup()

    def test_commit_readback_and_recompression_preserve_first_manifest(self):
        raw = example_archive()
        first = self.store.commit("upload-1", raw)
        validate_document(__import__("artifact_contract").json_bytes(first["manifest"]), "manifest")
        self.assertEqual(first, self.store.query("upload-1"))
        recompressed = gzip.compress(gzip.decompress(raw), mtime=12)
        second = self.store.commit("upload-2", recompressed)
        self.assertEqual(first["manifest"], second["manifest"])
        self.assertEqual(self.store.read_file("test-plugin", "1.0.0", "plugin.wasm"), example_payload()["plugin.wasm"])
        self.assertEqual(self.store.read_file("test-plugin", "1.0.0", "archive.tgz"), raw)
        self.assertIsNone(self.store.read_file("test-plugin", "1.0.0", "../state.sqlite3"))

    def test_version_and_idempotency_conflicts_never_change_old_files(self):
        first = self.store.commit("upload-1", example_archive())
        payload = example_payload()
        payload["docs/README.md"] = b"Changed content"
        with self.assertRaises(StorageConflict) as error:
            self.store.commit("upload-1", example_archive(payload))
        self.assertEqual(error.exception.code, "idempotency_conflict")
        with self.assertRaises(StorageConflict) as error:
            self.store.commit("upload-2", example_archive(payload))
        self.assertEqual(error.exception.code, "version_conflict")
        self.assertEqual(self.store.query("upload-1"), first)
        self.assertEqual(self.store.query("upload-2")["error"], "version_conflict")
        self.assertEqual(self.store.manifest("test-plugin", "1.0.0"), first["manifest"])

    def test_legacy_version_cannot_be_overwritten(self):
        old = self.root / "test-plugin/1.0.0"
        old.mkdir(parents=True)
        (old / "plugin.wasm").write_bytes(b"legacy bytes")
        with self.assertRaises(StorageConflict):
            self.store.commit("new-full-package", example_archive())
        self.assertEqual((old / "plugin.wasm").read_bytes(), b"legacy bytes")

    def test_invalid_request_has_durable_rejected_query(self):
        with self.assertRaises(ArtifactError):
            self.store.commit("invalid-1", b"invalid archive")
        query = ArtifactStore(self.root).query("invalid-1")
        self.assertEqual(query["state"], "rejected")
        self.assertNotIn("content_digest", query)
        with self.assertRaises(StorageConflict):
            self.store.commit("invalid-1", example_archive())

    def test_threads_compete_different_content_only_one_wins(self):
        barrier = threading.Barrier(4)
        def contender(index):
            payload = example_payload()
            payload["docs/README.md"] = str(index).encode()
            raw = example_archive(payload)
            barrier.wait(timeout=5)
            try:
                return self.store.commit("thread-" + str(index), raw)["state"]
            except StorageConflict as exc:
                return exc.code
        with ThreadPoolExecutor(max_workers=4) as pool:
            results = list(pool.map(contender, range(4)))
        self.assertEqual(results.count("committed"), 1)
        self.assertEqual(results.count("version_conflict"), 3)

    def test_independent_processes_share_posix_commit_lock(self):
        context = multiprocessing.get_context("spawn")
        barrier, queue = context.Barrier(3), context.Queue()
        processes = []
        for index in range(3):
            payload = example_payload()
            payload["docs/README.md"] = ("process-" + str(index)).encode()
            process = context.Process(target=process_commit, args=(str(self.root), "process-" + str(index), example_archive(payload), barrier, queue))
            process.start()
            processes.append(process)
        for process in processes:
            process.join(timeout=15)
            self.assertEqual(process.exitcode, 0)
        results = [queue.get(timeout=2)[0] for _ in processes]
        queue.close()
        self.assertEqual(results.count("committed"), 1)
        self.assertEqual(results.count("version_conflict"), 2)

    def test_process_exit_before_and_after_publish_recovers_by_same_key(self):
        for phase in ("before_rename", "after_rename", "after_commit"):
            with self.subTest(phase=phase):
                root = Path(self.temp.name) / phase
                context = multiprocessing.get_context("spawn")
                process = context.Process(target=process_commit, args=(str(root), phase, example_archive(), None, None, phase))
                process.start()
                process.join(timeout=15)
                self.assertEqual(process.exitcode, 71)
                recovered = ArtifactStore(root)
                receipt = recovered.query(phase)
                self.assertEqual(receipt["state"], "failed" if phase == "before_rename" else "committed")
                repeated = recovered.commit(phase, example_archive())
                self.assertEqual(repeated["state"], "committed")
                if receipt["state"] == "committed":
                    self.assertEqual(receipt, repeated)

    def test_storage_fault_preserves_old_version_and_queryable_intent(self):
        self.store.commit("old", example_archive())
        def fault(phase):
            if phase == "before_rename":
                raise OSError("simulated disk failure")
        failing = ArtifactStore(self.root, fault=fault)
        with self.assertRaises(StorageUnavailable):
            failing.commit("new", example_archive(version="1.0.1"))
        self.assertIsNone(self.store.manifest("test-plugin", "1.0.1"))
        self.assertEqual(self.store.query("new")["state"], "failed")
        self.assertEqual(self.store.query("old")["state"], "committed")

    def test_get_waits_for_whole_commit_and_corruption_fails_closed(self):
        entered, release = threading.Event(), threading.Event()
        def fault(phase):
            if phase == "after_rename":
                entered.set()
                if not release.wait(timeout=5):
                    raise OSError("test timed out")
        writer = ArtifactStore(self.root, fault=fault)
        with ThreadPoolExecutor(max_workers=2) as pool:
            committed = pool.submit(writer.commit, "blocked", example_archive())
            self.assertTrue(entered.wait(timeout=3))
            read = pool.submit(self.store.read_file, "test-plugin", "1.0.0", "docs/README.md")
            self.assertFalse(read.done())
            release.set()
            self.assertEqual(committed.result(timeout=5)["state"], "committed")
            self.assertEqual(read.result(timeout=5), example_payload()["docs/README.md"])
        (self.root / "test-plugin/1.0.0/plugin.wasm").write_bytes(b"tampered")
        with self.assertRaises(StorageUnavailable):
            self.store.read_file("test-plugin", "1.0.0", "plugin.wasm")

    def test_registered_seed_reuses_validation_and_preserves_published_identity(self):
        raw = example_archive()
        first = self.store.commit("uploaded", raw)
        destination = ArtifactStore(Path(self.temp.name) / "runtime/plugins")
        with patch("artifact_storage.validate_package", side_effect=AssertionError("must only validate on upload")):
            self.assertEqual(destination.seed_registered(self.root, self.store.list_manifests()),
                             {"copied": [first["artifact_id"]], "retained": []})
            self.assertEqual(destination.list_metadata()[0]["manifest"], first["manifest"])
            self.assertEqual(destination.seed_registered(self.root, self.store.list_manifests()),
                             {"copied": [], "retained": [first["artifact_id"]]})
        self.assertEqual(destination.read_file("test-plugin", "1.0.0", "archive.tgz"), raw)

    def test_registered_seed_keeps_destination_id_and_rejects_version_changes(self):
        self.store.commit("image-upload", example_archive())
        destination = ArtifactStore(Path(self.temp.name) / "runtime/plugins")
        published = destination.commit("published", example_archive())
        with patch("artifact_storage.validate_package", side_effect=AssertionError("unexpected package revalidation")):
            destination.seed_registered(self.root, self.store.list_manifests())
        self.assertEqual(destination.manifest("test-plugin", "1.0.0"), published["manifest"])
        other = ArtifactStore(Path(self.temp.name) / "other/plugins")
        payload = example_payload()
        payload["docs/README.md"] = b"changed version content"
        other.commit("other", example_archive(payload))
        with self.assertRaises(StorageConflict) as failure:
            destination.seed_registered(other.root, other.list_manifests())
        self.assertEqual(failure.exception.code, "version_conflict")
        self.assertEqual(destination.manifest("test-plugin", "1.0.0"), published["manifest"])

    def test_registered_seed_recovers_copy_faults_without_revalidating_package(self):
        uploaded = self.store.commit("image-upload", example_archive())
        for phase in ["before_rename", "after_rename"]:
            with self.subTest(phase=phase):
                def fault(current):
                    if current == phase:
                        raise OSError("copy interrupted")
                root = Path(self.temp.name) / phase / "plugins"
                destination = ArtifactStore(root, fault=fault)
                with patch("artifact_storage.validate_package", side_effect=AssertionError("unexpected package revalidation")):
                    with self.assertRaises(StorageUnavailable):
                        destination.seed_registered(self.root, self.store.list_manifests())
                    resumed = ArtifactStore(root)
                    resumed.seed_registered(self.root, self.store.list_manifests())
                self.assertEqual(resumed.manifest("test-plugin", "1.0.0"), uploaded["manifest"])


if __name__ == "__main__":
    unittest.main()
