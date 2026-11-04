import unittest
from pathlib import Path
import yaml

ROOT = Path(__file__).resolve().parents[1]

class NativeHistoryAdmissionOrder(unittest.TestCase):
    def test_tenant_history_runs_after_admission_before_cache_and_mapping(self):
        specs = {
            name: yaml.safe_load((ROOT / 'plugins/wasm-go/resources/plugins' / name / 'spec.yaml').read_text())['spec']
            for name in ['ai-quota', 'ai-billing', 'ai-history', 'ai-cache', 'model-mapper', 'ai-proxy']
        }
        self.assertEqual({spec['phase'] for spec in specs.values()}, {'default'})
        # Directory.Select requires both the original caller and admitted call;
        # history captures the client body before cache or model mapping.
        order = ['ai-quota', 'ai-billing', 'ai-history', 'ai-cache', 'model-mapper', 'ai-proxy']
        for before, after in zip(order, order[1:]):
            with self.subTest(before=before, after=after):
                self.assertGreater(specs[before]['priority'], specs[after]['priority'])
