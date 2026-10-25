"""D0-ART-02 public offline package validation boundary."""
import sys
import tempfile
import unittest
from pathlib import Path

sys.path.insert(0, str(Path(__file__).parent / 'contracts'))
from validate_artifact_fixture import validate_package


class OfflinePackageTest(unittest.TestCase):
    def test_missing_archive_is_execution_error_without_fake_package_verdict(self):
        report = validate_package(Path('/missing-artifact-fixture.tgz'), case_id='missing-io')
        self.assertEqual(report['result'], 'execution_error')
        self.assertEqual(report['validation_stage'], 'execution')
        self.assertEqual(report['errors'][0]['code'], 'ART_IO')
        self.assertNotIn('normalized_content_digest', report)


if __name__ == '__main__':
    unittest.main()


class PackageFixtureCorpusTest(unittest.TestCase):
    def test_effective_archives_execute_offline_not_just_their_catalog(self):
        import json
        root = Path(__file__).resolve().parents[1] / 'contracts/fixtures/v1/artifact'
        catalog = json.loads((root / 'package-cases.json').read_bytes())
        self.assertGreater(len(catalog['cases']), 35)
        for case in catalog['cases']:
            with self.subTest(case_id=case['case_id']):
                policy = {**catalog['policy'], **case['policy_overrides']}
                actual = validate_package(root / case['archive'], case_id=case['case_id'], policy=policy)
                expected = case['expected']
                self.assertEqual(actual['result'], expected['result'])
                self.assertEqual(actual['validation_stage'], expected['validation_stage'])
                self.assertEqual(actual['errors'][0]['code'] if actual['errors'] else None, expected['error_code'])
                self.assertNotIn('SYNTHETIC_SECRET_DO_NOT_PUBLISH', json.dumps(actual))


class DeterministicArchiveTest(unittest.TestCase):
    def test_each_frozen_archive_rebuilds_exact_bytes_without_network_or_filesystem_writes(self):
        import json
        from unittest.mock import patch
        from build_artifact_fixtures import build_archive
        root = Path(__file__).resolve().parents[1] / 'contracts/fixtures/v1/artifact'
        catalog = json.loads((root / 'package-cases.json').read_bytes())
        for case in catalog['cases']:
            with self.subTest(case_id=case['case_id']):
                recipe = json.loads((root / case['recipe']).read_bytes())
                self.assertEqual(build_archive(recipe), (root / case['archive']).read_bytes())
        with tempfile.TemporaryDirectory() as directory:
            sandbox = Path(directory)
            before = list(sandbox.iterdir())
            with patch('socket.socket', side_effect=AssertionError('offline boundary')), \
                 patch('tarfile.TarFile.extract', side_effect=AssertionError('no extraction')), \
                 patch('tarfile.TarFile.extractall', side_effect=AssertionError('no extraction')), \
                 patch.object(Path, 'write_bytes', side_effect=AssertionError('no writes')), \
                 patch.object(Path, 'write_text', side_effect=AssertionError('no writes')):
                for case in catalog['cases']:
                    policy = {**catalog['policy'], **case['policy_overrides']}
                    validate_package(root / case['archive'], case_id=case['case_id'], policy=policy)
            self.assertEqual(list(sandbox.iterdir()), before)

    def test_compression_preserves_content_but_docs_and_defaults_change_it(self):
        import json
        root = Path(__file__).resolve().parents[1] / 'contracts/fixtures/v1/artifact'
        valid = validate_package(root / 'packages/package-valid.tgz', case_id='base')
        alternate = validate_package(root / 'packages/package-same-content-different-compression.tgz', case_id='alternate')
        self.assertEqual(valid['normalized_content_digest'], alternate['normalized_content_digest'])
        self.assertNotEqual(valid['archive_digest'], alternate['archive_digest'])
        for name in ('changed-docs-valid', 'changed-defaults-valid'):
            result = validate_package(root / ('packages/' + name + '.tgz'), case_id=name)
            self.assertEqual(result['result'], 'valid')
            self.assertNotEqual(result['normalized_content_digest'], valid['normalized_content_digest'])


class PackageToolFailureTest(unittest.TestCase):
    def test_dependency_and_unknown_validator_faults_cannot_match_invalid_defaults(self):
        from unittest.mock import patch
        from artifact_contract_support import ContractError, contract_registry, mf
        contract_registry()
        original = mf.check_shape
        archive = Path(__file__).resolve().parents[1] / 'contracts/fixtures/v1/artifact/packages/defaults-invalid.tgz'
        for failure, code in [(ContractError('E_DEPENDENCY', 'test dependency fault', 'tool'), 'E_DEPENDENCY'),
                              (RuntimeError('unknown validator fault'), 'ART_TOOL')]:
            def check(value, schema, registry, *args, **kwargs):
                if schema.get('$ref', '').startswith('urn:modelfusion:fixture:payload:'):
                    raise failure
                return original(value, schema, registry, *args, **kwargs)
            with self.subTest(code=code), patch.object(mf, 'check_shape', side_effect=check):
                report = validate_package(archive, case_id='dependency-guard')
                self.assertEqual((report['result'], report['validation_stage']), ('execution_error', 'execution'))
                self.assertEqual(report['errors'][0]['code'], code)
                self.assertNotIn('normalized_content_digest', report)

    def test_locked_common_anchor_and_encoded_fragment_resolve_and_unknown_anchor_rejects(self):
        import copy
        import json
        import base64
        import hashlib
        from build_artifact_fixtures import build_archive
        root = Path(__file__).resolve().parents[1] / 'contracts/fixtures/v1/artifact'
        source = json.loads((root / 'package-recipes/package-valid.json').read_bytes())
        for reference, expected in [('urn:modelfusion:contracts:v1:common#revision', 'valid'),
                                    ('urn:modelfusion:contracts:v1:common#revi%73ion', 'valid'),
                                    ('urn:modelfusion:contracts:v1:common#/%24defs/Revision', 'valid'),
                                    ('urn:modelfusion:contracts:v1:common#unknown', 'invalid'),
                                    ('#/$defs/a%252Fb', 'valid'), ('#LocalAnchor', 'valid')]:
            recipe = copy.deepcopy(source)
            descriptor = json.loads(base64.b64decode(recipe['members'][0]['base64']))
            schema = {'$schema':'https://json-schema.org/draft/2020-12/schema', 'type':'object',
                      'properties':{'revision':{'$ref':reference}}, 'required':['revision'],
                      '$defs':{'a%2Fb':{'type':'string'}, 'local':{'$anchor':'LocalAnchor','type':'string'}}}
            content = {'config.schema.json':json.dumps(schema).encode(), 'defaults.json':b'{"revision":"1"}'}
            for member in recipe['members'][1:]:
                if member['path'] in content:
                    raw = content[member['path']]
                    member['base64'] = base64.b64encode(raw).decode()
                    entry = next(f for f in descriptor['files'] if f['path'] == member['path'])
                    entry.update(size_bytes=str(len(raw)),sha256='sha256:'+hashlib.sha256(raw).hexdigest())
            recipe['members'][0]['base64'] = base64.b64encode(json.dumps(descriptor).encode()).decode()
            with self.subTest(reference=reference), tempfile.TemporaryDirectory() as directory:
                path = Path(directory) / 'package.tgz'
                path.write_bytes(build_archive(recipe))
                report = validate_package(path, case_id='reference-guard')
                self.assertEqual(report['result'], expected)
                if expected == 'invalid':
                    self.assertEqual(report['errors'][0]['code'], 'ART_CONFIG_REFERENCE')
                    self.assertEqual(report['validation_stage'], 'schema')
