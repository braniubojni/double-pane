# Feature backlog

Not commitments — ideas to triage. Check here before proposing new features (see root `AGENTS.md`).

## Bugs

- [x] After each update we are loosing user's bookmarket paths. We can save this info inside of user folder in so that after each update data persisted (spec: `specs/bookmarks-persist/v1.md`)
- [ ] Seems like we lost having pointer within our app, I do not know is it related to latest macos 27 update. (spec: `specs/macos-cursor/v1.md`)
- [ ] In AI usage dialog let's not have scrolls at all (spec: `specs/ai-usage-popover/v1.md`)

## Core file-manager gaps (Double Commander parity)

- [ ] Quick view panel (F3-style preview: images, text, PDF) without opening editor (spec: `specs/quick-view/v1.md`)
- [ ] Folder compare/sync (diff two dirs, sync one-way or two-way)
- [ ] Batch rename (pattern-based, regex, counter)
- [ ] Checksum/hash tool (MD5/SHA) to verify after copy
- [ ] Symlink/hardlink create + follow toggle

## Search

- [ ] Filter results by size/date/type
- [ ] Save search as smart folder
- [ ] Search inside archives
- [ ] Option to search by filename(currently implemented only by folder name and file content)

## Remote/SFTP

- [ ] FTP support
- [ ] SMB Kerberos / ticket auth (NTLM only in V1)
- [ ] Copy/move between SSH and SMB in one step
- [ ] Persist remote passwords (SSH/SMB) in OS keychain
- [ ] SMB discovery / Bonjour browse for nearby shares
- [ ] Create-empty-file / archive on remote (SSH and SMB)
- [x] Search on remote (SSH and SMB; content mode; MEGA still blocked)
- [ ] Saved connection profiles (host/user/key) in SQLite alongside bookmarks
- [ ] SSH key auth UI (not just password) — pick key file, agent forwarding
- [ ] Remote tab reconnect on drop, connection status indicator per pane
- [ ] Parallel transfer progress + pause/resume/cancel for large SFTP copies (spec: `specs/sftp-transfer-control/v1.md`)
- [x] Paste should work with remote files as well (spec: `specs/remote-paste/v1.md`)

## UI/UX polish

## Editor/terminal

## Other

- [x] Duplicate finder (exact SHA-256, similar images, optional OCR; setup → progress → review → merge; background + status bar). Local, SSH/SFTP, SMB, MEGA.
- [ ] Duplicate finder on native Google Drive / iCloud APIs (today those connections are local OS mounts only)
- [ ] Add new option into duplicate finder, handle apple's/ios live videos, like if user wishes to delete them as well. I belive there should be connection between photo(meta info) and a short(1-3 seconds) video. Because when you watch this photos in ios there is no seperate photo and video, they are attached to each other. RESEARCH first. (spec: `specs/duplicate-finder/v3-live-photo.md`)
  - [ ] Add also option to remove this videos itself (same spec)
- [ ] Duplicate finder new option for image diff. Maybe if we see some similiarites between date, name and some other options we can also check via image diff this photos in order to select them to remove. (spec: `specs/duplicate-finder/v3-image-diff.md`)
- [x] Local trash/recycle bin (OS trash, in-app `trash://` restore, empty, Shift+Delete)
- [x] Cursor usage in the AI usage toolbar popover (personal Pro quota via local Cursor session; not a status-bar chip)
- [ ] SSH/SMB soft-delete/restore (MEGA already uses MEGA trash)
- [ ] Disk usage treemap view (like WinDirStat) per folder
- [ ] Plugin/extension points — low priority, only if long-term extensibility actually needed
- [ ] New settings to show only in the tray or both tray and system menu
- [ ] Password vault for local folders/files (spec: `specs/folder-vault/v1.md`) — per-file AES-256-GCM, no mount, auto-lock on leave/quit/idle. Not zip, not DMG.
- [ ] Research and add another option into find duplicates. Ability to search apple's live video(about two second video that goes with photo) and remove it if needed. (same work as `specs/duplicate-finder/v3-live-photo.md` — do not implement twice)
