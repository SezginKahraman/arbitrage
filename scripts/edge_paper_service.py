#!/usr/bin/env python3
"""Manage the bounded, public-data-only paper experiment on macOS launchd."""
import argparse
import json
import os
from pathlib import Path
import plistlib
import subprocess
import sys

ROOT = Path(__file__).resolve().parent.parent
LABEL = "com.arbitrage.edge-paper"
OUTPUT = ROOT / ".local" / "edge-paper"
PLIST = Path.home() / "Library" / "LaunchAgents" / (LABEL + ".plist")
DOMAIN = "gui/" + str(os.getuid())


def command(*args):
    return subprocess.run(["/bin/launchctl", *args], capture_output=True, text=True)


def owned_plist():
    if not PLIST.exists():
        return False
    with PLIST.open("rb") as handle:
        config = plistlib.load(handle)
    if config.get("WorkingDirectory") != str(ROOT) or config.get("Label") != LABEL:
        raise RuntimeError("Existing launch agent does not belong to this workspace")
    return True


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("action", choices=["install", "status", "stop"])
    args = parser.parse_args()
    if sys.platform != "darwin":
        parser.error("This service wrapper requires macOS launchd")
    if args.action == "install":
        runner = ROOT / "scripts" / "edge_paper.py"
        if not runner.is_file():
            parser.error("Paper engine is missing")
        if owned_plist():
            parser.error("Service already installed; inspect status or stop it first")
        if command("print", DOMAIN + "/" + LABEL).returncode == 0:
            parser.error("A service with this label already exists")
        OUTPUT.mkdir(parents=True, exist_ok=True)
        PLIST.parent.mkdir(parents=True, exist_ok=True)
        config = {
            "Label": LABEL,
            "ProgramArguments": [sys.executable, "-u", str(runner),
                                 "--interval", "300", "--duration-hours", "24",
                                 "--nominal", "100", "--max-coins", "12",
                                 "--output-dir", str(OUTPUT)],
            "WorkingDirectory": str(ROOT),
            "RunAtLoad": True,
            "KeepAlive": False,
            "ProcessType": "Background",
            "StandardOutPath": str(OUTPUT / "service.stdout.log"),
            "StandardErrorPath": str(OUTPUT / "service.stderr.log"),
        }
        with PLIST.open("xb") as handle:
            plistlib.dump(config, handle)
        os.chmod(PLIST, 0o600)
        result = command("bootstrap", DOMAIN, str(PLIST))
        if result.returncode:
            PLIST.unlink()
            raise RuntimeError("launchctl bootstrap failed: " + result.stderr.strip())
        print("Installed 24-hour paper experiment, interval 300 seconds.")
    elif args.action == "stop":
        if owned_plist():
            result = command("bootout", DOMAIN + "/" + LABEL)
            if result.returncode and command("print", DOMAIN + "/" + LABEL).returncode == 0:
                raise RuntimeError("Could not stop service: " + result.stderr.strip())
            PLIST.unlink()
        print("Paper service removed; recorded data retained at " + str(OUTPUT))
        return
    status = command("print", DOMAIN + "/" + LABEL)
    print("Service loaded: " + str(status.returncode == 0))
    for line in status.stdout.splitlines():
        if line.strip().startswith(("state =", "pid =", "last exit code =", "runs =")):
            print(line.strip())
    print("Report: " + str(OUTPUT / "latest.md"))
    print("State: " + str(OUTPUT / "state.json"))
    report = OUTPUT / "latest.json"
    if report.exists():
        try:
            value = json.loads(report.read_text())
            print("Latest report keys: " + ", ".join(sorted(value)))
        except (ValueError, OSError) as error:
            print("Report unavailable: " + str(error))


if __name__ == "__main__":
    main()
