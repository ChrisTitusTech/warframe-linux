# Warframe Linux - Fedora test archive

This is an unreleased offline mock catalog, built for Fedora 44 x86_64.
It is not a universal Linux binary. A normal graphical desktop session is
required; no Go, Node, npm, Overwolf, game or account is needed to run it.

## Runtime dependencies

On a clean Fedora 44 x86_64 desktop:

```sh
sudo dnf install glibc gtk3 webkit2gtk4.1
```

These packages pull in GLib, Cairo, Pango, ATK, libsoup and JavaScriptCore.
Actual build-machine package and bundled module versions are recorded in
`build-info.json`. This declares a target, not universal desktop/GPU compatibility.

## Install and run

Copy the archive and matching `.sha256` file into an empty directory on the
target machine. Substitute the supplied archive's exact name below:

```sh
sha256sum -c warframe-linux-fedora44-x86_64-REVISION.tar.gz.sha256
mkdir -p "$HOME/Applications"
tar -xzf warframe-linux-fedora44-x86_64-REVISION.tar.gz -C "$HOME/Applications"
cd "$HOME/Applications/warframe-linux-fedora44-x86_64-REVISION"
./warframe-linux
```

The install is this versioned directory. It does not change PATH, register a
service, request elevated app execution, or overwrite another version. A
`-dirty` suffix identifies a development preview from uncommitted work.
Never run the desktop application with sudo.

Confirm Example Warframe, Example Rifle and Example Relic appear, then close the
window normally. Record the desktop/session type, GPU, display scale and any
errors. Startup and process-tree memory measurements remain acceptance work;
this archive does not yet include catalog search.

## Uninstall and rollback

After closing the app, remove only its extracted versioned directory:

```sh
rm -r -- "$HOME/Applications/warframe-linux-fedora44-x86_64-REVISION"
```

Run a previously extracted version to roll back. The mock application writes no
inventory or database. These steps do not touch `oldhelp` or existing companion data.

## Notices

Original project material uses the included MIT + Commons Clause v1.0 license.
Bundled dependencies retain their own notices under `third-party-notices`.
`NOTICE.md` describes provenance; its repository links are available at
https://github.com/ChrisTitusTech/warframe-linux . No WFHelper or oldhelp code or
game artwork is included. This test archive is not a published release.
