# Spec: Duplicate finder V3 — name/date gated image diff

Status: draft
Depends on: V1 exact SHA-256 and V2 dHash (`specs/duplicate-finder/v1.md`, `v2.md`). Can ship beside `specs/duplicate-finder/v3-live-photo.md` or before it. Do not mix the two features in one change if both are in progress: this spec does not read movies, that spec does not pixel-diff.
Out of scope: HEIC decode, video frames, pHash / OpenCV / CGo, a new image library, replacing dHash, OCR changes, auto-delete

Cursor: implement this spec only. Read `internal/AGENTS.md` and `frontend/AGENTS.md` first.

## Goal

Optional matcher for photos that are the same shot saved twice (export, resize, "copy" in the name) but are not bit-identical and may sit too far apart in dHash after a crop or a recompress.

dHash stays. This matcher does not run on every image pair. It decodes only pairs that already look related by name and time. That is the point: pixel work is the confirmation, not the search.

When the option is off, groups and the MEGA byte estimate match today.

## Candidate gate

Both files must pass every line before any decode. Implement the gate as a pure function and test it without images.

- Both names pass `imghash.IsVisualName` (jpg, jpeg, png, gif, webp). HEIC is not a candidate. Count HEIC as skipped when the option is on, same as V2 visual.
- Neither path is already in an exact group or a visual (dHash) group. Those files must not appear again.
- Same parent directory.
- Names: equal stem after stripping one trailing copy suffix. Strip, in order, a final ` copy`, `-copy`, `_copy`, ` (N)`, `-N`, `_N` where N is an integer. Compare case-insensitively. `IMG_1234` and `IMG_1234 (1)` match. `a` and `b` do not. Do not use Levenshtein. Do not match across different stems (`DSC_0001` vs `DSC_0002` stays unmatched).
- Time: absolute mod-time difference ≤ 2 seconds. Do not read EXIF. Mod-time is the date signal. A matching name with times an hour apart is not a candidate.
- Size is not a gate. A recompress changes size.
- Each file is compared with at most 8 candidates that passed the gate. If more than 8 share a stem and a time window, take the 8 closest mod-times and skip the rest (count skipped).

## Pixel diff

Only gated pairs. Reuse the V2 decoders (`image/jpeg`, `image/png`, `image/gif`, `golang.org/x/image/webp`). Cap 50MB per file (`imghash.MaxBytes`). Over the cap: skip, exact hash still applies as today.

- Downscale to 32×32 and use luma. Share the resize already in `internal/imghash` if it is the same size; if dHash uses a different geometry, add a `MeanAbs` helper next to it rather than a second decoder.
- Score = `100 * (1 - mae/255)` truncated to int, mae = mean absolute luma difference over the 1024 samples.
- Keep the pair when score ≥ `diffPct`.
- Union-find: A~B and B~C become one group, same as `imghash.Cluster`.
- Group `kind` is `diff`. `similarity` is the minimum pair score in the group. `hash` is a stable id prefixed `diff:` so it cannot collide with `sha256`, visual, or `ocr:`.
- A file that landed in a `diff` group is not sent to OCR (same rule as exact: OCR skips files already grouped).

Slider `diffPct`: 70–100, default **92**. Separate from the dHash slider. 92 is the default because this score is a raw pixel error, not a Hamming similarity; do not reuse `similarityPct`.

## Cost

Phase `diff`, after `visual`, before `ocr`, inside the same job. Progress reuses `doneImages` / `totalImages` for this phase. Cancel aborts decode.

MEGA (and any path that must download before decode): download only gated candidates, not every image. Add those bytes to `megaDownloadBytes` only when this option is on. When it is off, `megaDownloadBytes` stays the V2 value. Add `etaDiffSeconds` to `ScanEstimate`, 0 when the option is off.

Local files: read in place. No temp copy.

## API

`EstimateDuplicateScan` and `StartDuplicateScan` gain `imageDiff bool` and `diffPct int` after the existing `ocrOn` argument. If `v3-live-photo.md` has already added its bools, add these after those bools. Every call site passes `imageDiff=false` and `diffPct=92` unless the user turned it on. Existing tests that assert group counts must still pass with the option off.

Regenerate bindings. Do not commit `frontend/bindings/**`.

`DupSetup` gets `imageDiff` (default **false**, including on local) and `diffPct` (default 92). Default false everywhere, including local: this matcher is opt-in. dHash's local-on / MEGA-off default is unchanged.

## Review

New section "Same photo (name, date, pixel diff)". Show the group's similarity. Do not merge it into "Similar photos" (that label is dHash).

Default keep = first path sorted, same as V1. Merge is still one `Delete` of checked rows.

A path listed under Identical or Similar photos is not listed here.

## UI

One checkbox `data-testid="chk-dup-diff"` and a slider `data-testid="slider-dup-diff"` enabled only when the checkbox is on. Caption states the gate in one line: same folder, similar name, time within 2 seconds, then pixel compare. Put the controls in a small component if `DupSimilarSetup` would exceed ~150 lines.

Estimate text: when the checkbox is on, show `etaDiffSeconds` and, for MEGA, that only name/time candidates are downloaded.

## Tests

Gate (no decode):

- `IMG_1234.jpg` and `IMG_1234 (1).jpg`, mod-time 1s apart → candidate
- same names, mod-time 10s apart → not a candidate
- `DSC_0001.jpg` and `DSC_0002.jpg`, same mtime → not a candidate
- different directories → not a candidate
- `.heic` → not a candidate
- file already in an exact group → not a candidate

Pixels (small PNGs written in the test):

- two PNGs, same stem pattern, mtime within 2s, a few pixels changed → one `diff` group at the default 92 if the score is ≥ 92; if your fixture's score is lower, set the fixture so the score is ≥ 92 and assert the number
- two PNGs, same name pattern and mtime, obviously different images (black vs white) → no group
- same pixels but mtimes an hour apart → no group, and the decode func is not called (inject it)
- option off → no `diff` group, decode func not called
- a pair that is bit-identical stays `exact` only
- more than 8 stem matches → at most 8 compares for that file

`go test` for `internal/imghash` and the scan test next to the existing dup tests (`file_service_dup_ocr_test.go` is the pattern: temp dir, stub what you must, assert groups).

## Must not

- Run this on all image pairs in the tree.
- Add OpenCV, pHash, libexif, or CGo.
- Change dHash groups.
- Change any group when the checkbox is off.
- Auto-delete.
- Hand-edit bindings.

## Done when

The tests above pass, the checkbox defaults off, MEGA estimate bytes do not grow until the checkbox is on, and a `diff` group never contains a file that is already in an exact or visual group.
