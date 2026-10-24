#!/usr/bin/env python3
"""Collect fresh, private staged evidence; never authorize a release."""
import argparse
import json
import os
from pathlib import Path
import re
import signal
import subprocess
import sys
import tarfile

STAGES = ('inventory', 'offline', 'full', 'race', 'vet', 'regen', 'canary',
          'example', 'schema', 'binding', 'observation', 'native',
          'ordinary', 'strict', 'manifest', 'manifest-evidence', 'manifest-native')
# These inherited deficits cannot be discharged by aggregate command success.
BLOCKERS = ('review-risk-precision', 'review-unknown', 'manifest-coverage',
            'native-performance-limits', 'historical-testmain-cause',
            'historical-expiry-cause', 'human-release-review')
DSNS = {
    'TEST_MYSQL_DSN': 'root:password@tcp(127.0.0.1:3306)/testdb?parseTime=true',
    'TEST_DB_DSN': 'root:password@tcp(127.0.0.1:3306)/testdb?parseTime=true',
    'TEST_POSTGRES_DSN': 'postgres://postgres:password@127.0.0.1:5432/testdb?sslmode=disable',
}


def private_archive_member(member, destination):
    # Preserve the data filter's traversal checks, but never restore public
    # archive modes on private source evidence.
    member = tarfile.data_filter(member, destination)
    return member.replace(mode=0o700 if member.isdir() else 0o600)


def result(status='not-run', code=None, **extra):
    return dict(status=status, exit=code, **extra)


def aggregate(rows, blockers):
    return 'blocked' if blockers or any(r['status'] != 'pass' for r in rows.values()) else 'checks-passed'


def test_evidence(path, code):
    counts = dict(passed=0, failed=0, testskip=0, packages=0, no_test=0)
    seen, tested = set(), set()
    try:
        for line in path.read_text().splitlines():
            event = json.loads(line)
            action, package, test = event.get('Action'), event.get('Package'), event.get('Test')
            if not package:
                continue
            seen.add(package)
            if test and action in ('pass', 'fail', 'skip'):
                tested.add(package)
                counts[{'pass': 'passed', 'fail': 'failed', 'skip': 'testskip'}[action]] += 1
            elif not test and action == 'pass':
                counts['packages'] += 1
        counts['no_test'] = len(seen - tested)
    except (ValueError, OSError, TypeError):
        return result('fail' if code else 'unverified', code)
    status = 'fail' if code or counts['failed'] else 'pass'
    if status == 'pass' and (not counts['passed'] or not counts['packages']):
        status = 'unverified'
    elif status == 'pass' and counts['testskip']:
        status = 'skip'
    return result(status, code, counts=counts)


