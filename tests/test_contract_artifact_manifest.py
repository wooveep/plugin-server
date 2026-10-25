"""D0-ART-01 public seams: shape, content identity and offline bundle import."""
import sys
import unittest
from pathlib import Path

sys.path.insert(0, str(Path(__file__).parent / 'contracts'))
from artifact_contract_support import ContractError, validate_document


class ArtifactManifestContractTest(unittest.TestCase):
    def test_client_descriptor_cannot_assign_server_identity(self):
        raw = (Path(__file__).resolve().parents[1] /
               'contracts/fixtures/v1/artifact/manifests/server-field-in-descriptor.json').read_bytes()
        with self.assertRaises(ContractError) as caught:
            validate_document(raw, 'descriptor')
        self.assertEqual((caught.exception.stage, caught.exception.code), ('shape', 'E_SCHEMA_SHAPE'))


if __name__ == '__main__':
    unittest.main()


class ArtifactCorpusTest(unittest.TestCase):
    def test_all_frozen_domain_cases_match_stage_code_and_pointer(self):
        import json
        root = Path(__file__).resolve().parents[1] / 'contracts/fixtures/v1/artifact'
        catalog = json.loads((root / 'manifest-cases.json').read_bytes())
        self.assertGreater(len(catalog['cases']), 40)
        for case in catalog['cases']:
            with self.subTest(case_id=case['case_id']):
                expected = case['expected']
                try:
                    validate_document((root / case['path']).read_bytes(), case['document_kind'])
                    actual = {'verdict': 'accept', 'stage': 'complete', 'error_code': None, 'pointer': ''}
                except ContractError as error:
                    actual = {'verdict': 'reject', 'stage': error.stage, 'error_code': error.code, 'pointer': error.pointer}
                self.assertEqual(actual, expected)

    def test_projected_identity_matches_independent_golden_and_excludes_receipt_fields(self):
        import json
        root = Path(__file__).resolve().parents[1] / 'contracts/fixtures/v1/artifact'
        golden = (root / 'canonical/base.json').read_bytes()
        digests = json.loads((root / 'digests.json').read_bytes())
        for case_id in digests['equivalent_cases']:
            with self.subTest(case_id=case_id):
                result = validate_document((root / 'manifests' / (case_id + '.json')).read_bytes(),
                                           'manifest' if case_id.startswith('manifest') else 'descriptor')
                self.assertEqual(result['canonical_bytes'], golden)
                self.assertEqual(result['content_digest'], digests['base_content_digest'])
        for case_id in digests['changed_cases']:
            with self.subTest(case_id=case_id):
                result = validate_document((root / 'manifests' / (case_id + '.json')).read_bytes(), 'descriptor')
                self.assertNotEqual(result['content_digest'], digests['base_content_digest'])


class OfflineArtifactBundleTest(unittest.TestCase):
    def test_complete_export_is_immutable_and_executes_primitive_corpus(self):
        import json
        import tempfile
        from artifact_contract_support import ROOT, manifest_bundle
        from export_bundle import export_bundle
        with tempfile.TemporaryDirectory() as directory:
            destination = Path(directory) / 'artifact'
            digest = export_bundle(ROOT / 'contracts/v1/bundle-manifest.json', ROOT / 'contracts/v1/export-map.json', destination)
            bundle = manifest_bundle(destination)
            rows = bundle.run_cases()
            self.assertGreater(len(rows), 50)
            self.assertTrue(all(row['matched'] for row in rows))
            self.assertEqual(len(digest), 64)
            self.assertEqual((destination / 'bundle-manifest.json').read_bytes(),
                             (ROOT / 'contracts/v1/bundle-manifest.json').read_bytes())
            self.assertFalse(any(p.suffix in ('.py', '.js', '.sh') for p in destination.rglob('*')))
            with self.assertRaises(ContractError) as caught:
                export_bundle(ROOT / 'contracts/v1/bundle-manifest.json', ROOT / 'contracts/v1/export-map.json', destination)
            self.assertEqual(caught.exception.code, 'E_EXPORT_EXISTS')

    def test_tampered_missing_network_ref_and_duplicate_urn_packages_fail_closed(self):
        import json
        import tempfile
        from artifact_contract_support import ROOT, manifest_bundle, sha256
        from export_bundle import export_bundle
        for mutation, expected in [('tamper', 'E_DIGEST'), ('missing', 'E_FILE'),
                                   ('network', 'E_NETWORK_REF'), ('duplicate', 'E_DUPLICATE_URN')]:
            with self.subTest(mutation=mutation), tempfile.TemporaryDirectory() as directory:
                destination = Path(directory) / 'artifact'
                export_bundle(ROOT / 'contracts/v1/bundle-manifest.json', ROOT / 'contracts/v1/export-map.json', destination)
                manifest_path = destination / 'bundle-manifest.json'
                manifest = json.loads(manifest_path.read_bytes())
                entry = manifest['entries'][0]
                schema_path = destination / entry['path']
                if mutation == 'tamper':
                    schema_path.write_bytes(b'{}')
                elif mutation == 'missing':
                    schema_path.unlink()
                elif mutation == 'duplicate':
                    manifest['entries'].append(dict(entry))
                else:
                    schema = json.loads(schema_path.read_bytes())
                    schema['$defs']['Unused'] = {'$ref': 'https://blocked.invalid/schema'}
                    schema_path.write_text(json.dumps(schema))
                    entry['sha256'] = sha256(schema_path.read_bytes())
                manifest_path.write_text(json.dumps(manifest))
                with self.assertRaises(ContractError) as caught:
                    manifest_bundle(destination)
                self.assertEqual(caught.exception.code, expected)
