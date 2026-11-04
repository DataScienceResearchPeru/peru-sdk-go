"""Test an isolated source archive and an external Go consumer; no parent checkout."""
import argparse
import gzip
import hashlib
import io
import json
import os
from pathlib import Path
import shutil
import subprocess
import tarfile
import tempfile

ROOT = Path(__file__).resolve().parents[1]
MODULE = "github.com/DataScienceResearchPeru/peru-sdk-go"


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--output", type=Path, required=True)
    args = parser.parse_args()
    output = args.output.resolve()
    if output.exists() and any(output.iterdir()):
        raise SystemExit("distribution destination must be empty")
    dist, metadata = output / "dist", output / "manifest"
    dist.mkdir(parents=True)
    metadata.mkdir()
    files = subprocess.check_output(["git", "ls-files", "-z"], cwd=ROOT).decode().split("\0")
    archive_path = dist / "peru-sdk-go-source.tar.gz"
    with archive_path.open("wb") as raw, gzip.GzipFile(fileobj=raw, mode="wb", mtime=0, filename="") as compressed:
        with tarfile.open(fileobj=compressed, mode="w", format=tarfile.USTAR_FORMAT) as archive:
            for name in sorted(filter(None, files)):
                if name.startswith(("src/peru_sdk/", "sdk-python/")) or name.endswith((".whl", ".jar")):
                    raise SystemExit("foreign SDK in Go distribution")
                data = (ROOT / name).read_bytes()
                info = tarfile.TarInfo(name)
                info.size, info.mode, info.mtime = len(data), 0o644, 0
                archive.addfile(info, io.BytesIO(data))
    Path("/var/tmp").mkdir(parents=True, exist_ok=True)
    with tempfile.TemporaryDirectory(prefix="peru-go-package-", dir="/var/tmp") as directory:
        task = Path(directory)
        source = task / "source"
        with tarfile.open(archive_path) as archive:
            for member in archive.getmembers():
                path = Path(member.name)
                if path.is_absolute() or ".." in path.parts or not member.isfile():
                    raise SystemExit("unsafe source archive entry")
                target = source / path
                target.parent.mkdir(parents=True, exist_ok=True)
                target.write_bytes(archive.extractfile(member).read())
        if not (source / "internal/conformance/cases.json").is_file():
            raise SystemExit("module is missing its independent test corpus")
        env = {**os.environ, "TMPDIR": directory, "TEMP": directory, "TMP": directory}

        def run(*command, cwd=source, extra=None):
            subprocess.run(command, cwd=cwd, env={**env, **(extra or {})}, check=True)

        run("go", "test", "-race", "./...")
        run("go", "test", "./...", extra={"CGO_ENABLED": "0"})
        consumer = task / "consumer"
        consumer.mkdir()
        shutil.copyfile(source / "examples/offline/main.go", consumer / "main.go")
        run("go", "mod", "init", "consumer.example/peru", cwd=consumer)
        run("go", "mod", "edit", "-require=" + MODULE + "@v0.0.0", "-replace=" + MODULE + "=" + str(source), cwd=consumer)
        run("go", "run", ".", cwd=consumer, extra={"CGO_ENABLED": "0"})
    commit = subprocess.check_output(["git", "rev-parse", "HEAD"], cwd=ROOT, text=True).strip()
    release = {"module": MODULE, "version": (ROOT / "VERSION").read_text().strip(), "commit": commit,
               "upstreams": json.loads((ROOT / "upstream.lock.json").read_text())}
    (metadata / "release.json").write_text(json.dumps(release, indent=2) + "\n")
    sums = [f"{hashlib.sha256(path.read_bytes()).hexdigest()}  {path.relative_to(output).as_posix()}"
            for path in sorted(output.rglob("*")) if path.is_file()]
    (metadata / "SHA256SUMS").write_text("\n".join(sums) + "\n")
    print("Verified isolated Go distribution:", output)


if __name__ == "__main__":
    main()