class Collector:
    def __init__(self, root, output, opt_in=False, base=None, head=None):
        self.root, self.output = root, output
        self.opt_in, self.base, self.head = opt_in, base, head
        self.env = dict(os.environ)
        for key in DSNS:
            self.env.pop(key, None)
        self.env['GOPROXY'] = 'off'
        self.env['GOTOOLCHAIN'] = 'local'
        self.number = 0
        self.cli = output / 'goquent'
        self.native_ready = None
        self.native_failure = None
        self.compose = ['docker', 'compose', '-f', str(root / 'docker-compose.yml'), '--project-name', 'goquent']

    def command(self, args, native=False, cwd=None):
        self.number += 1
        path = self.output / ('command-%03d.log' % self.number)
        env = dict(self.env)
        if native:
            env.update({key: os.environ[key] for key in DSNS})
            env['GOQUENT_NATIVE_DISPOSABLE_FIXTURE'] = '1'
        with path.open('xb') as stream:
            try:
                process = subprocess.Popen(args, cwd=cwd or self.root, env=env,
                                           stdout=stream, stderr=subprocess.STDOUT, start_new_session=True)
                try:
                    code = process.wait(timeout=1800)
                except subprocess.TimeoutExpired:
                    # Reap the command tree before starting another DB stage.
                    try:
                        os.killpg(process.pid, signal.SIGKILL)
                    except ProcessLookupError:
                        pass
                    except OSError:
                        raise RuntimeError('process cleanup unavailable') from None
                    process.wait()
                    code = 125
            except OSError:
                code = 125
        with (self.output / 'commands.jsonl').open('a') as metadata:
            metadata.write(json.dumps(dict(command=args, exit=code, log=path.name, native=native)) + '\n')
        return code, path

    def simple(self, args):
        code, _ = self.command(args)
        return result('pass' if code == 0 else 'fail', code)

    def tests(self, packages, pattern=None, native=False, race=False):
        args = ['go', 'test', '-json', '-p', '1', '-count=1']
        if race:
            args.append('-race')
        if pattern:
            args += ['-run', pattern]
        code, path = self.command(args + packages, native=native)
        row = test_evidence(path, code)
        if native and row['status'] == 'pass':
            required = {
                ('github.com/recoweft/goquent/orm', 'TestObservationIntegration/' + dialect)
                for dialect in ('mysql', 'postgres')
            } | {
                ('github.com/recoweft/goquent/tests', test + '/' + dialect)
                for dialect in ('mysql', 'postgres')
                for test in ('TestPerformancePlanNative', 'TestPerformancePlanNativeBoundaries', 'TestExternalAuthorizationDatabaseAndExternalTx',
                             'TestValidatedBindingDatabaseAndExternalTx')
            }
            required |= {
                ('github.com/recoweft/goquent/tests', 'TestPerformancePlanNativeBoundaries/' + dialect + '/' + case)
                for dialect in ('mysql', 'postgres')
                for case in ('restricted_role', 'network_disconnect', 'network_timeout', 'network_cancel')
            } | {
                ('github.com/recoweft/goquent/tests', 'TestPerformancePlanNativeBoundaries/postgres/' + case)
                for case in ('custom_operator', 'custom_cast')
            }
            events = [json.loads(line) for line in path.read_text().splitlines()]
            passed = {(e.get('Package'), e.get('Test')) for e in events if e.get('Action') == 'pass'}
            if not required <= passed:
                row['status'] = 'unverified'
                row['reason'] = 'native-test-evidence-missing'
        return row

    def prepare_native(self):
        if self.native_ready is not None:
            return self.native_ready
        self.native_ready = False
        if not self.opt_in or any(os.environ.get(k) != v for k, v in DSNS.items()):
            return False
        # Only the already running repository Compose fixtures; never provision.
        code, path = self.command(self.compose + ['ps', '--format', 'json'])
        if code:
            self.native_failure = code
            return False
        try:
            services = [json.loads(line) for line in path.read_text().splitlines()]
            for name, image, port in [('mysql', 'mysql:8', 3306), ('postgres', 'postgres:16', 5432)]:
                item = next(x for x in services if x['Service'] == name)
                if item['Image'] != image or item['State'] != 'running' or item['Project'] != 'goquent':
                    return False
                if not any(p['PublishedPort'] == port and p['TargetPort'] == port for p in item['Publishers']):
                    return False
            if code:
                return False
        except (ValueError, KeyError, StopIteration, TypeError):
            return False
        for args in [
            self.compose + ['exec', '-T', 'mysql', 'mysql', '-uroot', '-ppassword', '-Nse', 'SELECT VERSION()'],
            self.compose + ['exec', '-T', 'postgres', 'psql', '-U', 'postgres', '-d', 'testdb', '-Atc', 'SELECT VERSION()'],
        ]:
            code, path = self.command(args)
            if code:
                self.native_failure = code
                return False
            if not re.search(r'\d+\.\d+', path.read_text()):
                return False
        self.native_ready = True
        return True

    def build(self):
        if self.cli.exists():
            return 0
        code, _ = self.command(['go', 'build', '-o', str(self.cli), './cmd/goquent'])
        return code

    def manifest_evidence(self, native=False):
        if native and not self.prepare_native():
            if self.native_failure is not None:
                return result('fail', self.native_failure, reason='native-preflight-command')
            return result('unverified', reason='native-prerequisites')
        build_code = self.build()
        if build_code:
            return result('fail', build_code, reason='build')
        code, _ = self.command([str(self.cli), 'manifest', 'check-repositories',
                               '--config', 'tests/manifestfixture/repositories.json'])
        if code:
            return result('fail', code, reason='generated-source')
        area = self.output / ('manifest-native' if native else 'manifest-evidence')
        area.mkdir(mode=0o700)
        if native:
            code, log = self.command(['go', 'test', '-json', '-p', '1', '-count=1',
                './tests/manifestfixture', '-run', '^TestManifestEvidenceNative$',
                '-args', '-manifest-evidence-output', str(area)], native=True)
            observed = test_evidence(log, code)
            if observed['status'] != 'pass':
                return observed
            events = [json.loads(line) for line in log.read_text().splitlines()]
            passed = {e.get('Test') for e in events if e.get('Action') == 'pass'}
            if not {'TestManifestEvidenceNative/mysql', 'TestManifestEvidenceNative/postgres'} <= passed:
                return result('unverified', code, reason='native-evidence-missing')
        (area / 'provenance.json').write_text(json.dumps(dict(
            source='tests/manifestfixture/declaration.go',
            generator_input='tests/manifestfixture/generator-input.json',
            registry='tests/manifestfixture/repositories.json',
            expected='committed fingerprints; native changes only declared database identity',
            current='migration.ReadSchema owned fixture' if native else 'supplied declared snapshot',
            native=native), indent=2))
        rows = {}
        for dialect in ('mysql', 'postgres'):
            work = area / dialect
            work.mkdir(mode=0o700)
            code_name = 'tests/manifestfixture/users_generated.go'
            target = work / code_name
            target.parent.mkdir(parents=True, mode=0o700)
            target.write_bytes((self.root / code_name).read_bytes())
            schema = self.root / 'examples/ai-safe-orm/schema.json'
            policy = self.root / 'examples/ai-safe-orm/policies.json'
            fixture = self.root / 'tests/manifestfixture'
            stored = json.loads((area / (dialect + '-manifest.json') if native else
                                 fixture / (dialect + '.manifest.json')).read_text())
            current = area / (dialect + '-current.json') if native else fixture / (dialect + '-database.json')
            expected_checks = dict(schema='ok', policy='ok', generated_code='ok', database='ok')
            cases = ('four-axis', 'code-drift', 'database-drift', 'schema-content', 'policy-content',
                     'missing-code', 'database-disabled', 'missing-database', 'invalid-manifest',
                     'missing-fingerprints', 'missing-stored-code', 'unreadable-input')
            for case in cases:
                m = json.loads(json.dumps(stored))
                checks = dict(expected_checks)
                want, status = 0, 'pass'
                db = current
                args = ['--code', code_name, '--against-db', '--database-schema', str(db)]
                if case == 'code-drift':
                    target.write_bytes(target.read_bytes() + b'\n// CANARY_MANIFEST_SOURCE\n')
                    checks['generated_code'], want, status = 'stale', 1, 'fail'
                elif case == 'database-drift':
                    if native:
                        db = area / (dialect + '-drift.json')
                    else:
                        value = json.loads(current.read_text())
                        value['tables'][0]['columns'][0]['type'] = 'CANARY_DATABASE_TYPE'
                        db = work / 'drift.json'
                        db.write_text(json.dumps(value))
                    args[-1] = str(db)
                    checks['database'], want, status = 'stale', 1, 'fail'
                elif case in ('schema-content', 'policy-content'):
                    axis = case.split('-')[0]
                    if axis == 'schema':
                        m['tables'][0]['columns'][0]['type'] = 'CANARY_SCHEMA_TYPE'
                    else:
                        m['tables'][0]['policies'][0]['column'] = 'CANARY_POLICY_COLUMN'
                    checks[axis], want, status = 'stale', 1, 'fail'
                elif case == 'missing-code':
                    args = args[2:]
                    checks['generated_code'], want, status = 'stale', 1, 'fail'
                elif case == 'database-disabled':
                    args = args[:2]
                    checks['database'], status = 'skipped', 'skip'
                elif case == 'missing-database':
                    args = args[:3]
                    checks, want, status = {}, 2, 'fail'
                elif case == 'invalid-manifest':
                    m['version'] = 'CANARY_INVALID_VERSION'
                    checks, want, status = {}, 2, 'fail'
                elif case == 'missing-fingerprints':
                    m.pop('generated_code_fingerprint', None)
                    m.pop('database_fingerprint', None)
                    args = []
                    checks['generated_code'] = checks['database'] = 'skipped'
                    status = 'skip'
                elif case == 'missing-stored-code':
                    m.pop('generated_code_fingerprint', None)
                    checks['generated_code'], want, status = 'stale', 1, 'fail'
                elif case == 'unreadable-input':
                    args[1] = 'CANARY_MISSING_PATH'
                    checks, want, status = {}, 2, 'fail'
                path = work / (case + '.json')
                path.write_text(json.dumps(m))
                command = [str(self.cli), 'manifest', 'verify', '--format', 'json', '--manifest', str(path),
                           '--schema', str(schema), '--policy', str(policy)] + args
                code, log = self.command(command, cwd=work)
                row = manifest_evidence(log, code)
                # Error cases have no fabricated check results. Raw logs remain local.
                boundary = 'CANARY_' not in log.read_text()
                matched = code == want and row['status'] == status and row.get('checks', {}) == checks and boundary
                rows[dialect + '/' + case] = dict(row, regression='pass' if matched else 'fail')
                target.write_bytes((self.root / code_name).read_bytes())
        (area / 'cases.json').write_text(json.dumps(rows, indent=2))
        passed = sum(row['regression'] == 'pass' for row in rows.values())
        return result('pass' if passed == len(rows) else 'fail', 0 if passed == len(rows) else 1,
                      counts=dict(passed=passed, failed=len(rows)-passed), cases=rows)

    def strict_inputs(self):
        if not self.base or not self.head:
            return None
        resolved = []
        for ref in (self.base, self.head, 'HEAD'):
            code, path = self.command(['git', 'rev-parse', '--verify', ref + '^{commit}'])
            value = path.read_text().strip()
            if code or not re.fullmatch('[0-9a-f]{40}', value):
                return None
            resolved.append(value)
        if resolved[1] != resolved[2] or resolved[0] == resolved[1]:
            return None
        code, path = self.command(['git', 'status', '--porcelain'])
        if code or path.read_text().strip():
            return None
        roots = []
        for name, commit in zip(('base', 'head'), resolved):
            code, archive = self.command(['git', 'archive', '--format=tar', commit])
            if code:
                return None
            target = self.output / name
            target.mkdir()
            with tarfile.open(archive) as source:
                if any(not (m.isfile() or m.isdir()) for m in source.getmembers()):
                    return None
                source.extractall(target, filter=private_archive_member)
            roots.append(target)
        code, patch = self.command(['git', 'diff', '--no-ext-diff', '--no-renames', resolved[0], resolved[1], '--'])
        if code or not patch.stat().st_size:
            return None
        (self.output / 'commits.json').write_text(json.dumps(resolved[:2]))
        return ['--diff', str(patch), '--diff-base-root', str(roots[0]), str(roots[1])]

    def stage(self, name):
        if name in ('manifest-evidence', 'manifest-native'):
            return self.manifest_evidence(native=name == 'manifest-native')
        if name == 'inventory':
            return self.simple(['go', 'version'])
        if name in ('full', 'race', 'native'):
            if not self.prepare_native():
                return result('unverified', reason='native-precondition')
            if name == 'native':
                return self.tests(['./orm', './tests'],
                    'Test(ObservationIntegration|PerformancePlanNative|ExternalAuthorizationDatabaseAndExternalTx|Validated)', native=True)
            return self.tests(['./...'], native=True, race=name == 'race')
        if name == 'offline':
            return self.tests(['./tests/contracts', './orm/operation', './orm/manifest', './examples/ai-safe-orm'])
        if name == 'schema':
            return self.tests(['./orm/migration'])
        if name == 'binding':
            return self.tests(['./orm/query', './orm'], '^Test(Binding|Authorization|Tenant|Strict|Identity)')
        if name == 'observation':
            return self.tests(['./orm'], '^TestObservation(Rows|Legacy|Policy|Structured|Generic|Internal|Command|Row|Connection|Different|Custom)')
        if name == 'vet':
            return self.simple(['go', 'vet', './...'])
        if name == 'regen':
            return self.simple(['go', 'run', './cmd/goquent', 'manifest', 'check-repositories', '--config', 'tests/typedfixture/repositories.json'])
        if name == 'canary':
            first = self.simple(['sh', 'scripts/test-public-output-check.sh'])
            if first['status'] != 'pass':
                return first
            return self.tests(['./cmd/goquent', './orm/query', './orm/review', './orm/mcp'], 'Canary|Public|Redact|SARIF|Sarif')
        if name == 'example':
            return self.simple(['sh', 'examples/ai-safe-orm/check.sh'])
        build_code = self.build()
        if build_code:
            return result('fail', build_code, reason='build')
        if name == 'manifest':
            code, _ = self.command([str(self.cli), 'manifest', 'verify', '--manifest', 'examples/ai-safe-orm/goquent.manifest.json',
                '--schema', 'examples/ai-safe-orm/schema.json', '--policy', 'examples/ai-safe-orm/policies.json'])
            return result('fail' if code else 'skip', code, reason='generated-code-database-not-supplied')
        args = [str(self.cli), 'review', '--format', 'review-ci-json', '--fail-on', 'high']
        if name == 'strict':
            inputs = self.strict_inputs()
            if inputs is None:
                return result('unverified', reason='exact-clean-inputs-required')
            args += ['--fail-on-precision', 'partial'] + inputs
        else:
            args += ['./...']
        code, path = self.command(args)
        # Public JSON is supplementary classification only. The actual command
        # computes gates from its complete private report; retain its real exit.
        return review_evidence(path, code)


