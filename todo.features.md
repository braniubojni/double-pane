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

## Remote/SFTP

- [ ] FTP support
- [ ] SMB Kerberos / ticket auth (NTLM only in V1)
- [ ] Copy/move between SSH and SMB in one step
- [ ] Persist remote passwords (SSH/SMB) in OS keychain
- [ ] SMB discovery / Bonjour browse for nearby shares
- [ ] Create-empty-file / archive / search on remote (SSH and SMB)
- [ ] Saved connection profiles (host/user/key) in SQLite alongside bookmarks
- [ ] SSH key auth UI (not just password) — pick key file, agent forwarding
- [ ] Remote tab reconnect on drop, connection status indicator per pane
- [ ] Parallel transfer progress + pause/resume/cancel for large SFTP copies
- [ ] Paste should work with remote files as well

## UI/UX polish

## Editor/terminal

## Other

- [ ] Smart tool that will analyze the files and folders and will highlight the files that are similar to each other(by content or by name, by metadata and etc.). It should be able to work with the files in the all remote connections as well. Let's also have OCR I want functionality that will going to find duplicates and if user need it it will show in dialog percentage of two photos together and user will be able to select all or select one by one, then remove it. Tool should work everywhere. sftp, google drive, icloude, SMB, Mega. OCR can be optional, user should have some sort of dialog with checkboxes -> OCR and we can show him approximate estimation time according to what he picked. We should have also abiltiy to do it in background(we will keep info in status bar + option to open back that dialog(in progress one)). Example use case, I am using mega cloud and I have a lot of duplicate photos which I would like to remove, so I need - dialog where I can select exact folder/drive to look for duplicates(with options that I suggested, u r free to add more) - another state of tool dialog which will start to work after I clicked start, it is going to show the progress, or if during work some error accured we should have options like continue, revert or restart - after progress finished, we need to have another state of dialog which will going to be some kind of merger, in this dialog I should be able to review duplicate files and unselect or select back if needed. At the end I will click to merge and we simply remove duplicates - we may also have some percentage option like if in case of metadata missmatch we have OCR percentage of 90 that they are similar to each other
- [ ] Trash/recycle bin integration (soft delete, restore) instead of permanent delete
- [ ] Disk usage treemap view (like WinDirStat) per folder
- [ ] Plugin/extension points — low priority, only if long-term extensibility actually needed
- [ ] New settings to show only in the tray or both tray and system menu
