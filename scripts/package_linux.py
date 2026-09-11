#!/usr/bin/env python3
"""Build the Fedora 44 x86_64 test archive without installing it on the host."""

import hashlib
import json
import os
from pathlib import Path
import platform
import shutil
import subprocess
import tarfile
import tempfile


ROOT = Path(__file__).resolve().parents[1]


def output(*args):
    return subprocess.check_output(args, cwd=ROOT, text=True).strip()


def copy_notices(source, destination):
    names = ("LICENSE", "LICENCE", "COPYING", "NOTICE", "PATENTS")
    found = False
    for path in sorted(source.rglob("*")):
        if path.is_file() and not path.is_symlink() and path.name.upper().startswith(names):
            target = destination / path.relative_to(source)
            target.parent.mkdir(parents=True, exist_ok=True)
            shutil.copyfile(path, target)
            found = True
    if not found:
        raise RuntimeError(f"No license/notice files found in {source}")


def main():
    distro = platform.freedesktop_os_release()
    if (distro.get("ID"), distro.get("VERSION_ID"), platform.machine()) != (
        "fedora", "44", "x86_64"
    ):
        raise RuntimeError("This first test archive must be built on Fedora 44 x86_64")
    wails = shutil.which("wails") or str(Path(output("go", "env", "GOPATH")) / "bin/wails")
    subprocess.run([wails, "build", "-tags", "webkit2_41", "-trimpath"], cwd=ROOT, check=True)
    binary = ROOT / "build/bin/warframe-linux"
    revision = output("git", "rev-parse", "HEAD")
    dirty = bool(output("git", "status", "--porcelain"))
    label = revision[:12] + ("-dirty" if dirty else "")
    name = f"warframe-linux-fedora44-x86_64-{label}"
    destination = ROOT / "build/packages"
    destination.mkdir(parents=True, exist_ok=True)
    with tempfile.TemporaryDirectory(prefix="warframe-package-") as temporary:
        stage = Path(temporary) / name
        stage.mkdir()
        shutil.copy2(binary, stage / "warframe-linux")
        for filename in ("LICENSE", "NOTICE.md"):
            shutil.copyfile(ROOT / filename, stage / filename)
        shutil.copyfile(ROOT / "packaging/README.md", stage / "README.md")
        notices = stage / "third-party-notices"
        notices.mkdir()
        go_root = Path(output("go", "env", "GOROOT"))
        for filename in ("LICENSE", "PATENTS"):
            source = go_root / filename
            if not source.is_file():
                source = Path("/usr/share/licenses/golang") / filename
            shutil.copyfile(source, notices / ("Go-" + filename))
        modules = []
        for line in output("go", "version", "-m", str(binary)).splitlines():
            fields = line.split()
            if fields and fields[0] == "dep":
                module, version = fields[1:3]
                metadata = json.loads(output("go", "list", "-m", "-json", module))
                if metadata.get("Replace") or metadata["Version"] != version:
                    raise RuntimeError(f"Unrecorded module replacement/version: {module}")
                key = module.replace("/", "__") + "@" + version
                copy_notices(Path(metadata["Dir"]), notices / key)
                modules.append({"module": module, "version": version})
        frontend = []
        for item in output("npm", "ls", "--prefix", "frontend", "--omit=dev", "--all", "--parseable").splitlines():
            directory = Path(item)
            if directory == ROOT / "frontend":
                continue
            if not directory.is_relative_to(ROOT / "frontend/node_modules"):
                raise RuntimeError(f"Unexpected frontend dependency path: {directory}")
            metadata = json.loads((directory / "package.json").read_text())
            key = metadata["name"].replace("/", "__") + "@" + metadata["version"]
            copy_notices(directory, notices / key)
            frontend.append({"package": metadata["name"], "version": metadata["version"]})
        manifest = {
            "target": "Fedora 44 x86_64", "revision": revision, "dirty": dirty,
            "go": output("go", "version"), "node": output("node", "--version"),
            "npm": output("npm", "--version"),
            "runtime_packages": output("rpm", "-q", "glibc.x86_64", "gtk3.x86_64", "webkit2gtk4.1.x86_64").splitlines(),
            "binary_sha256": hashlib.sha256(binary.read_bytes()).hexdigest(),
            "go_modules": modules, "frontend_packages": frontend,
        }
        (stage / "build-info.json").write_text(json.dumps(manifest, indent=2) + "\n")
        archive = destination / (name + ".tar.gz")
        with tempfile.NamedTemporaryFile(dir=destination, suffix=".tmp", delete=False) as file:
            pending = Path(file.name)
        try:
            with tarfile.open(pending, "w:gz") as bundle:
                bundle.add(stage, arcname=name)
            os.replace(pending, archive)
        finally:
            pending.unlink(missing_ok=True)
        checksum = hashlib.sha256(archive.read_bytes()).hexdigest()
        archive.with_suffix(archive.suffix + ".sha256").write_text(f"{checksum}  {archive.name}\n")
        print(archive)
        print(f"SHA256 {checksum}")


if __name__ == "__main__":
    main()