def manifest_evidence(path, code):
    """Require the complete four-check CLI view; exit zero alone proves nothing."""
    try:
        view = json.loads(path.read_text())
        if (view['kind'] != 'goquent.verification_view' or view['version'] != 1 or
                view['count'] != 4 or view.get('truncated', False) or
                view.get('present') is not True or view.get('known') is not True):
            raise ValueError()
        items = view['items']
        checks = {item['check']: item['status'] for item in items}
        if len(items) != 4 or set(checks) != {'schema', 'policy', 'generated_code', 'database'}:
            raise ValueError()
        if any(v not in ('ok', 'skipped', 'stale') for v in checks.values()):
            raise ValueError()
        if code:
            status = 'fail'
        elif 'stale' in checks.values() or view.get('fresh') is not True:
            status = 'unverified'
        elif 'skipped' in checks.values():
            status = 'skip'
        else:
            status = 'pass'
        return result(status, code, checks=checks)
    except (ValueError, KeyError, TypeError, OSError):
        return result('fail' if code else 'unverified', code)


def review_evidence(path, code):
    try:
        view = json.loads(path.read_text())
        if view['kind'] != 'goquent.review_ci_view' or view['version'] != 1:
            raise ValueError()
        gate = view['gate']
        for key in ('full_report', 'risk_failed', 'precision_failed', 'manifest_failed', 'input_failed'):
            if type(gate[key]) is not bool:
                raise ValueError()
        findings = view['head_findings']
        for key in ('matched', 'new', 'worsened', 'unknown', 'unchanged'):
            if type(findings[key]) is not int or findings[key] < 0:
                raise ValueError()
        status = 'fail' if code else 'pass'
        if not code and (not gate['full_report'] or findings['unknown'] or any(
                gate[key] for key in ('risk_failed', 'precision_failed', 'manifest_failed', 'input_failed'))):
            status = 'unverified'
        return result(status, code, gate=gate, findings=findings)
    except (ValueError, KeyError, TypeError, OSError):
        return result('fail' if code else 'unverified', code)



