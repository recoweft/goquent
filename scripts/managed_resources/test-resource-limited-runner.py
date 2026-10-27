import importlib.util
import io
from pathlib import Path
import subprocess
import unittest
from unittest.mock import Mock, patch

spec = importlib.util.spec_from_file_location('runner', Path(__file__).with_name('run-resource-limited.py'))
runner = importlib.util.module_from_spec(spec)
spec.loader.exec_module(runner)


class RunnerTests(unittest.TestCase):
    def test_no_exec_on_missing_limit(self):
        with patch.object(runner.probe, 'snapshot', return_value={'resolved': False}), \
             patch.object(runner.os, 'execv') as execute, patch.object(runner, 'emit'):
            self.assertEqual(runner.guard('unit.service', '/tmp', ['/bin/true']), 125)
            execute.assert_not_called()

    def test_verified_gate_executes_exact_arguments(self):
        evidence = {'resolved': True, 'memory_max': runner.probe.MEMORY, 'pids_max': runner.probe.TASKS,
                    'cgroup_path': '/sys/fs/cgroup/unit.service'}
        with patch.object(runner.probe, 'snapshot', return_value=evidence), \
             patch.object(runner.os, 'execv') as execute, patch.object(runner.os, 'chdir') as chdir, \
             patch.object(runner, 'emit'):
            runner.guard('unit.service', '/tmp/path with space', ['/bin/echo', 'literal; $(command)'])
            chdir.assert_called_once_with('/tmp/path with space')
            execute.assert_called_once_with('/bin/echo', ['/bin/echo', 'literal; $(command)'])

    def test_launch_uses_guard_and_exact_unit_deadline(self):
        args = runner.launch_args('unit.service', '/tmp', 60, ['/bin/true'])
        self.assertIn('--property=RuntimeMaxSec=60s', args)
        self.assertIn('--property=KillMode=control-group', args)
        self.assertIn('--expand-environment=no', args)
        self.assertEqual(args[-4:], ['--guard', 'unit.service', '/tmp', '/bin/true'])

    def test_success_requires_cleanup(self):
        for verified, expected in [(True, 0), (False, 125)]:
            child = Mock()
            child.wait.return_value = 0
            child.poll.return_value = 0
            with patch.object(runner.subprocess, 'Popen', return_value=child), \
                 patch.object(runner.probe, 'cleanup', return_value={'verified': verified}), \
                 patch.object(runner, 'emit'):
                self.assertEqual(runner.run('/tmp', 10, ['/bin/true']), expected)

    def test_timeout_stops_only_owned_unit(self):
        child = Mock()
        child.wait.side_effect = [subprocess.TimeoutExpired('run', 10), 0]
        child.poll.return_value = None
        with patch.object(runner.subprocess, 'Popen', return_value=child), \
             patch.object(runner.probe, 'cleanup', return_value={'verified': True}) as cleanup, \
             patch.object(runner, 'emit'):
            self.assertEqual(runner.run('/tmp', 1, ['/bin/true']), 124)
        self.assertTrue(cleanup.call_args.args[0].startswith('relayweft-resource-run-'))
        child.terminate.assert_called_once()

    def test_failure_is_not_retried(self):
        with patch.object(runner.subprocess, 'Popen', side_effect=OSError) as launch, \
             patch.object(runner.probe, 'cleanup', return_value={'verified': True}), \
             patch.object(runner, 'emit'):
            self.assertEqual(runner.run('/tmp', 1, ['/bin/true']), 125)
        launch.assert_called_once()


if __name__ == '__main__':
    unittest.main()
