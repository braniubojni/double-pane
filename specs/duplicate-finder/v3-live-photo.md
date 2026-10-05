# Spec: Duplicate finder V3 — Apple Live Photos

Status: draft
Depends on: V1 exact groups and V2 visual/OCR (`specs/duplicate-finder/v1.md`, `v2.md`). When both new options are off, scan results match what ships today.
Out of scope: remote Live Photo pairing (`ssh://`, `smb://`, `mega://`), HEIC pixel decode, playing the video, iCloud Photos API, writing metadata, auto-delete

Cursor: implement this spec only. Read `internal/AGENTS.md` and `frontend/AGENTS.md` first. The pairing rules below are the research. Do not shell out to `mdls`, `exiftool`, or `ffmpeg`.

## What a Live Photo is

Photos.app shows one asset. On disk it is two files in the same folder:

- a still: `.jpg`, `.jpeg`, `.heic`, or `.heif`
- a movie: `.mov` (sometimes `.mp4`), usually 1–3 seconds

They are linked by a UUID, not by being the same bytes.

- Movie: QuickTime user-data key `com.apple.quicktime.content.identifier` inside `moov` (`meta` / `keys` / `ilst`). Duration is the `mvhd` timescale and duration in that same `moov`.
- JPEG still: the same UUID in the XMP packet as `ContentIdentifier` (often near the start of the file).
- HEIC still: the same UUID in the HEIF meta box. HEIF is ISO-BMFF, so the movie walker can read it. Do not decode HEIC pixels.

`moov` is often at the **end** of the movie. A header-only read misses it. Walk top-level boxes (32-bit size + 4-byte type, and the 64-bit size form when size is 1). Seek to `moov`, and only if its size is ≤ 8MB read that range. A larger `moov` is skipped (count as skipped, not fatal). Use `ReadAt` / `Seek`. Do not read the whole movie.

Basename is a fallback, not the primary key. Exports sometimes share a stem (`IMG_1234.HEIC` + `IMG_1234.MOV`) and sometimes do not.

## Pairing

Package `internal/livephoto`. Pure Go. No CGo.

A still and a movie pair when all of these hold:

1. Same directory. Do not pair across folders.
2. Movie duration is known and in `(0, 5]` seconds, **or** duration is unknown and rule 3 matched by UUID. A movie whose `mvhd` says longer than 5 seconds never pairs, even if the UUID matches. That rejects a normal video that happens to share a name or a stale id.
3. Identity, either:
   - both have a content id and the ids are equal, or
   - the movie has an id, the still has none, and the stems are equal (case-insensitive, extension stripped), or
   - neither has an id, the stems are equal, and the duration is known and ≤ 5 seconds.

Conflicts:

- One movie matches two stills: pair it with the UUID match. If both are stem-only, pair with neither and count a skip. Do not guess.
- One still matches two movies: same rule.
- A paired movie is not a duplicate of its still. Do not put the two files in an exact, visual, OCR, or image-diff group together.

Local files only. If the scan root is remote, ignore both options (treat as off) and do not download movies to sniff them. The setup checkbox is disabled with the hint "Live Photos are local files only".

## Options

Both default **off**. Off means the walker does not read `moov` and does not stat movies for this feature.

1. `liveWithStills` — "Delete the Live Photo video when its photo is deleted".
2. `stripLiveVideos` — "Remove Live Photo videos and keep the photos".

They are independent. The existing Merge button still runs one `FileService.Delete` of the checked paths. Nothing deletes until the user presses Merge.

### Review

Do not add the movie as another member of the photo's hash group.

- When `liveWithStills` is on, each still row that has a pair shows a second line: the movie's base name. If that still is checked for deletion, the movie path is included in the delete set. The still the user keeps (`keepByHash`) does not take its movie with it. Unchecking the still unchecks its movie.
- When `stripLiveVideos` is on, add a section "Live Photo videos" listing each paired movie whose still is **not** already in the delete set. Checkbox defaults to checked. Uncheck to keep that video. This section is not a `DuplicateGroup` and has no similarity percent.
- A movie that is already in the delete set because its still is checked must not also appear in the strip section.
- `stripLiveVideos` with zero pairs: section hidden, not an error.

Default keep for real duplicate groups is unchanged (V1: first path sorted).

## API

Extend the existing scan. Do not add a second job.

`EstimateDuplicateScan` and `StartDuplicateScan` gain two bools at the end: `liveWithStills`, `stripLiveVideos`. Update every Go call site and the frontend store (`DupSetup`, default false, and the MEGA default path must also pass false). Regenerate bindings. Do not commit `frontend/bindings/**`.

Estimate, only when either option is on and the root is local:

- `livePairCount` — pairs found. Finding pairs needs the same box walk as the scan. The estimate may do that walk; it must not hash file bodies. Cap it with the same cancel context as the rest of the estimate.
- No new MEGA byte count. Remote is off.

Progress phase string stays `walk | exact | visual | ocr`. Live pairing runs inside `walk` (it is a stat + range read, not a pixel pass). You may set `phase` to `live` during that pass. One `jobId`. Cancel stops the walk.

Delete plan is a pure function: `(groups, keep, checked, pairs, liveWithStills, stripChecked) → []deletePath`. Test that. The Merge handler calls it and passes the result to the existing delete. Do not delete inside the scanner.

## UI

`DupSimilarSetup` (or a sibling component if that file would pass ~150 lines): two checkboxes, `data-testid="chk-dup-live-with"` and `chk-dup-strip-live"`. Disabled on a remote root.

Review: movie line on the still row (`data-testid` including the still path is enough). Strip section only when that option is on and there is at least one row.

## Tests

Build fixtures in the test with `bytes.Buffer`. Do not commit a binary MOV from a phone.

- Minimal `moov` with `mvhd` duration 2s and `com.apple.quicktime.content.identifier` = a fixed UUID. Still JPEG with an XMP `ContentIdentifier` of that UUID. `Pair` returns that one pair.
- Same stem, no UUIDs, duration 2s → one pair.
- Same stem, duration 30s → no pair.
- UUID match but duration 30s → no pair.
- Two stills, one movie, only one UUID match → that one pair.
- Remote root with the options forced on → zero pairs, no error.
- Delete plan: still checked + `liveWithStills` → movie path included; kept still's movie absent; `stripLiveVideos` lists only movies of kept stills; a movie already deleted with its still is not listed twice.
- Both options off: existing duplicate tests stay valid. A fixture MOV next to two identical JPEGs does not create an extra group and is not deleted.

## Must not

- `mdls`, `exiftool`, `ffmpeg`, CGo, a new module for MP4.
- Download a remote tree to pair Live Photos.
- Auto-delete, or delete on scan finish.
- Treat the movie as a visual duplicate of the still.
- Change results when both checkboxes are off.

## Done when

The tests above pass, both checkboxes default off, Merge deletes the extra movies only when the user left them checked, and a remote root disables the checkboxes.
