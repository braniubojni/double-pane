# Feature backlog

Not commitments — ideas to triage. Check here before proposing new features (see root `AGENTS.md`).

## Bugs

## Core file-manager gaps (Double Commander parity)

- [ ] Quick view panel (F3-style preview: images, text, PDF) without opening editor
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
- [ ] Parallel transfer progress + pause/resume/cancel for large SFTP copies
- [ ] Paste should work with remote files as well

## UI/UX polish

## Editor/terminal

## Other

- [x] Duplicate finder (exact SHA-256, similar images, optional OCR; setup → progress → review → merge; background + status bar). Local, SSH/SFTP, SMB, MEGA.
- [ ] Duplicate finder on native Google Drive / iCloud APIs (today those connections are local OS mounts only)
- [x] Local trash/recycle bin (OS trash, in-app `trash://` restore, empty, Shift+Delete)
- [x] Cursor usage in the AI usage toolbar popover (personal Pro quota via local Cursor session; not a status-bar chip)
- [ ] SSH/SMB soft-delete/restore (MEGA already uses MEGA trash)
- [ ] Disk usage treemap view (like WinDirStat) per folder
- [ ] Plugin/extension points — low priority, only if long-term extensibility actually needed
- [ ] New settings to show only in the tray or both tray and system menu
