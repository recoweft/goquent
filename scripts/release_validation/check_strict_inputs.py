#!/usr/bin/env python3
"""Real CLI regression for the companion registry's finite diff input contract."""
import json
import os
from pathlib import Path
import subprocess
import tempfile

import run


def check(root, output, registry_path='tests/manifestfixture/repositories.json'):
    collector = run.Collector(root, output)
    if collector.build():
        raise ValueError('build')
    registry = (root / registry_path).read_bytes()
    if not registry.endswith(b'\n'):
        raise ValueError('registry termination')
    base, head = output / 'base', output / 'head'
    for tree in (base, head):
        tree.mkdir(mode=0o700)
        (tree / 'query.sql').write_text('DELETE FROM CANARY_STRICT_TABLE;\n')
    target = head / 'repositories.json'
    patch = output / 'change.diff'
    # This is the actual committed registry, not a regenerated expectation.
    def invoke(content, unlisted=False):
        target.write_bytes(content)
        lines = content.decode().splitlines()
        diff = ('diff --git a/repositories.json b/repositories.json\n'
                'new file mode 100644\n--- /dev/null\n+++ b/repositories.json\n'
                '@@ -0,0 +1,%d @@\n' % len(lines))
        diff += ''.join('+' + line + '\n' for line in lines)
        if not content.endswith(b'\n'):
            diff += '\\ No newline at end of file\n'
        patch.write_text(diff)
        extra = head / 'unlisted.txt'
        if unlisted:
            extra.write_text('CANARY_UNLISTED\n')
        elif extra.exists():
            extra.unlink()
        code, log = collector.command([str(collector.cli), 'review', '--format',
            'review-ci-json', '--fail-on', 'high', '--fail-on-precision', 'partial',
            '--diff', str(patch), '--diff-base-root', str(base), str(head)])
        raw = log.read_text()
        if 'CANARY_' in raw or str(output) in raw:
            raise ValueError('public boundary')
        view = json.loads(raw)
        return code, view

    cases = ((registry, False, 1), (registry[:-1], False, 2),
             (registry, True, 2), (registry, False, 1))
    for content, unlisted, want in cases:
        code, view = invoke(content, unlisted)
        gate = view['gate']
        if (code != want or gate['input_failed'] != (want == 2) or
                not gate['full_report'] or not gate['risk_failed'] or
                not gate['precision_failed']):
            raise ValueError('gate regression')
    for path in output.rglob('*'):
        if path.stat().st_mode & 0o077:
            raise ValueError('private permissions')


def main():
    os.umask(0o077)
    try:
        with tempfile.TemporaryDirectory(prefix='goquent-strict-regression-') as area:
            for name, registry in (('companion', 'tests/manifestfixture/repositories.json'),
                                   ('original', 'examples/ai-safe-orm/storedmodel/repositories.json')):
                output = Path(area) / name
                output.mkdir(mode=0o700)
                check(Path(__file__).resolve().parents[2], output, registry)
        print('STRICT_INPUT_REGRESSION_PASSED: cases=8; gate failures retained')
        return 0
    except Exception:
        print('STRICT_INPUT_REGRESSION_FAILED: details omitted')
        return 1


if __name__ == '__main__':
    raise SystemExit(main())
