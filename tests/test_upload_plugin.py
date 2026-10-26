import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch

import upload_plugin
from test_artifact_contract import example_archive


class UploadPluginCliTest(unittest.TestCase):
    def test_upload_cli_delegates_same_complete_bytes_and_explicit_key(self):
        with tempfile.NamedTemporaryFile() as archive:
            archive.write(example_archive())
            archive.flush()
            with patch("upload_plugin.ArtifactClient") as client:
                client.return_value.commit.return_value = {"state": "committed"}
                result = upload_plugin.upload_plugin("http://localhost:8080", "secret-token", Path(archive.name),
                                                     "test-plugin", "1.0.0", upload_id="stable-intent")
        self.assertEqual(client.return_value.commit.call_args.args, ("stable-intent", example_archive()))
        self.assertEqual(result[0], 201)

    def test_upload_plugin_rejects_missing_file(self):
        with self.assertRaises(FileNotFoundError):
            upload_plugin.upload_plugin("http://localhost:8080", "secret", Path("missing.tgz"), "demo", "1.0.0",
                                        upload_id="missing")

    def test_upload_plugin_rejects_wrong_identity_before_network(self):
        with tempfile.NamedTemporaryFile() as archive:
            archive.write(example_archive())
            archive.flush()
            with patch("upload_plugin.ArtifactClient") as client, self.assertRaises(ValueError):
                upload_plugin.upload_plugin("http://localhost:8080", "secret", Path(archive.name), "wrong", "1.0.0",
                                            upload_id="wrong")
            client.assert_not_called()
