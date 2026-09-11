# F01c validation - 2026-09-10

Prerequisite F01b merged in PR #16 as `b0e542c47a76`.
Branch: `codex/f01c-linux-package`. Local acceptance is complete under the
maintainer-approved scope below; merge evidence is recorded in the PR.

## Artifact and build

`python3 scripts/package_linux.py` built the native mock app with Wails v2.15.0
and `-trimpath` on the F01a Fedora 44 x86_64 environment. Python 3.14.6 is the
packaging helper runtime. The archive is a development preview from uncommitted
packaging work, labeled `b0e542c47a76-dirty`, not a published release.

Artifact: `build/packages/warframe-linux-fedora44-x86_64-b0e542c47a76-dirty.tar.gz`.
Initial desktop-tested archive SHA-256:
`a9c697f0ebb278496021293497c8729beba7667baea490b6b395a3f2e9515339`.

A subsequent requested rebuild replaced the local archive at the same path.
Its SHA-256 is `09eb7e7835118d62a6ccebe0b638152c5c86bd24e7477c43029e5a291987deca`.
The executable is byte-identical to the desktop-tested executable, SHA-256
`c4d94aa2609490323046be0f9d92b77750ec6578d464d065afadda3a17bfc1ba`.
Archive metadata timestamps are not normalized; archive bytes are not claimed
reproducible. The rebuild's structure, executable hash and sidecar passed.

The archive contains the executable, runtime/install/uninstall instructions,
project license and notices, bundled dependency notices, and `build-info.json`.
The manifest lists 14 linked Go modules plus React, React DOM and Scheduler.
Only the standard Fedora runtime libraries are external; Go and Node are not
runtime dependencies. Fedora's separate Go LICENSE/PATENTS location is handled.

## Automated evidence

- Native rebuild and archive generation: passed.
- Archive structure: one relative root, no parent traversal or symlinks;
  executable permission and manifest binary checksum verified.
- SHA-256 sidecar and frontend transitive-notice coverage: verified.
- `python3 -m unittest discover -s scripts -p 'test_*.py'`: three tests passed,
  covering unsupported hosts, nested notice copying, missing/symlink-only notices.
- Python compilation, actionlint and diff whitespace checks: passed.
- Requested repeat native build, frontend typecheck/lint/tests, Go race tests
  and vet, package-helper tests and actionlint: passed after desktop evidence
  was recorded. A new CI step runs the package-helper tests.

A fresh rootless Fedora 44 container used image digest
`sha256:669116f4e61f56ebe14fc25ea27f23d483a8198d77854fafa52259c5b791e130`.
Installed only declared runtime packages and archive utilities, copied the
archive, extracted it, checked executable mode and ran `ldd`. Every library
resolved with glibc 2.43-8, GTK 3.24.52-2 and WebKitGTK 2.52.5-1 (Fedora 44
packages). Go, Node and npm were absent. Removing the extracted versioned
directory succeeded. The task container was removed after validation.

The pre-existing cached image failed to start with a missing `/run/.containerenv`;
refreshing the Fedora image resolved that environment failure. The previous
image was not deleted. No container desktop or game was launched, and no display
socket, home directory, credential or account data was mounted into it.

## Desktop evidence

At the user's request, the exact archive was extracted and launched on this
Fedora 44 x86_64 desktop with Warframe already running. This is the development
desktop, not a clean graphical installation; the fresh-container dependency
check above is separate evidence. Session: X11, dwm, Xft.dpi 96; NVIDIA GeForce
RTX 4070 SUPER, driver 610.57.04.

The packaged app displayed all three fictional catalog examples and its demo
notice. [App-only screenshot](evidence/f01c-dwm.png). Initial off-workspace
captures were black; focusing the app exposed the rendered content. No game
capture, game-memory access or account interaction was performed.

Three subsequent warm launches produced these measurements:

| Trial | Window mapped (seconds) | Idle combined RSS (MiB) | Normal close |
| --- | --- | --- | --- |
| 1 | 0.507 | 435.4 | Exit 0, no remaining child processes |
| 2 | 0.514 | 397.8-399.5 | Exit 0, no remaining child processes |
| 3 | 0.487 | 399.6 | Exit 0, no remaining child processes |

Window mapping is not a first-paint measurement. Two usable timed captures
showed rendered app content by 1.576 and 1.546 seconds, including a deliberate
one-second wait and capture overhead. Their smaller tiled viewport showed two
rows and a scrollbar; the full screenshot above independently verifies all
three examples. The first trial's black capture is excluded from visual timing.
These few warm samples do not establish cold startup or a percentile budget.

RSS sums `/proc` resident memory for the app and its WebKit network/web children,
with three idle samples per trial after settling. Shared pages may be counted
in more than one process. The measured 397.8-435.4 MiB exceeds the proposed
250 MiB combined-RSS target. SPEC.md calls this a proposed P0 measurement
target, so it is retained as a performance risk rather than misreported as a
passing budget or treated as an additional F01c package gate. All controlled
closes used the normal WM_DELETE_WINDOW protocol.
No test companion remains running.

## Installed-preview user verification

The rebuilt archive was checksum-verified and installed without elevated app
execution under `/home/titus/Applications/warframe-linux-fedora44-x86_64-b0e542c47a76-dirty`.
Its executable matched the manifest hash and all `ldd` dependencies resolved.
The installed executable was launched and its window activated. In response to
"verify the three mock items and close normally", the user confirmed "done";
the subsequent process check found no running `warframe-linux` process.
This confirms the installed preview on the user-selected existing desktop,
not a fresh graphical OS installation. The installed copy remains available.

## Approved acceptance scope

The maintainer explicitly approved using the confirmed installed-app check on
this desktop together with the clean-container dependency/install/uninstall
check as sufficient F01c evidence, and measuring search in C02b when it exists.
This closes the F01c manual gate without claiming a clean graphical OS test.
B01a retains its clean-machine install and launch requirement for distribution.
The proposed memory target remains unchanged; this phase records its measured
shortfall rather than claiming it is met.

## Review and limitations

- Repeated independent `codex review --uncommitted` after the rebuild and
  desktop evidence update: no actionable defects; verified the three tests
  and rebuilt artifact structure, executable permissions and checksums.
  CodeRabbit CLI completed
  its separate review with zero findings across the original eight changed
  files, before the desktop evidence was appended.
- Current-desktop display and normal-close checks passed. Clean graphical
  installation remains unproven and is explicitly outside the accepted F01c
  evidence adjustment above.
- The proposed combined idle-RSS target is exceeded. Performance tuning or a
  budget revision remains future work; no target is revised by this report.
  Cold startup remains unmeasured but is not an explicit F01c acceptance gate.
- Search timing is unavailable because F01a has no search. The maintainer
  approved measuring it in C02b, which requires 10,000-item cached-search timing.
- The PR records final remote checks, review state and the merge commit.

F01c local acceptance is complete. Instructions are in
[the package README](../packaging/README.md).
