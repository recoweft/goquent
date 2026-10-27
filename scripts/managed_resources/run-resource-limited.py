"""Resource envelope only, not a filesystem/network/approval sandbox."""
import argparse
import importlib.util
import json
import os
from pathlib import Path
import subprocess
import sys
import uuid

spec = importlib.util.spec_from_file_location('probe', Path(__file__).with_name('verify-wsl-resources.py'))
probe = importlib.util.module_from_spec(spec)
spec.loader.exec_module(probe)


def emit(value):
    print('RELAYWEFT_RESOURCE ' + json.dumps(value, ensure_ascii=False), file=sys.stderr, flush=True)


def valid_limits(snapshot, unit):
    return (snapshot.get('resolved') is True
            and snapshot.get('memory_max') == probe.MEMORY
            and snapshot.get('pids_max') == probe.TASKS
            and Path(snapshot.get('cgroup_path', '')).name == unit)


def guard(unit, cwd, command):
    evidence = probe.snapshot()
    if not valid_limits(evidence, unit):
        emit({'stage': 'before_exec', 'status': 'limit_mismatch', 'unit': unit})
        return 125
    os.chdir(cwd)
    emit({'stage': 'before_exec', 'status': 'limits_verified', 'unit': unit,
          'memory_max': evidence['memory_max'], 'pids_max': evidence['pids_max']})
    # No shell, PATH lookup, sudo, retries, or unrestricted fallback.
    os.execv(command[0], command)
    return 125


def launch_args(unit, cwd, seconds, command):
    return ['systemd-run', '--user', '--no-ask-password', '--quiet', '--wait', '--pipe', '--collect',
            '--service-type=exec', '--expand-environment=no', '--unit=' + unit,
            '--property=MemoryMax=' + probe.MEMORY, '--property=TasksMax=' + probe.TASKS,
            '--property=RuntimeMaxSec=' + str(seconds) + 's', '--property=TimeoutStopSec=3s',
            '--property=KillMode=control-group', '--property=UMask=0077',
            sys.executable, str(Path(__file__).resolve()), '--guard', unit, cwd, *command]


def run(cwd, seconds, command):
    unit = 'relayweft-resource-run-' + uuid.uuid4().hex + '.service'
    emit({'stage': 'launch', 'unit': unit, 'memory_max': probe.MEMORY,
          'pids_max': probe.TASKS, 'deadline_seconds': seconds,
          'notice': 'Resource limits only. No comparison admission or security isolation claim.'})
    child = None
    exit_code = 125
    cleanup = {'verified': False}
    try:
        # Output streams directly to caller: do not buffer or automatically upload raw results.
        child = subprocess.Popen(launch_args(unit, cwd, seconds, command), stdin=subprocess.DEVNULL)
        exit_code = child.wait(timeout=seconds + 10)
    except subprocess.TimeoutExpired:
        emit({'stage': 'wait', 'status': 'timeout'})
        exit_code = 124
    except KeyboardInterrupt:
        emit({'stage': 'wait', 'status': 'interrupted'})
        exit_code = 130
    except OSError:
        emit({'stage': 'launch', 'status': 'launch_unavailable'})
    finally:
        cleanup = probe.cleanup(unit)
        if child is not None and child.poll() is None:
            # Stop only the local client after requesting stop of the exact unit.
            child.terminate()
            try:
                child.wait(timeout=3)
            except subprocess.TimeoutExpired:
                child.kill()
                child.wait()
        emit({'stage': 'finished', 'unit': unit, 'launch_exit': exit_code,
              'cleanup_verified': cleanup['verified'],
              'comparison_acceptance': 'not_evaluated'})
    if not cleanup['verified']:
        return 125
    return exit_code if 0 <= exit_code <= 255 else 125


def main():
    if sys.argv[1:2] == ['--guard']:
        if len(sys.argv) < 5:
            return 125
        unit, cwd, *command = sys.argv[2:]
        if not unit.startswith('relayweft-resource-run-') or not unit.endswith('.service'):
            return 125
        try:
            return guard(unit, cwd, command)
        except OSError:
            emit({'stage': 'before_exec', 'status': 'cwd_or_executable_unavailable'})
            return 125
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--cwd', required=True, help='Existing absolute working directory')
    parser.add_argument('--timeout-seconds', required=True, type=int, help='Explicit total deadline (1..3600)')
    parser.add_argument('command', nargs=argparse.REMAINDER)
    args = parser.parse_args()
    command = args.command[1:] if args.command[:1] == ['--'] else args.command
    if not Path(args.cwd).is_absolute() or not Path(args.cwd).is_dir():
        parser.error('--cwd must be an existing absolute directory')
    if not command or not Path(command[0]).is_absolute():
        parser.error('supply -- followed by an absolute executable and its arguments')
    if not 1 <= args.timeout_seconds <= 3600:
        parser.error('--timeout-seconds must be between 1 and 3600')
    return run(args.cwd, args.timeout_seconds, command)


if __name__ == '__main__':
    sys.exit(main())
