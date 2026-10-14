# Spec: Duplicate finder on iCloud and Google Drive files V1

Status: draft
Depends on: duplicate finder V1 (`specs/duplicate-finder/v1.md`). Visual/OCR options behave as they do on any other local root.
Out of scope: a new `gdrive://` or `icloud://` scheme, Google OAuth, CloudKit, browsing a cloud account that is not signed in on this Mac, Windows/Linux cloud providers

Cursor: implement this spec only. Read `internal/AGENTS.md` and `frontend/AGENTS.md` first.

## Current behavior

iCloud and Google Drive are local folders, not connections.

- `FileService.ICloudDrivePath` → `filesystem.ICloudDrivePath` (`~/Library/Mobile Documents/com~apple~CloudDocs`, else `~/Library/CloudStorage/iCloud Drive*`).
- `FileService.GoogleDrivePaths` → `filesystem.GoogleDrivePaths` (`~/Library/CloudStorage/GoogleDrive-*` and `/Volumes/GoogleDrive`).
- The connections menu opens those paths in a pane (`ConnectionsMenu.tsx`, `menu-conn-icloud`, `menu-conn-gdrive-*`).

`StartDuplicateScan(jobID, root, …)` lists with `ListDir`. For a non-remote root that is `filesystem.ListDir`. A scan started on that mount path already hashes files that are on disk. There is no Drive API client and no CloudKit client. The app also has no OAuth. Files the desktop client has not downloaded are still directory entries; reading them either blocks on a provider fetch or fails. The scan does not treat that case on purpose (`validateDupRoot` only checks `ListDir`).

"Native API" here means the OS file provider that already backs those folders (materialize, then hash). It does not mean a second cloud account system.

## Desired

When the scan root is under an iCloud Drive path or a Google Drive path returned by those two functions:

- A placeholder file is not hashed as an empty or short file.
- Optional checkbox, default **off**: "Download cloud files while scanning". Off: skip placeholders, count them as skipped (the scan result already has a skipped count — use it), reason available in the log line the duplicate job already prints (`dupLog`). On: read the file through so the OS downloads it, then hash as usual. The read is the hash read (`HashFile` / the remote hash path is not used; these paths are local).

Detect a placeholder on darwin only:

- `UF_DATALESS` on the file flags when the `stat` structure exposes it, or
- the File Provider resource that `getattrlist` reports as dataless, if that is the flag this OS version actually sets.

If neither flag is readable for a file, treat it as a normal local file (today's behavior). Do not skip a file just because it lives under `CloudStorage`.

Non-darwin: the checkbox is hidden and the scan is unchanged, because those path helpers are macOS folders.

The duplicate setup dialog (`StartDuplicateScan`'s UI) gets the checkbox only when the chosen root is under one of those paths. Starting a scan from an unrelated local folder does not show it. Pass it as a new bool argument on `StartDuplicateScan`, default false so existing callers and tests match current results.

Progress: a download wait is part of the current file, not a new phase. Cancel still aborts the job.

## Tests

- Root outside the cloud paths: flag false, dataless detector not consulted.
- Fixture file marked dataless by a stubbed detector (do not require a real iCloud account): checkbox off → skipped, not hashed; checkbox on → hash function is called.
- `ICloudDrivePath` / `GoogleDrivePaths` behavior unchanged.

## Must not

- Add Google API or CloudKit dependencies.
- Upload, delete, or create files in the cloud as part of the scan. Delete of duplicates stays the existing local delete (the mount then syncs, which is the OS client's job).
- Register a new connection protocol.
