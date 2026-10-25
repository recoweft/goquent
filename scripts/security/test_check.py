"""Harmless process fixtures exercise the real capture and publication boundary."""
import contextlib
import io
import json
import os
from pathlib import Path
import stat
import tempfile
import unittest
from unittest.mock import patch

import check

CANARY = "fixture-private-raw@example.test"


def stream(level=None):
    messages = [{"config": {"protocol_version": "v1.0.0", "scanner_name": "govulncheck",
                 "scanner_version": check.SCANNER, "go_version": check.GO,
                 "db": check.DB, "scan_level": "symbol", "scan_mode": "source",
                 "db_last_modified": "2026-10-21T00:27:29Z"}},
                {"SBOM": {"go_version": check.GO, "roots": ["fixture"]}},
                {"progress": {"message": CANARY}}]
    if level:
        frame = {"module": "fixture", "position": {"filename": CANARY}}
        if level in ("package", "symbol"):
            frame["package"] = "fixture/package"
        if level == "symbol":
            frame["function"] = "Function"
        messages.append({"finding": {"osv": "GO-2026-1234", "trace": [frame]}})
    return "\n".join(json.dumps(m) for m in messages)


class BoundaryTests(unittest.TestCase):
    def test_levels_and_deduplication(self):
        for level in (None, "module", "package", "symbol"):
            with self.subTest(level=level):
                raw = stream(level)
                if level:
                    raw += '\n' + raw.splitlines()[-1]
                result = check.summary(raw)
                self.assertEqual(result["symbol"], int(level == "symbol"))
                self.assertEqual(result["package_only"], int(level == "package"))
                self.assertEqual(result["module_only"], int(level == "module"))
                self.assertNotIn(CANARY, str(result))

    def test_invalid_and_incomplete_results(self):
        for raw in ("", "{}", stream()[:-4], stream().replace(check.DB, "untrusted"),
                    stream().replace("db_last_modified", "missing"),
                    stream().replace(check.GO, "go1.25.0")):
            with self.subTest(raw_length=len(raw)), self.assertRaises((check.CheckError, ValueError)):
                check.summary(raw)

    def test_process_failure_partial_timeout_and_capture_failure(self):
        with tempfile.TemporaryDirectory() as tmp:
            directory = Path(tmp)
            env = dict(os.environ)
            code = check.capture(["sh", "-c", f"printf '{CANARY}'; printf '{CANARY}' >&2; exit 7"], directory, "partial", env, tmp)
            self.assertEqual(code, 7)
            self.assertIn(CANARY, check.read(directory, "partial"))
            with self.assertRaisesRegex(check.CheckError, "TIMEOUT"):
                check.capture(["sh", "-c", f"printf '{CANARY}'; sleep 20"], directory, "timeout", env, tmp, timeout=0.05)
            with self.assertRaisesRegex(check.CheckError, "EXECUTION_ERROR"):
                check.capture(["/nonexistent-fixture-command"], directory, "missing", env, tmp)
            with self.assertRaisesRegex(check.CheckError, "EXECUTION_ERROR"):
                check.capture(["true"], directory / "absent", "capture", env, tmp)

    def test_end_to_end_stub(self):
        for scenario, expected in [("clean", 0), ("module", 0), ("symbol", 3),
                                   ("execution", 2), ("partial", 2),
                                   ("no_db", 2), ("install", 2), ("wrong_go", 2),
                                   ("gate_error", 2)]:
            with self.subTest(scenario=scenario), tempfile.TemporaryDirectory() as tmp:
                directory = Path(tmp)
                tools = directory / "tools"
                tools.mkdir()
                scanner = directory / "scanner"
                raw = stream(scenario if scenario in ("module", "symbol") else None)
                if scenario == "no_db":
                    raw = raw.replace("db_last_modified", "missing")
                if scenario == "partial":
                    raw = raw[:-4]
                scanner.write_text("#!/usr/bin/python3\nimport sys\n"
                                   f"print({CANARY!r}, file=sys.stderr)\n"
                                   "if '-mode' in sys.argv:\n"
                                   f"    sys.exit({1 if scenario == 'gate_error' else 3 if scenario == 'symbol' else 0})\n"
                                   f"print({raw!r})\n"
                                   f"sys.exit({1 if scenario == 'execution' else 0})\n")
                scanner.chmod(0o700)
                go = tools / "go"
                go.write_text("#!/usr/bin/python3\nimport json,os,pathlib,shutil,sys\n"
                              f"print({CANARY!r},file=sys.stderr)\n"
                              "if sys.argv[1] == 'env':\n"
                              f"    print(json.dumps({dict(GOVERSION='go1.25.0' if scenario == 'wrong_go' else check.GO, GOTOOLCHAIN='local')!r}))\n"
                              "elif sys.argv[1] == 'install':\n"
                              f"    sys.exit(9) if {scenario == 'install'} else None\n"
                              "    pathlib.Path(os.environ['GOBIN']).mkdir()\n"
                              f"    shutil.copy({str(scanner)!r}, pathlib.Path(os.environ['GOBIN'])/'govulncheck')\n"
                              "else:\n"
                              f"    print('fixture: {check.GO}\\n\\tmod\\tgolang.org/x/vuln\\t{check.SCANNER}\\th1:fixture')\n")
                go.chmod(0o700)
                capture_dir = directory / "capture"
                capture_dir.mkdir(mode=0o700)
                output = io.StringIO()
                old_umask = os.umask(0o077)
                try:
                    with patch.dict(os.environ, PATH=str(tools) + os.pathsep + os.environ["PATH"]), patch.object(check.tempfile, "mkdtemp", return_value=str(capture_dir)), contextlib.redirect_stdout(output), contextlib.redirect_stderr(output):
                        result = check.main()
                finally:
                    os.umask(old_umask)
                self.assertEqual(result, expected)
                self.assertNotIn(CANARY, output.getvalue())
                self.assertNotIn(str(directory), output.getvalue())
                for path in capture_dir.glob('*.out'):
                    self.assertEqual(stat.S_IMODE(path.stat().st_mode), 0o600)

    def test_exception_never_published(self):
        output = io.StringIO()
        with patch.object(check.tempfile, "mkdtemp", side_effect=OSError(CANARY)), contextlib.redirect_stdout(output):
            self.assertEqual(check.main(), 2)
        self.assertNotIn(CANARY, output.getvalue())


if __name__ == "__main__":
    unittest.main()
