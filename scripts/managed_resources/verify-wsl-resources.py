"""Explicit, bounded systemd user-unit probe; never runs project commands."""
import importlib.util
import json
import os
from pathlib import Path
import subprocess
import sys
import time
import uuid
from datetime import datetime, timezone

spec = importlib.util.spec_from_file_location('preflight', Path(__file__).with_name('diagnose-wsl-resources.py'))
preflight = importlib.util.module_from_spec(spec)
spec.loader.exec_module(preflight)
MEMORY = '2147483648'
TASKS = '64'


def snapshot():
    location = preflight.locate_cgroup(preflight.read('/proc/self/cgroup'), preflight.read('/proc/self/mountinfo'))
    if not location:
        return {'resolved': False}
    path = location[0]
    return {'resolved': True, 'pid': os.getpid(), 'membership': preflight.read('/proc/self/cgroup'),
            'cgroup_path': str(path), 'memory_max': preflight.read(path / 'memory.max'),
            'pids_max': preflight.read(path / 'pids.max')}


def payload():
    parent = snapshot()
    child = subprocess.run([sys.executable, str(Path(__file__).resolve()), '--child'],
                           capture_output=True, text=True, timeout=5, check=True)
    print(json.dumps({'parent': parent, 'child': json.loads(child.stdout), 'child_exit': child.returncode}), flush=True)


def check_evidence(evidence, unit):
    parent, child = evidence.get('parent', {}), evidence.get('child', {})
    return (all(item.get('resolved') and item.get('memory_max') == MEMORY
                and item.get('pids_max') == TASKS
                and Path(item.get('cgroup_path', '')).name == unit for item in (parent, child))
            and parent.get('membership') == child.get('membership')
            and parent.get('pid') != child.get('pid') and evidence.get('child_exit') == 0)


def launch_args(unit):
    return ['systemd-run', '--user', '--no-ask-password', '--quiet', '--wait', '--pipe', '--collect',
            '--service-type=exec', '--unit=' + unit,
            '--property=MemoryMax=' + MEMORY, '--property=TasksMax=' + TASKS,
            '--property=RuntimeMaxSec=20s', '--property=TimeoutStopSec=3s',
            '--property=KillMode=control-group',
            sys.executable, str(Path(__file__).resolve()), '--payload']


def cleanup(unit):
    # Only this invocation's UUID-named unit is addressed. Never stop a slice or worker.
    try:
        subprocess.run(['systemctl', '--user', '--no-ask-password', 'stop', unit],
                       capture_output=True, text=True, timeout=6)
        for _ in range(10):
            result = subprocess.run(['systemctl', '--user', 'show', unit, '--property=LoadState',
                                     '--property=ActiveState'], capture_output=True, text=True, timeout=3)
            props = dict(line.split('=', 1) for line in result.stdout.splitlines() if '=' in line)
            # A failed command alone is not proof that a unit was removed.
            if props.get('LoadState') == 'not-found':
                return {'verified': True, 'load_state': 'not-found'}
            time.sleep(.2)
    except (OSError, subprocess.TimeoutExpired):
        pass
    return {'verified': False, 'load_state': 'unknown_or_retained'}


def verify():
    unit = 'relayweft-resource-probe-' + uuid.uuid4().hex + '.service'
    report = {'schema_version': 1, 'observed_at': datetime.now(timezone.utc).isoformat(),
              'unit': unit, 'requested': {'memory_max': MEMORY, 'pids_max': TASKS},
              'limits_and_child_membership_verified': False, 'stress_test_performed': False,
              'scope': 'Temporary probe only; existing workers and project comparisons are unchanged.',
              'status': 'unverified'}
    try:
        result = subprocess.run(launch_args(unit), capture_output=True, text=True, timeout=30)
        report['launch_exit'] = result.returncode
        if result.returncode == 0:
            evidence = json.loads(result.stdout)
            report['evidence'] = evidence
            report['limits_and_child_membership_verified'] = check_evidence(evidence, unit)
            report['status'] = 'verified' if report['limits_and_child_membership_verified'] else 'evidence_mismatch'
        else:
            report['status'] = 'launch_failed'
            # Fixed error categories only. Do not publish arbitrary stderr.
            report['failure_hint'] = 'user_manager_or_unit_start_failed'
    except subprocess.TimeoutExpired:
        report['status'] = 'timeout'
    except (OSError, ValueError, TypeError, AttributeError):
        report['status'] = 'unavailable_or_invalid_evidence'
    finally:
        report['cleanup'] = cleanup(unit)
        if not report['cleanup']['verified'] and report['status'] == 'verified':
            report['status'] = 'cleanup_unconfirmed'
    report['notes'] = [
        '64 tasks includes threads. Swap policy is unchanged and is not verified by this probe.',
        'No OOM or process-exhaustion test is performed; verification reads kernel limits and membership.',
        'If cleanup is unconfirmed, inspect only the named unit. RuntimeMaxSec is 20 seconds.',
    ]
    return report


if __name__ == '__main__':
    if sys.argv[1:] == ['--child']:
        print(json.dumps(snapshot()))
    elif sys.argv[1:] == ['--payload']:
        payload()
    elif sys.argv[1:] == ['--run']:
        result = verify()
        print(json.dumps(result, ensure_ascii=False, indent=2))
        sys.exit(0 if result['status'] == 'verified' else 1)
    else:
        print('Explicit test: python3 scripts/verify-wsl-resources.py --run\n'
              'Creates one temporary systemd user service with a small parent/child Python probe.\n'
              'No project work or persistent configuration changes. No sudo.')
