"""Publish a verified Go source release, preserving existing tags and assets."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import re
import subprocess

REPOSITORY = "DataScienceResearchPeru/peru-sdk-go"


def gh(*arguments, data=None, optional=False):
    result = subprocess.run(["gh", *arguments], input=None if data is None else json.dumps(data),
                            capture_output=True, text=True)
    if result.returncode:
        if optional and "404" in result.stderr:
            return None
        raise SystemExit(result.stderr)
    return json.loads(result.stdout) if result.stdout.strip() else None


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--artifact", type=Path, required=True)
    args = parser.parse_args()
    if os.environ.get("GITHUB_REPOSITORY") != REPOSITORY or os.environ.get("GITHUB_REF") != "refs/heads/main":
        raise SystemExit("publication is restricted to the SDK repository main branch")
    artifact = args.artifact.resolve()
    for line in (artifact / "manifest/SHA256SUMS").read_text().splitlines():
        digest, name = line.split("  ", 1)
        path = Path(name)
        if path.is_absolute() or ".." in path.parts or hashlib.sha256((artifact / path).read_bytes()).hexdigest() != digest:
            raise SystemExit("artifact integrity mismatch")
    release = json.loads((artifact / "manifest/release.json").read_text())
    version, commit = release["version"], release["commit"]
    if not re.fullmatch(r"[01]\.[0-9]+\.[0-9]+(?:-[0-9A-Za-z.-]+)?", version):
        raise SystemExit("invalid version or unsupported module major version")
    if commit != os.environ["GITHUB_SHA"]:
        raise SystemExit("artifact does not belong to this workflow commit")
    main_ref = gh("api", f"repos/{REPOSITORY}/git/ref/heads/main")
    if main_ref["object"]["sha"] != commit:
        raise SystemExit("main advanced; run the verified release from its current version commit")
    tag = "v" + version
    existing = gh("api", f"repos/{REPOSITORY}/git/ref/tags/{tag}", optional=True)
    if existing:
        if existing["object"]["type"] != "commit" or existing["object"]["sha"] != commit:
            raise SystemExit("existing tag belongs to another commit; tags are immutable")
    else:
        gh("api", "--method", "POST", f"repos/{REPOSITORY}/git/refs", "--input", "-",
           data={"ref": "refs/tags/" + tag, "sha": commit})
    published = gh("api", f"repos/{REPOSITORY}/releases/tags/{tag}", optional=True)
    if published is None:
        notes = artifact / "manifest/RELEASE_NOTES.md"
        notes.write_text(f"Native Go SDK {version}.\n\nModule: `{release['module']}`\n\nSource commit: `{commit}`\n\nIncludes pinned models and synthetic conformance cases. Institutional live compatibility remains unverified.\n")
        command = ["gh", "release", "create", tag, "--repo", REPOSITORY, "--verify-tag",
                   "--title", tag, "--notes-file", str(notes)]
        if "-" in version:
            command.append("--prerelease")
        subprocess.run(command, check=True)
        published = gh("api", f"repos/{REPOSITORY}/releases/tags/{tag}")
    assets = {asset["name"]: asset for asset in published["assets"]}
    for path in [*sorted((artifact / "dist").iterdir()), artifact / "manifest/release.json", artifact / "manifest/SHA256SUMS"]:
        digest = "sha256:" + hashlib.sha256(path.read_bytes()).hexdigest()
        if path.name in assets:
            if assets[path.name].get("digest") != digest:
                raise SystemExit("existing release asset differs; refusing overwrite")
        else:
            subprocess.run(["gh", "release", "upload", tag, str(path), "--repo", REPOSITORY], check=True)
    print(f"Published {REPOSITORY}@{tag}")


if __name__ == "__main__":
    main()
