"""Linux-only harmless descendant probe; no scanner or network acquisition."""
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest


@unittest.skipUnless(sys.platform == "linux", "Linux process cleanup probe")
class ProcessBoundaryTests(unittest.TestCase):
    def test_timeout_stops_and_reaps_descendant(self):
        # Isolate subreaper state and environment from the test runner. The child
        # has its own finite lifetime even if a future capture regression occurs.
        probe = r'''
import ctypes, os, signal, sys, time
from pathlib import Path
import check
assert ctypes.CDLL(None).prctl(36, 1, 0, 0, 0) == 0
directory = Path(sys.argv[1])
child = "import time; time.sleep(5)"
parent = ("import subprocess,sys,time,pathlib; "
          "p=subprocess.Popen([sys.executable,'-c'," + repr(child) + "]); "
          "pathlib.Path('child.pid').write_text(str(p.pid)); p.wait()")
try:
    check.capture([sys.executable, '-c', parent], directory, 'group',
                  {'PATH':'/usr/bin:/bin'}, str(directory), timeout=1)
    raise AssertionError('timeout not observed')
except check.CheckError as error:
    assert str(error) == 'TIMEOUT'
pid = int((directory / 'child.pid').read_text())
try:
    deadline = time.monotonic() + 2
    while time.monotonic() < deadline:
        got, status = os.waitpid(pid, os.WNOHANG)
        if got:
            assert os.WIFSIGNALED(status) and os.WTERMSIG(status) == signal.SIGKILL
            pid = 0
            break
        time.sleep(.01)
    assert pid == 0
finally:
    if pid:
        os.kill(pid, signal.SIGKILL)
        os.waitpid(pid, 0)
'''
        with tempfile.TemporaryDirectory() as tmp:
            result = subprocess.run(
                [sys.executable, "-c", probe, tmp],
                cwd=Path(__file__).resolve().parent,
                env={"PATH": "/usr/bin:/bin", "PYTHONDONTWRITEBYTECODE": "1"},
                stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL,
                timeout=12,
            )
            self.assertEqual(result.returncode, 0, "isolated group cleanup failed")
