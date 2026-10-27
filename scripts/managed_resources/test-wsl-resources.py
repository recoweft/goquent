import importlib.util
from pathlib import Path
import subprocess
import tempfile
import unittest
from unittest.mock import patch

spec = importlib.util.spec_from_file_location('diagnostic', Path(__file__).with_name('diagnose-wsl-resources.py'))
diagnostic = importlib.util.module_from_spec(spec)
spec.loader.exec_module(diagnostic)


class ResourceDiagnosticTests(unittest.TestCase):
    def test_mount_root_is_respected(self):
        mount = '20 10 0:20 /user.slice /sys/fs/cgroup rw - cgroup2 cgroup rw'
        self.assertEqual(diagnostic.locate_cgroup('0::/user.slice/job', mount),
                         (Path('/sys/fs/cgroup/job'), Path('/sys/fs/cgroup'), False))
        self.assertIsNone(diagnostic.locate_cgroup('0::/other/job', mount))

    def test_unknown_or_unsafe_membership_is_not_guessed(self):
        mount = '20 10 0:20 / /sys/fs/cgroup ro - cgroup2 cgroup rw'
        for membership in (None, '1:memory:/job', '0::/../job'):
            self.assertIsNone(diagnostic.locate_cgroup(membership, mount))
        self.assertTrue(diagnostic.locate_cgroup('0::/job', mount)[2])

    def test_visible_ancestor_limits_are_retained(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            child = root / 'job'
            child.mkdir()
            (root / 'memory.max').write_text('2147483648')
            (root / 'pids.max').write_text('64')
            (child / 'memory.max').write_text('max')
            levels = diagnostic.inspect_tree(child, root)
            self.assertEqual(len(levels), 2)
            self.assertEqual(levels[0]['memory_max'], 'max')
            self.assertEqual(levels[1]['memory_max'], '2147483648')
            self.assertEqual(levels[1]['pids_max'], '64')
            self.assertIsNone(levels[0]['pids_max'])

    def test_command_failure_is_unknown_and_stderr_is_not_exposed(self):
        with patch.object(diagnostic.subprocess, 'run', return_value=subprocess.CompletedProcess([], 1, '', 'private data')):
            result = diagnostic.command(['systemctl'])
        self.assertEqual(result['status'], 'unavailable')
        self.assertNotIn('private', str(result))
        with patch.object(diagnostic.subprocess, 'run', side_effect=subprocess.TimeoutExpired('systemctl', 5)):
            self.assertEqual(diagnostic.command(['systemctl'])['status'], 'timeout')

    def test_available_manager_never_claims_enforcement(self):
        mount = '20 10 0:20 / /sys/fs/cgroup rw - cgroup2 cgroup rw'
        files = {'/proc/1/comm': 'systemd', '/proc/self/cgroup': '0::/', '/proc/self/mountinfo': mount}
        with patch.object(diagnostic, 'read', side_effect=lambda p: files.get(str(p))), \
             patch.object(diagnostic, 'command', return_value={'status': 'ok', 'properties': {}}):
            result = diagnostic.diagnose()
        self.assertEqual(result['assessment'], 'user_manager_route_candidate')
        self.assertFalse(result['enforcement_verified'])
        self.assertFalse(result['current_memory_controller_available'])


if __name__ == '__main__':
    unittest.main()
