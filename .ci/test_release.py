import hashlib
import importlib.util
import json
import os
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch

SPEC = importlib.util.spec_from_file_location("release_publish", Path(__file__).with_name("publish.py"))
release = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(release)


class ReleaseGuardTests(unittest.TestCase):
    def test_other_repositories_cannot_publish(self):
        with patch.dict(os.environ, {"GITHUB_REPOSITORY": "example/fork", "GITHUB_REF": "refs/heads/main"}):
            with patch("sys.argv", ["publish.py", "--artifact", "/nonexistent"]):
                with patch.object(release, "gh") as gh:
                    with self.assertRaises(SystemExit):
                        release.main()
                    gh.assert_not_called()

    def test_tampered_artifact_never_reaches_github(self):
        Path("/var/tmp").mkdir(parents=True, exist_ok=True)
        with tempfile.TemporaryDirectory(prefix="peru-release-guard-", dir="/var/tmp") as directory:
            root = Path(directory)
            (root / "manifest").mkdir()
            (root / "manifest/release.json").write_text('{"version":"0.1.0-alpha.1"}')
            digest = hashlib.sha256(b"original bytes").hexdigest()
            (root / "manifest/SHA256SUMS").write_text(f"{digest}  manifest/release.json\n")
            environment = {"GITHUB_REPOSITORY": release.REPOSITORY, "GITHUB_REF": "refs/heads/main"}
            with patch.dict(os.environ, environment), patch("sys.argv", ["publish.py", "--artifact", directory]):
                with patch.object(release, "gh") as gh:
                    with self.assertRaises(SystemExit):
                        release.main()
                    gh.assert_not_called()

    def test_mismatched_commit_never_creates_a_tag(self):
        Path("/var/tmp").mkdir(parents=True, exist_ok=True)
        with tempfile.TemporaryDirectory(prefix="peru-release-guard-", dir="/var/tmp") as directory:
            root = Path(directory)
            (root / "manifest").mkdir()
            data = json.dumps({"version": "0.1.0-alpha.1", "commit": "a" * 40}).encode()
            (root / "manifest/release.json").write_bytes(data)
            digest = hashlib.sha256(data).hexdigest()
            (root / "manifest/SHA256SUMS").write_text(f"{digest}  manifest/release.json\n")
            environment = {"GITHUB_REPOSITORY": release.REPOSITORY, "GITHUB_REF": "refs/heads/main", "GITHUB_SHA": "b" * 40}
            with patch.dict(os.environ, environment), patch("sys.argv", ["publish.py", "--artifact", directory]):
                with patch.object(release, "gh") as gh:
                    with self.assertRaises(SystemExit):
                        release.main()
                    gh.assert_not_called()


if __name__ == "__main__":
    unittest.main()
