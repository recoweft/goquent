"""Negative and orchestration regressions; no real database is used here."""
import contextlib
import io
import json
import os
from pathlib import Path
import tempfile
import subprocess
import signal
import unittest
from unittest.mock import patch, Mock
import run


class ValidationTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)

    def test_manifest_closed_four_axes_and_real_exits(self):
        log = self.root / 'manifest.log'
        view = dict(kind='goquent.verification_view', version=1, count=4,
                    present=True, known=True, fresh=True,
                    items=[dict(check=k, status='ok') for k in
                           ('schema', 'policy', 'generated_code', 'database')])
        def check(value, code=0):
            log.write_text(json.dumps(value))
            return run.manifest_evidence(log, code)['status']
        self.assertEqual(check(view), 'pass')
        self.assertEqual(check(view, 2), 'fail')
        view['items'][-1]['status'] = 'skipped'
        self.assertEqual(check(view), 'skip')
        view['items'][-1]['status'] = 'stale'
        view['fresh'] = False
        self.assertEqual(check(view, 1), 'fail')
        self.assertEqual(check(view), 'unverified')
        view['items'][-1]['check'] = 'CANARY_UNKNOWN'
        self.assertEqual(check(view), 'unverified')
        view['items'][-1] = view['items'][0]
        self.assertEqual(check(view), 'unverified')
        log.write_text('CANARY_ERROR')
        self.assertEqual(run.manifest_evidence(log, 2), run.result('fail', 2))
        self.assertEqual(run.manifest_evidence(log, 0), run.result('unverified', 0))

    def test_manifest_native_invoked_preflight_failure_is_not_unverified(self):
        collector = run.Collector(self.root, self.root, True)
        log = self.root / 'failed-preflight'
        log.write_text('CANARY_SOCKET_ERROR')
        with patch.dict(os.environ, run.DSNS, clear=True), patch.object(
                collector, 'command', return_value=(125, log)):
            row = collector.manifest_evidence(native=True)
        self.assertEqual(row['status'], 'fail')
        self.assertEqual(row['exit'], 125)

    def test_manifest_real_cli_regressions(self):
        # Independent committed expectations; no generation precedes this check.
        repo = Path(__file__).resolve().parents[2]
        out = self.root / 'private'
        out.mkdir(mode=0o700)
        previous = os.umask(0o077)
        try:
            collector = run.Collector(repo, out)
            row = collector.manifest_evidence()
            self.assertEqual(row['status'], 'pass', 'finite real CLI matrix failed; details omitted')
            self.assertEqual(row['counts'], dict(passed=24, failed=0))
            self.assertEqual(row['cases']['mysql/database-disabled']['status'], 'skip')
            self.assertEqual(row['cases']['postgres/missing-code']['exit'], 1)
            for path in out.rglob('*'):
                self.assertEqual(path.stat().st_mode & 0o077, 0)
        finally:
            os.umask(previous)

    def test_aggregate_never_promotes_missing_evidence(self):
        for status in ('fail', 'not-run', 'skip', 'unverified'):
            with self.subTest(status=status):
                self.assertEqual(run.aggregate({'a': run.result('pass'), 'b': run.result(status)}, []), 'blocked')
        self.assertEqual(run.aggregate({'a': run.result('pass')}, ['unresolved']), 'blocked')
        self.assertEqual(run.aggregate({'a': run.result('pass')}, []), 'checks-passed')

    def evidence(self, events, code=0):
        log = self.root / 'log'
        log.write_text(''.join(json.dumps(e) + '\n' for e in events))
        return run.test_evidence(log, code)

    def test_real_exit_testskip_no_test_and_corrupt_evidence(self):
        passed = [{'Package': 'p', 'Test': 'T', 'Action': 'pass'}, {'Package': 'p', 'Action': 'pass'}]
        self.assertEqual(self.evidence(passed)['status'], 'pass')
        self.assertEqual(self.evidence(passed, 7)['status'], 'fail')
        self.assertEqual(self.evidence(passed + [{'Package': 'p', 'Test': 'S', 'Action': 'skip'}])['status'], 'skip')
        no_test = self.evidence([{'Package': 'p', 'Action': 'pass'}])
        self.assertEqual(no_test['status'], 'unverified')
        self.assertEqual(no_test['counts']['no_test'], 1)
        log = self.root / 'log'
        log.write_text('PASSED secret-canary\n')
        self.assertEqual(run.test_evidence(log, 0)['status'], 'unverified')
        self.assertEqual(run.test_evidence(log, 9)['status'], 'fail')

    def test_native_requires_opt_in_all_dsns_and_local_targets(self):
        for opt_in, env in [(False, run.DSNS), (True, {}),
                            (True, {**run.DSNS, 'TEST_DB_DSN': 'secret-canary'}),
                            (True, {**run.DSNS, 'TEST_POSTGRES_DSN': 'production'})]:
            with patch.dict(os.environ, env, clear=True):
                collector = run.Collector(self.root, self.root, opt_in)
                with patch.object(collector, 'command', side_effect=AssertionError('must not execute')):
                    for name in ('native', 'full', 'race', 'manifest-native'):
                        self.assertEqual(collector.stage(name)['status'], 'unverified')

    def test_fixture_versions_are_sequential_and_failure_stops_native(self):
        collector = run.Collector(self.root, self.root, True)
        calls = []
        def command(args, **kwargs):
            calls.append(args)
            log = self.root / ('fixture-' + str(len(calls)))
            if 'ps' in args:
                log.write_text('\n'.join(json.dumps(dict(Service=name, Image=image,
                    State='running', Project='goquent', Publishers=[dict(PublishedPort=port, TargetPort=port)]))
                    for name, image, port in [('mysql', 'mysql:8', 3306), ('postgres', 'postgres:16', 5432)]))
            else:
                log.write_text('8.0' if 'mysql' in args else '16.10')
            return 0, log
        with patch.dict(os.environ, run.DSNS, clear=True), patch.object(collector, 'command', command):
            self.assertTrue(collector.prepare_native())
        self.assertEqual(len(calls), 3)
        self.assertIn('mysql', calls[1])
        self.assertIn('postgres', calls[2])
        self.assertIn('--project-name', calls[0])
        collector.native_ready = None
        calls.clear()
        def fail_version(args, **kwargs):
            code, log = command(args, **kwargs)
            return (7 if 'exec' in args else code), log
        with patch.dict(os.environ, run.DSNS, clear=True), patch.object(collector, 'command', fail_version):
            self.assertFalse(collector.prepare_native())
        self.assertEqual(len(calls), 2)

    def test_commands_strip_dsns_and_capture_secret_failures(self):
        with patch.dict(os.environ, run.DSNS):
            collector = run.Collector(self.root, self.root)
        code, log = collector.command(['sh', '-c', 'test -z "$TEST_MYSQL_DSN$TEST_DB_DSN$TEST_POSTGRES_DSN" || exit 8; echo secret-canary; exit 7'])
        self.assertEqual(code, 7)
        self.assertIn('secret-canary', log.read_text())
        metadata = json.loads((self.root / 'commands.jsonl').read_text())
        self.assertEqual(metadata['exit'], 7)
        self.assertEqual(metadata['log'], log.name)

    def test_timeout_reaps_process_group_before_return(self):
        collector = run.Collector(self.root, self.root)
        process = Mock(pid=12345)
        process.wait.side_effect = [subprocess.TimeoutExpired('fixture', 1800), -9]
        with patch('run.subprocess.Popen', return_value=process) as popen, patch('run.os.killpg') as kill:
            code, _ = collector.command(['fixture'])
        self.assertEqual(code, 125)
        self.assertTrue(popen.call_args.kwargs['start_new_session'])
        kill.assert_called_once_with(12345, signal.SIGKILL)
        self.assertEqual(process.wait.call_count, 2)

    def test_default_never_calls_db_and_public_boundary(self):
        output = self.root / 'fresh'
        calls = []
        def command(instance, args, **kwargs):
            calls.append(args)
            log = output / 'raw'
            log.write_text('secret-canary SQL DSN /absolute/path')
            return 7, log
        with patch('sys.argv', ['run.py', '--output', str(output)]), patch.object(run.Collector, 'command', command):
            stdout = io.StringIO()
            with contextlib.redirect_stdout(stdout):
                self.assertEqual(run.main(), 1)
        self.assertEqual(calls, [['go', 'version']])
        self.assertNotIn('secret-canary', stdout.getvalue())
        self.assertNotIn('/absolute/path', stdout.getvalue())
        data = json.loads((output / 'results.json').read_text())
        self.assertEqual(data['stages']['native']['status'], 'not-run')
        self.assertEqual(data['stages']['inventory']['status'], 'fail')
        self.assertEqual(output.stat().st_mode & 0o777, 0o700)

    def test_stage_order_and_explicit_skip_remain_non_success(self):
        output = self.root / 'ordered'
        calls = []
        def stage(instance, name):
            calls.append(name)
            return run.result('pass', 0)
        with patch('sys.argv', ['run.py', '--output', str(output), '--stages', 'race,native,full', '--skip', 'native']), patch.object(run.Collector, 'stage', stage):
            with contextlib.redirect_stdout(io.StringIO()):
                self.assertEqual(run.main(), 1)
        self.assertEqual(calls, ['full', 'race'])
        self.assertEqual(json.loads((output / 'results.json').read_text())['stages']['native']['status'], 'skip')

    def test_expected_refusals_use_existing_exact_checker(self):
        collector = run.Collector(self.root, self.root)
        with patch.object(collector, 'command', return_value=(3, self.root / 'unused')) as command:
            self.assertEqual(collector.stage('example')['status'], 'fail')
            self.assertEqual(command.call_args.args[0], ['sh', 'examples/ai-safe-orm/check.sh'])
        # That checker, not arbitrary nonzero exits, owns expected 3/4 refusals.

    def test_review_keeps_exit_gate_unknown_and_input_separate(self):
        log = self.root / 'review'
        view = dict(kind='goquent.review_ci_view', version=1,
                    gate=dict(full_report=True, risk_failed=True, precision_failed=True,
                              manifest_failed=False, input_failed=False),
                    head_findings=dict(matched=0, new=1, worsened=0, unknown=1473, unchanged=0))
        log.write_text(json.dumps(view))
        row = run.review_evidence(log, 1)
        self.assertEqual(row['status'], 'fail')
        self.assertFalse(row['gate']['input_failed'])
        self.assertEqual(row['findings']['unknown'], 1473)
        self.assertEqual(run.review_evidence(log, 0)['status'], 'unverified')
        view['gate'].update(risk_failed=False, precision_failed=False)
        log.write_text(json.dumps(view))
        self.assertEqual(run.review_evidence(log, 0)['status'], 'unverified')
        view['head_findings']['unknown'] = 0
        log.write_text(json.dumps(view))
        self.assertEqual(run.review_evidence(log, 0)['status'], 'pass')
        view['gate']['input_failed'] = True
        log.write_text(json.dumps(view))
        self.assertEqual(run.review_evidence(log, 2)['status'], 'fail')
        log.write_text('secret-canary')
        self.assertEqual(run.review_evidence(log, 0)['status'], 'unverified')

    def test_native_success_requires_actual_tests_for_both_databases(self):
        collector = run.Collector(self.root, self.root, True)
        log = self.root / 'native'
        events = []
        for dialect in ('mysql', 'postgres'):
            for package, tests in [('orm', ['TestObservationIntegration']),
                    ('tests', ['TestPerformancePlanNative', 'TestPerformancePlanNativeBoundaries', 'TestExternalAuthorizationDatabaseAndExternalTx',
                               'TestValidatedBindingDatabaseAndExternalTx'])]:
                for test in tests:
                    events.append(dict(Package='github.com/recoweft/goquent/' + package,
                                       Test=test + '/' + dialect, Action='pass'))
        for dialect in ('mysql', 'postgres'):
            cases = ['restricted_role', 'network_disconnect', 'network_timeout', 'network_cancel']
            if dialect == 'postgres':
                cases += ['custom_operator', 'custom_cast']
            for case in cases:
                events.append(dict(Package='github.com/recoweft/goquent/tests',
                                   Test='TestPerformancePlanNativeBoundaries/' + dialect + '/' + case,
                                   Action='pass'))
        events.append(dict(Package='github.com/recoweft/goquent/orm', Action='pass'))
        log.write_text(''.join(json.dumps(e) + '\n' for e in events))
        with patch.object(collector, 'command', return_value=(0, log)) as command:
            self.assertEqual(collector.tests(['./orm', './tests'], native=True)['status'], 'pass')
            self.assertIn('-p', command.call_args.args[0])
            events.pop(-2)
            log.write_text(''.join(json.dumps(e) + '\n' for e in events))
            self.assertEqual(collector.tests(['./orm', './tests'], native=True)['status'], 'unverified')

    def test_strict_requires_explicit_distinct_clean_commits(self):
        collector = run.Collector(self.root, self.root)
        with patch.object(collector, 'command', side_effect=AssertionError('no command')):
            self.assertIsNone(collector.strict_inputs())


if __name__ == '__main__':
    unittest.main()
