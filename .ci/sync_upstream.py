"""Explicit, integrity-checked development synchronization; never used at runtime."""
import argparse
import hashlib
import json
from pathlib import Path
import re
import subprocess
import sys
import tempfile
from urllib.request import urlopen

ROOT = Path(__file__).resolve().parents[1]


def fetch(source: dict, path: str, digest: str) -> bytes:
    repository, revision = source["repository"], source["revision"]
    if not re.fullmatch(r"DataScienceResearchPeru/peru-sdk-(models|conformance)", repository):
        raise ValueError("unexpected upstream repository")
    if not re.fullmatch(r"[0-9a-f]{40}", revision) or not re.fullmatch(r"[0-9a-f]{64}", digest):
        raise ValueError("upstreams require full commits and SHA-256 hashes")
    relative = Path(path)
    if relative.is_absolute() or ".." in relative.parts:
        raise ValueError("invalid upstream path")
    with urlopen(f"https://raw.githubusercontent.com/{repository}/{revision}/{path}", timeout=30) as response:
        data = response.read(2 * 1024 * 1024 + 1)
    if len(data) > 2 * 1024 * 1024 or hashlib.sha256(data).hexdigest() != digest:
        raise ValueError(f"upstream integrity mismatch: {path}")
    return data


def synchronize(check: bool):
    lock = json.loads((ROOT / "upstream.lock.json").read_text(encoding="utf-8"))
    if lock["format_version"] != 1:
        raise ValueError("unsupported lock format")
    case_path = Path(lock["cases_file"])
    if case_path.is_absolute() or ".." in case_path.parts:
        raise ValueError("invalid corpus destination")
    source = lock["conformance"]
    corpus = fetch(source, source["path"], source["sha256"])
    outputs = {ROOT / case_path: corpus}
    Path("/var/tmp").mkdir(parents=True, exist_ok=True)
    with tempfile.TemporaryDirectory(prefix="peru-sdk-upstream-", dir="/var/tmp") as directory:
        task = Path(directory)
        if "generator" in lock:
            language = lock["language"]
            if language not in ("go", "python"):
                raise ValueError("unsupported language")
            generator = lock["generator"]
            for name, digest in generator["files"].items():
                data = fetch(generator, name, digest)
                target = task / "models" / name
                target.parent.mkdir(parents=True, exist_ok=True)
                target.write_bytes(data)
            destination = task / "output"
            subprocess.run([sys.executable, str(task / "models/tools/generate.py"),
                            "--language", language, "--output", str(destination)], check=True)
            root = ROOT if language == "go" else ROOT / "src/peru_sdk"
            for path in destination.rglob("*"):
                if path.is_file():
                    outputs[root / path.relative_to(destination)] = path.read_bytes()
        differences = []
        for path, content in outputs.items():
            if check:
                if not path.exists() or path.read_bytes() != content:
                    differences.append(str(path.relative_to(ROOT)))
            else:
                path.parent.mkdir(parents=True, exist_ok=True)
                path.write_bytes(content)
        if differences:
            raise ValueError("regeneration required: " + ", ".join(differences))
    print("Pinned upstreams and local outputs match." if check else "Updated from pinned upstreams.")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--check", action="store_true")
    args = parser.parse_args()
    try:
        synchronize(args.check)
    except (ValueError, KeyError, OSError) as error:
        parser.exit(1, str(error) + "\n")


if __name__ == "__main__":
    main()
