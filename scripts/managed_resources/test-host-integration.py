"""Opt-in real systemd test. No Go loader, database or managed comparison."""
import argparse
import json
from pathlib import Path
import subprocess
import sys


def run_case(name, code, seconds, expected):
    runner = Path(__file__).with_name('run-resource-limited.py')
    completed = subprocess.run([sys.executable, str(runner), '--cwd', '/tmp',
                                '--timeout-seconds', str(seconds), '--',
                                sys.executable, '-c', code], capture_output=True, text=True, timeout=70)
    events = [json.loads(line.removeprefix('RELAYWEFT_RESOURCE '))
              for line in completed.stderr.splitlines() if line.startswith('RELAYWEFT_RESOURCE ')]
    gate = next((e for e in events if e.get('stage') == 'before_exec'), {})
    final = next((e for e in events if e.get('stage') == 'finished'), {})
    exit_ok = completed.returncode == expected if expected is not None else completed.returncode != 0
    ok = (gate.get('status') == 'limits_verified' and final.get('cleanup_verified') is True
          and exit_ok)
    return {'case': name, 'passed': ok, 'exit': completed.returncode,
            'limits_verified': gate.get('status') == 'limits_verified',
            'cleanup_verified': final.get('cleanup_verified', False)}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--run', action='store_true', help='Create three temporary user services')
    if not parser.parse_args().run:
        parser.print_help()
        return 0
    results = []
    for args in [
        ('normal', 'import subprocess,sys; subprocess.run([sys.executable,"-c","pass"],check=True)', 10, 0),
        ('command_failure', 'raise SystemExit(7)', 10, 7),
        ('deadline', 'import time; time.sleep(30)', 1, None),
    ]:
        try:
            result = run_case(*args)
        except (subprocess.TimeoutExpired, ValueError, OSError):
            result = {'case': args[0], 'passed': False, 'reason': 'test_unconfirmed'}
        results.append(result)
        if not result['passed']:
            break
    print(json.dumps({'resource_stage_only': True, 'managed_comparison_run': False,
                      'cases': results, 'passed': len(results) == 3 and all(r['passed'] for r in results)}, indent=2))
    return 0 if len(results) == 3 and all(r['passed'] for r in results) else 1


if __name__ == '__main__':
    sys.exit(main())
