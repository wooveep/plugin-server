"""Pure D0-ART-03 trace/readback oracles; no server, HTTP or production storage."""
import sys
import unittest
from pathlib import Path

sys.path.insert(0, str(Path(__file__).parent / 'contracts'))
from artifact_contract_support import ContractError


def load_ref(root, reference):
    from artifact_contract_support import mf, sha256, parse_json
    path = mf.safe_path(root, reference['path'])
    raw = path.read_bytes()
    if sha256(raw) != reference['sha256']:
        raise ContractError('ART_REFERENCE_DIGEST', 'fixture reference bytes differ', 'semantic')
    return parse_json(raw)


def verify_references(root, value):
    from artifact_contract_support import mf, sha256
    if isinstance(value, dict):
        if set(value) == {'path', 'sha256'}:
            path = mf.safe_path(root, value['path'])
            if sha256(path.read_bytes()) != value['sha256']:
                raise ContractError('ART_REFERENCE_DIGEST', 'fixture reference bytes differ', 'semantic')
        for child in value.values():
            verify_references(root, child)
    elif isinstance(value, list):
        for child in value:
            verify_references(root, child)


def check_trace(trace, root):
    """Check declared protocol facts; this oracle does not implement upload persistence."""
    verify_references(root, trace)
    state = None
    allowed = {None: {'receiving'}, 'receiving': {'validating', 'failed'},
               'validating': {'committed', 'rejected', 'failed'},
               'committed': {'committed'}, 'rejected': {'rejected'}, 'failed': {'failed'}}
    state_actions = {'receiving': ('validate_payload', []), 'validating': ('commit_or_reject', []),
                     'committed': ('return_committed_artifact', ['artifact_visible_once']),
                     'rejected': ('correct_input_new_intent', []), 'failed': ('retry_same_upload', [])}
    original = load_ref(root, trace['original_request_ref']) if 'original_request_ref' in trace else None
    first = load_ref(root, trace['original_result_ref']) if 'original_result_ref' in trace else None
    for ordinal, step in enumerate(trace['steps'], 1):
        if 'observed_result_ref' not in step:
            observed, request = step, {}
        else:
            if (step.get('step') != str(ordinal) or step.get('case_id') != trace['case_id']
                    or step.get('actor_kind') not in ('artifact_service', 'consumer')):
                raise ContractError('ART_TRACE_SHAPE', 'invalid trace record', 'semantic')
            observed = load_ref(root, step['observed_result_ref'])
            request = load_ref(root, step['input_ref'])
            verify_references(root, observed)
            verify_references(root, request)
        command = step['command']
        effects = []
        if command == 'state':
            target = observed.get('server_state', observed.get('state'))
            if target not in allowed[state]:
                raise ContractError('ART_LIFECYCLE_TRANSITION', 'illegal protocol sample transition', 'semantic')
            if target == 'committed' and state == 'committed':
                raise ContractError('ART_FIRST_RESULT', 'commit sample cannot create a second visible artifact', 'semantic')
            state = target
            action, effects = state_actions[state]
        elif command in ('response_timeout', 'query'):
            if observed.get('query_result') == 'committed':
                action = 'use_original_result'
                if any(observed.get(key) != first[key] for key in ('artifact_id', 'content_digest', 'archive_digest')):
                    raise ContractError('ART_FIRST_RESULT', 'query changed committed identity', 'semantic')
            else:
                if observed.get('client_outcome') != 'outcome_unknown':
                    raise ContractError('ART_UNKNOWN_CLASSIFICATION', 'timeout is an unknown outcome', 'semantic')
                action = 'query_original_identity_or_replay_same_key' if observed.get('query_result') == 'missing' else 'query_original_identity'
        elif command == 'replay':
            if request['content_digest'] != original['content_digest']:
                raise ContractError('ART_IDENTITY_CONFLICT', 'different content cannot replay original identity', 'semantic')
            if any(observed.get(key) != first[key] for key in ('artifact_id', 'content_digest', 'archive_digest')):
                raise ContractError('ART_FIRST_RESULT', 'replay must return first committed receipt', 'semantic')
            action, effects = 'use_original_result', ['original_result_retained']
        elif command == 'compare_identity':
            code = 'idempotency_conflict' if request['upload_id'] == original['upload_id'] else 'version_conflict'
            if request['content_digest'] == original['content_digest'] or observed.get('error_code') != code:
                raise ContractError('ART_IDENTITY_CONFLICT', 'content identity conflict was lost', 'semantic')
            action = 'reconcile_identity'
        elif command == 'catalog_registration':
            if observed.get('artifact_state') != 'committed' or observed.get('catalog_state') != 'retry_pending':
                raise ContractError('ART_CATALOG_ACTION', 'catalog retry must preserve committed artifact', 'semantic')
            action, effects = 'retry_same_registration', ['artifact_retained']
        elif command == 'readback':
            if 'phase' in observed or 'admission_state' in observed:
                raise ContractError('ART_RELEASE_AUTHORITY', 'artifact facts do not set release status', 'semantic')
            action, effects = 'retry_readback', ['artifact_retained']
        elif command == 'retention':
            if observed.get('retention_action') != 'retain':
                raise ContractError('ART_RETENTION', 'incomplete or live references require retain', 'semantic')
            action, effects = 'retain', ['artifact_retained']
        elif command == 'delete':
            if observed.get('delete_outcome') != 'outcome_unknown' or observed.get('retention_action') != 'retain':
                raise ContractError('ART_RETENTION', 'unknown delete does not prove deletion', 'semantic')
            action, effects = 'query_delete_and_retain', ['artifact_retained']
        elif command == 'legacy_read':
            if observed['source']['verification'] != 'unknown' or observed.get('new_publication') is not False:
                raise ContractError('ART_SOURCE_EVIDENCE', 'legacy unknown cannot become verified publication', 'semantic')
            action = 'legacy_compatibility_read'
        else:
            raise ContractError('ART_TRACE_SHAPE', 'unknown logical protocol action', 'semantic')
        if 'expected_next_action' in step and (step['expected_next_action'] != action or step['expected_side_effects'] != effects):
            raise ContractError('ART_NEXT_ACTION', 'consumer action differs from protocol facts', 'semantic')
    return state


