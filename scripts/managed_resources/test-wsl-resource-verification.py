import importlib.util
from pathlib import Path
import subprocess
import unittest
from unittest.mock import patch

spec = importlib.util.spec_from_file_location('probe', Path(__file__).with_name('verify-wsl-resources.py'))
probe = importlib.util.module_from_spec(spec)
spec.loader.exec_module(probe)


class VerificationTests(unittest.TestCase):
    def evidence(self):
        item = {'resolved': True, 'memory_max': probe.MEMORY, 'pids_max': probe.TASKS,
                'cgroup_path': '/sys/fs/cgroup/test.service', 'membership': '0::/test.service', 'pid': 1}
        return {'parent': item, 'child': {**item, 'pid': 2}, 'child_exit': 0}

    def test_requires_both_limits_and_same_dedicated_cgroup(self):
        self.assertTrue(probe.check_evidence(self.evidence(), 'test.service'))
        for field, value in [('memory_max', 'max'), ('pids_max', 'max'), ('membership', '0::/other'),
                             ('cgroup_path', '/sys/fs/cgroup/other.service'), ('resolved', False), ('pid', 1)]:
            evidence = self.evidence()
            evidence['child'][field] = value
            self.assertFalse(probe.check_evidence(evidence, 'test.service'))

    def test_bounded_user_unit_only(self):
        args = probe.launch_args('test.service')
        for value in ['--user', '--collect', '--property=RuntimeMaxSec=20s',
                      '--property=KillMode=control-group', '--property=MemoryMax=2147483648', '--property=TasksMax=64']:
            self.assertIn(value, args)
        self.assertNotIn('sudo', args)

    def test_timeout_still_cleans_up_exact_unit(self):
        with patch.object(probe.subprocess, 'run', side_effect=subprocess.TimeoutExpired('probe', 30)), \
             patch.object(probe, 'cleanup', return_value={'verified': True}) as cleanup:
            result = probe.verify()
        self.assertEqual(result['status'], 'timeout')
        cleanup.assert_called_once_with(result['unit'])
        self.assertFalse(result['limits_and_child_membership_verified'])

    def test_nonzero_launch_does_not_count_as_verification(self):
        with patch.object(probe.subprocess, 'run', return_value=subprocess.CompletedProcess([], 1, '', 'secret')), \
             patch.object(probe, 'cleanup', return_value={'verified': False}):
            result = probe.verify()
        self.assertEqual(result['status'], 'launch_failed')
        self.assertNotIn('secret', str(result))

    def test_missing_unit_requires_explicit_evidence(self):
        with patch.object(probe.subprocess, 'run', return_value=subprocess.CompletedProcess([], 1, 'LoadState=not-found\n', '')):
            self.assertTrue(probe.cleanup('test.service')['verified'])
        with patch.object(probe.subprocess, 'run', return_value=subprocess.CompletedProcess([], 1, '', '')), \
             patch.object(probe.time, 'sleep'):
            self.assertFalse(probe.cleanup('test.service')['verified'])


if __name__ == '__main__':
    unittest.main()
