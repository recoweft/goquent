#!/usr/bin/env python3
"""Bounded govulncheck runner. Never publish captured diagnostics or exceptions."""
import datetime
import json
import os
from pathlib import Path
import re
import signal
import subprocess
import sys
import tempfile

GO = "go1.26.10"
SCANNER = "v1.9.0"
DB = "https://vuln.go.dev"
TIMEOUT = 300


class CheckError(Exception):
    pass


def capture(args, directory, name, env, cwd, stdin=None, timeout=TIMEOUT):
    """Capture both streams before starting; kill the process group on timeout."""
    try:
        with (directory / (name + ".out")).open("xb") as out, (directory / (name + ".err")).open("xb") as err:
            process = subprocess.Popen(args, cwd=cwd, env=env, stdin=stdin,
                                       stdout=out, stderr=err, start_new_session=True)
            try:
                return process.wait(timeout=timeout)
            except subprocess.TimeoutExpired:
                os.killpg(process.pid, signal.SIGKILL)
                process.wait()
                raise CheckError("TIMEOUT") from None
    except OSError:
        raise CheckError("EXECUTION_ERROR") from None


def read(directory, name):
    path = directory / (name + ".out")
    if path.stat().st_size > 64 * 1024 * 1024:
        raise CheckError("OUTPUT_LIMIT")
    return path.read_text()


def summary(raw):
    decoder = json.JSONDecoder()
    messages = []
    while raw.strip():
        item, end = decoder.raw_decode(raw.lstrip())
        messages.append(item)
        raw = raw.lstrip()[end:]
    configs = [m["config"] for m in messages if "config" in m]
    sboms = [m["SBOM"] for m in messages if "SBOM" in m]
    if len(configs) != 1 or len(sboms) != 1:
        raise CheckError("INCOMPLETE_RESULT")
    config = configs[0]
    expected = {"protocol_version": "v1.0.0", "scanner_name": "govulncheck",
                "scanner_version": SCANNER, "go_version": GO, "db": DB,
                "scan_level": "symbol", "scan_mode": "source"}
    if any(config.get(k) != v for k, v in expected.items()):
        raise CheckError("INPUT_MISMATCH")
    modified = config.get("db_last_modified", "")
    if not re.fullmatch(r"\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d{1,9})?Z", modified):
        raise CheckError("DB_UNVERIFIED")
    if sboms[0].get("go_version") != GO or not sboms[0].get("roots"):
        raise CheckError("INCOMPLETE_RESULT")
    levels = {}
    for message in messages:
        if "finding" not in message:
            continue
        finding = message["finding"]
        advisory = finding["osv"]
        frame = finding["trace"][0]
        if not re.fullmatch(r"GO-\d{4}-\d+", advisory) or not frame.get("module"):
            raise CheckError("INVALID_RESULT")
        level = 2 if frame.get("function") else 1 if frame.get("package") else 0
        levels[advisory] = max(levels.get(advisory, 0), level)
    return {"db_modified": modified, "module_only": list(levels.values()).count(0),
            "package_only": list(levels.values()).count(1),
            "symbol": list(levels.values()).count(2)}


def run(directory):
    root = Path.cwd()
    if f"toolchain {GO}" not in (root / "go.mod").read_text().splitlines():
        raise CheckError("PREFERRED_GO_MISMATCH")
    env = dict(os.environ)
    env.pop("GOVERSION", None)
    env.update(GOTOOLCHAIN="local", GOOS="linux", GOARCH="amd64", GOAMD64="v1",
               CGO_ENABLED="1", GOFLAGS="", GOWORK="off", GOPACKAGESDRIVER="off",
               GOBIN=str(directory / "bin"), GOCACHE=str(directory / "cache"),
               GOMODCACHE=str(directory / "modules"), GOPROXY="https://proxy.golang.org",
               GOSUMDB="sum.golang.org", GOPRIVATE="", GONOPROXY="", GONOSUMDB="")
    code = capture(["go", "env", "-json", "GOVERSION", "GOTOOLCHAIN"], directory, "go", env, root)
    if code or json.loads(read(directory, "go")) != {"GOVERSION": GO, "GOTOOLCHAIN": "local"}:
        raise CheckError("BOOTSTRAP_MISMATCH")
    code = capture(["git", "rev-parse", "HEAD"], directory, "source", env, root)
    source = read(directory, "source").strip()
    if code or not re.fullmatch(r"[0-9a-f]{40}", source):
        raise CheckError("SOURCE_UNVERIFIED")
    code = capture(["git", "status", "--porcelain"], directory, "status", env, root)
    if code:
        raise CheckError("SOURCE_UNVERIFIED")
    dirty = bool(read(directory, "status").strip())
    started = datetime.datetime.now(datetime.timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")
    print(f"SECURITY_INPUT source={source} dirty={str(dirty).lower()} go={GO} GOTOOLCHAIN=local scanner={SCANNER} platform=linux/amd64 GOAMD64=v1 CGO_ENABLED=1 tags=default tests=normal_and_test started={started}", flush=True)
    code = capture(["go", "install", f"golang.org/x/vuln/cmd/govulncheck@{SCANNER}"], directory, "install", env, directory)
    if code:
        print(f"SECURITY_INSTALL_ERROR exit={code}")
        return 2
    scanner = str(directory / "bin" / "govulncheck")
    code = capture(["go", "version", "-m", scanner], directory, "build", env, directory)
    build = read(directory, "build")
    if code or not build.splitlines()[0].endswith(": " + GO) or f"\tmod\tgolang.org/x/vuln\t{SCANNER}\t" not in build:
        raise CheckError("SCANNER_BUILD_MISMATCH")
    overall = 0
    for name, flags in [("normal", []), ("test", ["-test"])]:
        code = capture([scanner, "-json", "-db", DB, *flags, "./..."], directory, name, env, root)
        print(f"SECURITY_PROCESS scope={name} json_exit={code}", flush=True)
        if code:
            overall = 2
            continue
        counts = summary(read(directory, name))
        # Use the official text handler for exit semantics, without printing text.
        with (directory / (name + ".out")).open("rb") as raw:
            gate = capture([scanner, "-mode", "convert", "-format", "text"], directory, name + "-gate", env, root, stdin=raw)
        if gate not in (0, 3) or (gate == 3) != (counts["symbol"] > 0):
            raise CheckError("GATE_MISMATCH")
        print(f"SECURITY_RESULT scope={name} gate_exit={gate} " + " ".join(f"{k}={v}" for k, v in counts.items()), flush=True)
        if gate and not overall:
            overall = gate
    return overall


def main():
    os.umask(0o077)
    try:
        # Outside the checkout, owner-only; never upload, print, or attach raw files.
        directory = Path(tempfile.mkdtemp(prefix="goquent-security-", dir="/tmp"))
        return run(directory)
    except CheckError as error:
        print("SECURITY_ERROR " + str(error))
    except Exception:
        print("SECURITY_ERROR INVALID_OR_UNAVAILABLE_RESULT")
    return 2


if __name__ == "__main__":
    sys.exit(main())