def check_readback(expected, observed, root):
    from artifact_contract_support import validate_document
    import json
    manifest = load_ref(root, expected['manifest_ref'])
    validate_document(json.dumps(manifest).encode(), 'manifest')
    files = expected['expected_files']
    by_path = {item['path']: item for item in files}
    manifest_item = by_path.get('@artifact-manifest')
    wanted = {item['path']: item for item in manifest['files']}
    if (not files or len(by_path) != len(files) or not manifest_item
            or manifest_item['sha256'] != 'sha256:' + expected['manifest_ref']['sha256']
            or set(by_path) != set(wanted) | {'@artifact-manifest'}
            or any(by_path[path] != item for path, item in wanted.items())):
        raise ContractError('ART_READBACK_EXPECTED', 'expected collection must cover manifest and every payload', 'semantic')
    if observed['source_kind'] != 'http':
        raise ContractError('ART_READBACK_SOURCE', 'temporary/local bytes are not HTTP observations', 'semantic')
    instances = observed['instances']
    required = expected['required_instances']
    if (not required or len(set(required)) != len(required)
            or len(instances) != len(required) or {i['instance_id'] for i in instances} != set(required)):
        raise ContractError('ART_READBACK_INSTANCES', 'every frozen instance is required', 'semantic')
    for instance in instances:
        actual = {item['path']: item for item in instance['files']}
        if len(actual) != len(instance['files']) or set(actual) != set(by_path):
            raise ContractError('ART_READBACK_FILES', 'observed file set differs', 'semantic')
        for path, item in actual.items():
            if item['status'] != '200':
                raise ContractError('ART_READBACK_STATUS', 'file response was not successful', 'semantic')
            if item['sha256'] != by_path[path]['sha256']:
                raise ContractError('ART_READBACK_DIGEST', 'successful response has wrong content', 'semantic')
    return True


class ArtifactLifecycleTest(unittest.TestCase):
    def test_type_correct_committed_regression_is_rejected(self):
        trace = {'case_id': 'regression', 'steps': [
            {'actor_kind': 'artifact_service', 'command': 'state', 'state': 'receiving'},
            {'actor_kind': 'artifact_service', 'command': 'state', 'state': 'validating'},
            {'actor_kind': 'artifact_service', 'command': 'state', 'state': 'committed'},
            {'actor_kind': 'artifact_service', 'command': 'state', 'state': 'failed'}]}
        with self.assertRaises(ContractError) as caught:
            check_trace(trace, Path('.'))
        self.assertEqual(caught.exception.code, 'ART_LIFECYCLE_TRANSITION')


class ArtifactLifecycleCorpusTest(unittest.TestCase):
    def test_all_protocol_traces_match_fixed_actions_and_reject_type_correct_illegal_moves(self):
        import json
        root = Path(__file__).resolve().parents[1] / 'contracts/fixtures/v1/artifact'
        catalog = json.loads((root / 'lifecycle-cases.json').read_bytes())
        self.assertGreaterEqual(len(catalog['cases']), 20)
        for case in catalog['cases']:
            with self.subTest(case_id=case['case_id']):
                trace = json.loads((root / case['path']).read_bytes())
                try:
                    check_trace(trace, root)
                    actual = {'verdict': 'accept', 'error_code': None}
                except ContractError as error:
                    actual = {'verdict': 'reject', 'error_code': error.code}
                self.assertEqual(actual, case['expected'])

    def test_readback_requires_nonempty_full_files_and_every_frozen_instance(self):
        import json
        root = Path(__file__).resolve().parents[1] / 'contracts/fixtures/v1/artifact'
        catalog = json.loads((root / 'readback/cases.json').read_bytes())
        for case in catalog['cases']:
            with self.subTest(case_id=case['case_id']):
                try:
                    check_readback(load_ref(root, case['expected_ref']), load_ref(root, case['observed_ref']), root)
                    actual = {'verdict': 'accept', 'error_code': None}
                except ContractError as error:
                    actual = {'verdict': 'reject', 'error_code': error.code}
                self.assertEqual(actual, case['expected'])
