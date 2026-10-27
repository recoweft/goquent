"""Read-only resource-control preflight. No cgroups or services are created."""
import json
import os
from pathlib import Path
import platform
import subprocess
from datetime import datetime, timezone


def read(path):
    try:
        return Path(path).read_text().strip()
    except OSError:
        return None


def command(args):
    try:
        result = subprocess.run(args, capture_output=True, text=True, timeout=5,
                                env={**os.environ, 'LC_ALL': 'C'})
        return {'status': 'ok' if result.returncode == 0 else 'unavailable',
                'exit_code': result.returncode,
                'properties': dict(line.split('=', 1) for line in result.stdout.splitlines()
                                   if '=' in line) if result.returncode == 0 else {}}
    except subprocess.TimeoutExpired:
        return {'status': 'timeout', 'properties': {}}
    except OSError:
        return {'status': 'unavailable', 'properties': {}}


def locate_cgroup(membership, mountinfo):
    """Resolve mount root as well as mount point; do not guess on namespaces."""
    if membership is None or mountinfo is None:
        return None
    relative = next((line[3:] for line in membership.splitlines() if line.startswith('0::')), None)
    if relative is None or '..' in Path(relative).parts:
        return None
    for line in mountinfo.splitlines():
        fields = line.split()
        sep = fields.index('-') if '-' in fields else -1
        if sep < 6 or fields[sep + 1] != 'cgroup2':
            continue
        def decode(value):
            for escaped, actual in [('\\040', ' '), ('\\011', '\t'), ('\\012', '\n'), ('\\134', '\\')]:
                value = value.replace(escaped, actual)
            return value
        root, mount = map(decode, (fields[3], fields[4]))
        try:
            suffix = Path(relative).relative_to(root)
        except ValueError:
            continue
        return Path(mount) / suffix, Path(mount), 'ro' in fields[5].split(',')
    return None


def inspect_tree(current, mount):
    levels = []
    while True:
        levels.append({
            'level': len(levels),
            'controllers': read(current / 'cgroup.controllers'),
            'subtree_control': read(current / 'cgroup.subtree_control'),
            'memory_max': read(current / 'memory.max'),
            'memory_swap_max': read(current / 'memory.swap.max'),
            'pids_max': read(current / 'pids.max'),
            'memory_max_writable': os.access(current / 'memory.max', os.W_OK),
            'pids_max_writable': os.access(current / 'pids.max', os.W_OK),
        })
        if current == mount:
            break
        current = current.parent
    return levels


def diagnose():
    release = platform.release()
    resolved = locate_cgroup(read('/proc/self/cgroup'), read('/proc/self/mountinfo'))
    pid1 = read('/proc/1/comm')
    user_manager = command(['systemctl', '--user', 'show', '--no-pager',
                            '--property=Version', '--property=ControlGroup'])
    delegation = command(['systemctl', 'show', f'user@{os.getuid()}.service', '--no-pager',
                          '--property=ActiveState', '--property=Delegate',
                          '--property=DelegateControllers', '--property=ControlGroup'])
    levels = inspect_tree(resolved[0], resolved[1]) if resolved else []
    controllers = set((levels[0]['controllers'] or '').split()) if levels else set()
    candidate = (pid1 == 'systemd' and user_manager['status'] == 'ok'
                 and resolved is not None and not resolved[2])
    return {
        'schema_version': 1,
        'observed_at': datetime.now(timezone.utc).isoformat(),
        'scope': 'This diagnostic process only; not evidence about an existing RelayWeft worker.',
        'kernel_release': release,
        'wsl2_kernel_detected': 'microsoft-standard' in release.lower() and 'wsl2' in release.lower(),
        'pid1': pid1,
        'cgroup_v2_resolved': resolved is not None,
        'cgroup_mount_readonly': resolved[2] if resolved else None,
        'visible_cgroup_ancestors': levels,
        'current_memory_controller_available': 'memory' in controllers,
        'current_pids_controller_available': 'pids' in controllers,
        'user_manager': user_manager,
        'user_service_delegation': delegation,
        'assessment': 'user_manager_route_candidate' if candidate else 'prerequisites_need_inspection',
        'enforcement_verified': False,
        'notes': [
            'Read-only diagnostic: no sudo, configuration changes, service starts, or limit writes.',
            'A writable current cgroup is not required to use a separate systemd-managed unit.',
            'A reachable user manager does not prove memory/pids delegation or successful enforcement.',
            'max is local to that cgroup; ancestors outside the visible mount are unknown.',
            'pids limits count threads too. Swap requirements must be specified separately.',
            'Exit 0 means diagnostics completed, not that the requested limits can be enforced.',
        ],
    }


if __name__ == '__main__':
    print(json.dumps(diagnose(), ensure_ascii=False, indent=2))
