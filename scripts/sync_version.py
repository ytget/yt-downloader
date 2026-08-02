#!/usr/bin/env python3
"""Sync VERSION / env VERSION into wails.json info.productVersion."""
from __future__ import annotations

import json
import os
import pathlib
import sys


def main() -> int:
    version = os.environ.get("VERSION", "").strip()
    if not version:
        version_path = pathlib.Path("VERSION")
        if not version_path.is_file():
            print("VERSION env or VERSION file required", file=sys.stderr)
            return 1
        version = version_path.read_text(encoding="utf-8").strip()
    if not version:
        print("empty version", file=sys.stderr)
        return 1

    path = pathlib.Path("wails.json")
    data = json.loads(path.read_text(encoding="utf-8"))
    info = data.setdefault("info", {})
    if info.get("productVersion") != version:
        info["productVersion"] = version
        path.write_text(json.dumps(data, indent=2) + "\n", encoding="utf-8")
        print(f"wails.json productVersion -> {version}")
    else:
        print(f"wails.json productVersion already {version}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