class PublicParser(argparse.ArgumentParser):
    def error(self, message):
        self.exit(2, 'collection=error release=blocked\n')


def main():
    parser = PublicParser(description=__doc__)
    parser.add_argument('--stages', default='inventory')
    parser.add_argument('--skip', default='')
    parser.add_argument('--output', required=True, help='new private directory outside repository')
    parser.add_argument('--native-disposable-fixture', action='store_true')
    parser.add_argument('--base')
    parser.add_argument('--head')
    args = parser.parse_args()
    rows = {name: result() for name in STAGES}
    try:
        requested = args.stages.split(',')
        skipped = args.skip.split(',') if args.skip else []
        if any(name not in STAGES for name in requested + skipped):
            raise ValueError()
        root = Path(__file__).resolve().parents[2]
        output = Path(args.output).resolve()
        if output.is_relative_to(root) or output.exists():
            raise ValueError()
        os.umask(0o077)
        output.mkdir(mode=0o700)
        collector = Collector(root, output, args.native_disposable_fixture, args.base, args.head)
        if '-json' in os.environ.get('GOFLAGS', ''):
            raise ValueError()
        for name in STAGES:
            if name in skipped:
                rows[name] = result('skip', reason='explicit-skip')
            elif name in requested:
                rows[name] = collector.stage(name)
        summary = dict(collection='complete', release=aggregate(rows, BLOCKERS), stages=rows, blockers=BLOCKERS)
        (output / 'results.json').write_text(json.dumps(summary, indent=2) + '\n')
        # Only closed classifications, counts and numeric exits are published.
        for name, row in rows.items():
            public = {key: row[key] for key in ('status', 'exit', 'counts') if key in row}
            print(name + ' ' + json.dumps(public, sort_keys=True))
        print('collection=complete release=' + summary['release'])
        return 1 if summary['release'] == 'blocked' else 0
    except Exception:
        print('collection=error release=blocked')
        return 2


if __name__ == '__main__':
    sys.exit(main())
