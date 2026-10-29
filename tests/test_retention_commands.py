"""Offline temp-store and handler seams; no listener/network or live artifact."""
import copy
import io
import tempfile
import threading
import unittest
from concurrent.futures import ThreadPoolExecutor
from datetime import datetime, timedelta, timezone
from email.message import Message
from pathlib import Path
from unittest.mock import Mock

from artifact_contract import ArtifactError, digest, json_bytes
from artifact_storage import ArtifactStore, StorageConflict, StorageUnavailable
from plugin_server import PluginRequestHandler
from test_artifact_contract import example_archive


class RetentionCommands(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.root = Path(self.temp.name) / 'plugins'
        self.store = ArtifactStore(self.root)
        self.original = self.store.commit('upload-original', example_archive())
        self.artifact = self.original['artifact_id']

    def tearDown(self):
        self.temp.cleanup()

    def decision(self, key='original-delete', revision='1', action='delete_allowed', references=None):
        refs = references or []
        now = datetime.now(timezone.utc)
        return {'decision_id': key, 'artifact_id': self.artifact, 'revision': revision,
                'reference_snapshot_digest': digest(json_bytes(refs)), 'references': refs, 'action': action,
                'decided_at': (now-timedelta(seconds=1)).isoformat(), 'expires_at': (now+timedelta(seconds=60)).isoformat(), 'decided_by': 'system_admin/actual'}

    def test_explicit_delete_is_exact_and_original_upload_audit_survives(self):
        manifest = self.original['manifest']
        result = self.store.retain_or_delete(self.decision())
        self.assertEqual(result['state'], 'deleted')
        self.assertEqual(result['content_digest'], manifest['content_digest'])
        self.assertFalse((self.root/'test-plugin/1.0.0').exists())
        self.assertIsNone(self.store.manifest('test-plugin','1.0.0'))
        self.assertEqual(self.store.query('upload-original'), self.original)
        self.assertEqual(ArtifactStore(self.root).retention_query('original-delete'), result)
        with self.assertRaises(StorageConflict):
            self.store.commit('new-seed', example_archive())

    def test_retain_and_references_never_delete(self):
        for index,kind in enumerate(['active_release','inflight_call','media_operation','historical_fact','rollback'],1):
            decision=self.decision(key=kind, revision=str(index), action='retain', references=[{'kind':kind,'ref_id':'old-reference','retention_reason':'original history'}])
            self.assertEqual(self.store.retain_or_delete(decision)['state'],'retained')
        self.assertTrue((self.root/'test-plugin/1.0.0/plugin.wasm').is_file())

    def test_expired_permission_is_audited_without_filesystem_write(self):
        decision=self.decision()
        decision['decided_at']='2020-01-01T00:00:00Z';decision['expires_at']='2020-01-02T00:00:00Z'
        result=self.store.retain_or_delete(decision)
        self.assertEqual(result['state'],'refused')
        self.assertEqual(self.store.retention_query(decision['decision_id']),result)
        self.assertTrue((self.root/'test-plugin/1.0.0/plugin.wasm').is_file())

    def test_unknown_after_rename_does_not_retry_or_claim_deleted(self):
        phases=[]
        def fault(phase):
            phases.append(phase)
            if phase=='retention_after_rename':raise OSError('injected filesystem interruption')
        self.store.fault=fault
        decision=self.decision()
        with self.assertRaises(StorageUnavailable):self.store.retain_or_delete(decision)
        self.assertEqual(self.store.retention_query(decision['decision_id'])['state'],'unknown')
        self.assertEqual(self.store.retain_or_delete(decision)['state'],'unknown')
        self.assertEqual(phases.count('retention_before_rename'),1)
        self.assertEqual(phases.count('retention_after_rename'),1)
        self.assertFalse((self.root/'test-plugin/1.0.0').exists())
        self.assertEqual(len(list(self.store.state.glob('deleted-*'))),1)
        with self.assertRaises(StorageConflict):self.store.commit('new-seed',example_archive())

    def test_lost_ack_after_complete_delete_is_resolved_only_by_readback(self):
        def fault(phase):
            if phase=='retention_after_delete':raise OSError('lost completion')
        self.store.fault=fault;decision=self.decision()
        with self.assertRaises(StorageUnavailable):self.store.retain_or_delete(decision)
        self.assertEqual(ArtifactStore(self.root).retention_query(decision['decision_id'])['state'],'deleted')

    def test_pre_rename_failure_readback_refuses_without_retry(self):
        def fault(phase):
            if phase=='retention_before_rename':raise OSError('not performed')
        self.store.fault=fault;decision=self.decision()
        with self.assertRaises(StorageUnavailable):self.store.retain_or_delete(decision)
        result=self.store.retention_query(decision['decision_id'])
        self.assertEqual(result['state'],'refused')
        self.assertEqual(result['reason'],'original_files_still_present')
        self.assertEqual(self.store.retain_or_delete(decision),result)
        self.assertEqual(self.store.manifest('test-plugin','1.0.0'),self.original['manifest'])

    def test_same_key_changed_decision_cannot_overwrite_audit(self):
        original=self.decision(action='retain');first=self.store.retain_or_delete(original)
        changed=copy.deepcopy(original);changed['action']='delete_allowed'
        with self.assertRaises(StorageConflict):self.store.retain_or_delete(changed)
        self.assertEqual(self.store.retention_query(original['decision_id']),first)

    def test_concurrent_seed_cannot_revive_reserved_artifact(self):
        entered=threading.Event();release=threading.Event()
        def fault(phase):
            if phase=='retention_before_rename':
                entered.set();self.assertTrue(release.wait(5))
        self.store.fault=fault
        with ThreadPoolExecutor(max_workers=2) as executor:
            deletion=executor.submit(self.store.retain_or_delete,self.decision())
            self.assertTrue(entered.wait(5))
            seed=executor.submit(self.store.commit,'concurrent-seed',example_archive())
            release.set()
            self.assertEqual(deletion.result(5)['state'],'deleted')
            with self.assertRaises(StorageConflict):seed.result(5)

    def handler(self, token, decision):
        cls=PluginRequestHandler.create(self.root,'upload-only',retention_token='console-coordinator')
        h=object.__new__(cls);h.headers=Message();raw=json_bytes(decision)
        if token is not None:h.headers['Authorization']='Bearer '+token
        h.headers['Content-Length']=str(len(raw));h.headers['Content-Type']='application/json'
        h.rfile=io.BytesIO(raw);h.wfile=io.BytesIO();h.connection=Mock();h.command='POST'
        h.path='/retention/'+decision['decision_id'];h.send_response=Mock();h.send_header=Mock();h.end_headers=Mock()
        return h

    def test_upload_token_does_not_authorize_retention_handler(self):
        h=self.handler('upload-only',self.decision());h.do_POST()
        self.assertEqual(h.send_response.call_args.args[0],401)
        self.assertIsNone(self.store.retention_query('original-delete'))
        h=self.handler('console-coordinator',self.decision());h.do_POST()
        self.assertEqual(h.send_response.call_args.args[0],200)
        self.assertEqual(self.store.retention_query('original-delete')['state'],'deleted')

    def test_disabled_and_duplicate_auth_never_dispatch(self):
        h=self.handler('console-coordinator',self.decision());h.retention_token=None;h.do_POST()
        self.assertEqual(h.send_response.call_args.args[0],403)
        h=self.handler('console-coordinator',self.decision());h.headers['Authorization']='Bearer console-coordinator';h.do_POST()
        self.assertEqual(h.send_response.call_args.args[0],401)
        self.assertIsNone(self.store.retention_query('original-delete'))

    def test_unknown_or_malformed_decision_fails_closed(self):
        for field,value in [('revision','01'),('reference_snapshot_digest','bad'),('decided_by',''),('artifact_id','')]:
            decision=self.decision();decision[field]=value
            with self.assertRaises(ArtifactError):self.store.retain_or_delete(decision)
        decision=self.decision();decision['allowed']=True
        with self.assertRaises(ArtifactError):self.store.retain_or_delete(decision)
        self.assertIsNone(self.store.retention_query('original-delete'))

    def test_stale_original_revision_cannot_authorize_another_delete(self):
        original=self.decision(key='retain-current',revision='2',action='retain')
        self.assertEqual(self.store.retain_or_delete(original)['state'],'retained')
        stale=self.decision(key='stale-delete',revision='1')
        result=self.store.retain_or_delete(stale)
        self.assertEqual(result['state'],'refused')
        self.assertEqual(result['reason'],'stale_or_reserved_decision')
        self.assertTrue((self.root/'test-plugin/1.0.0/plugin.wasm').is_file())

    def test_nonstring_reference_kind_is_a_closed_contract_error(self):
        original=self.decision(references=[{'kind':[],'ref_id':'original','retention_reason':'unknown'}])
        with self.assertRaises(ArtifactError):self.store.retain_or_delete(original)
        self.assertIsNone(self.store.retention_query(original['decision_id']))


if __name__=='__main__':unittest.main()
