import tempfile
from pathlib import Path
import unittest
from unittest.mock import patch

import package_linux


class PackageSafetyTests(unittest.TestCase):
    def test_unsupported_host_is_rejected_before_build(self):
        with patch.object(package_linux.platform, "freedesktop_os_release", return_value={
            "ID": "ubuntu", "VERSION_ID": "24.04"
        }), patch.object(package_linux.subprocess, "run") as build:
            with self.assertRaisesRegex(RuntimeError, "Fedora 44"):
                package_linux.main()
            build.assert_not_called()

    def test_notices_preserve_nested_paths_without_copying_source(self):
        with tempfile.TemporaryDirectory() as temporary:
            source = Path(temporary) / "module"
            destination = Path(temporary) / "notices"
            (source / "vendor").mkdir(parents=True)
            (source / "LICENSE").write_text("module terms")
            (source / "vendor/COPYING.txt").write_text("vendor terms")
            (source / "private_fixture.json").write_text("not a notice")
            package_linux.copy_notices(source, destination)
            self.assertEqual((destination / "LICENSE").read_text(), "module terms")
            self.assertEqual((destination / "vendor/COPYING.txt").read_text(), "vendor terms")
            self.assertFalse((destination / "private_fixture.json").exists())

    def test_absent_or_symlink_only_notices_fail_closed(self):
        with tempfile.TemporaryDirectory() as temporary:
            source = Path(temporary) / "module"
            source.mkdir()
            for symlink in (False, True):
                if symlink:
                    outside = Path(temporary) / "external.txt"
                    outside.write_text("outside module")
                    (source / "LICENSE").symlink_to(outside)
                with self.assertRaisesRegex(RuntimeError, "No license/notice"):
                    package_linux.copy_notices(source, Path(temporary) / "notices")


if __name__ == "__main__":
    unittest.main()
