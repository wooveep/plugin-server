import json
from pathlib import Path
import runpy
import subprocess
import tempfile
import unittest
from unittest.mock import patch

from artifact_contract import ValidationPolicy, digest, json_bytes
from artifact_storage import ArtifactStore, StorageConflict
from plugin_catalog import select_releases, source_inputs

ROOT = Path(__file__).resolve().parents[1]
prepare = runpy.run_path(str(ROOT / 'scripts/package-plugins.py'))['prepare']


class SourceReleaseTests(unittest.TestCase):
    def setUp(self):
        temporary = tempfile.TemporaryDirectory()
        self.addCleanup(temporary.cleanup)
        self.root = Path(temporary.name)
        metadata = self.root / 'plugins/key-auth'
        metadata.mkdir(parents=True)
        (self.root / 'plugins/release').mkdir()
        (self.root / 'scripts').mkdir()
        (self.root / 'scripts/build-plugins.py').write_text('# initial builder\n')
        (metadata / 'VERSION').write_text('1.0.3\n')
        (metadata / 'spec.yaml').write_bytes((ROOT / 'tests/fixtures/schema-conversion/key-auth.yaml').read_bytes())
        (metadata / 'defaults.json').write_text('{}')
        (metadata / 'environment.json').write_text('[]')
        (metadata / 'README.md').write_text('Documented plugin.\n')
        (self.root / 'plugins/release/catalog.json').write_text(json.dumps({
            'schemaVersion': 1, 'requiredPlugins': ['key-auth'], 'plugins': [{
                'logicalId': 'key-auth', 'implementation': 'go', 'sourceDir': 'plugins/key-auth',
                'metadataDir': 'plugins/key-auth', 'declaredVersion': '1.0.3',
                'releaseEligible': True, 'artifactInputs': ['plugins/key-auth/**']}]}))
        for arguments in [('init', '-q'), ('config', 'user.name', 'Release fixture'),
                          ('config', 'user.email', 'fixture@example.test'),
                          ('config', 'commit.gpgsign', 'false'),
                          ('remote', 'add', 'origin', 'https://github.com/wooveep/plugin-server.git')]:
            subprocess.run(['git', *arguments], cwd=self.root, check=True, capture_output=True)
        self.wasm = b'\0asm\1\0\0\0'
        self.record_build()
        prepare(self.root)
        self.candidate = self.root / 'build/packages/key-auth/1.0.3'
        self.original_review = json.loads((self.candidate / 'review.json').read_bytes())
        self.original_archive = (self.candidate / 'package.tgz').read_bytes()
        policy = ValidationPolicy(provenance_receipts=frozenset((tuple(
            self.original_review[key] for key in ['source_digest', 'wasm_digest', 'content_digest']),)))
        self.store = ArtifactStore(self.root / 'dist/plugins', policy=policy)
        self.original_receipt = self.store.commit('original', self.original_archive)

    def record_build(self):
        subprocess.run(['git', 'add', 'plugins', 'scripts'], cwd=self.root, check=True, capture_output=True)
        subprocess.run(['git', 'commit', '-qm', 'source fixture'], cwd=self.root, check=True, capture_output=True)
        commit = subprocess.check_output(['git', 'rev-parse', 'HEAD'], cwd=self.root, text=True).strip()
        output = self.root / 'build/plugins/key-auth/1.0.3'
        output.mkdir(parents=True, exist_ok=True)
        (output / 'plugin.wasm').write_bytes(self.wasm)
        (output / 'build-receipt.json').write_bytes(json_bytes({
            'plugin_name': 'key-auth', 'plugin_version': '1.0.3', 'implementation': 'go',
            'source_inputs': source_inputs(self.root, select_releases(self.root)[0]),
            'source_commit': commit, 'artifact': 'build/plugins/key-auth/1.0.3/plugin.wasm',
            'sha256': digest(self.wasm), 'size_bytes': len(self.wasm), 'toolchain': 'fixture'}))
        return commit

    def test_rebuild_with_identical_payload_reuses_original_archive_and_provenance(self):
        (self.root / 'scripts/build-plugins.py').write_text('# replace disposable build output\n')
        current_commit = self.record_build()
        with patch('artifact_storage.validate_package', side_effect=AssertionError('revalidated registered package')):
            prepare(self.root)
        review = json.loads((self.candidate / 'review.json').read_bytes())
        self.assertEqual((self.candidate / 'package.tgz').read_bytes(), self.original_archive)
        self.assertEqual(review['source_commit'], self.original_review['source_commit'])
        self.assertEqual(review['current_build_commit'], current_commit)
        self.assertNotEqual(review['current_build_commit'], review['source_commit'])
        self.assertEqual(review['registered_artifact_id'], self.original_receipt['artifact_id'])
        self.assertEqual(review['content_digest'], self.original_review['content_digest'])
        self.assertEqual(self.store.manifest('key-auth', '1.0.3'), self.original_receipt['manifest'])

    def test_changed_payload_or_environment_requires_a_new_version(self):
        metadata = self.root / 'plugins/key-auth'
        for name, content in [('README.md', 'Changed documentation.\n'),
                              ('defaults.json', '{"global_auth":false}'),
                              ('environment.json', '[{"name":"SETTING","kind":"setting","required":false}]')]:
            with self.subTest(member=name):
                path = metadata / name
                original = path.read_text()
                path.write_text(content)
                self.record_build()
                with self.assertRaisesRegex(StorageConflict, 'version_conflict'):
                    prepare(self.root)
                path.write_text(original)
        self.wasm += b'\0\1\0'
        self.record_build()
        with self.assertRaisesRegex(StorageConflict, 'version_conflict'):
            prepare(self.root)
        self.assertEqual(self.store.read_file('key-auth', '1.0.3', 'archive.tgz'), self.original_archive)
