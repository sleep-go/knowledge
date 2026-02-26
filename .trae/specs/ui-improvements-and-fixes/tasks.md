# Tasks

- [x] Task 1: Fix Chat List UI Overlap
    - [x] Update `web/static/style.css` to adjust `conversation-item` layout and `conversation-item-title` width/padding.
- [x] Task 2: Implement Hard Delete in Database
    - [x] Modify `internal/db/sqlite.go` to define `BaseModel` without `DeletedAt`.
    - [x] Update `Conversation`, `Message`, `Setting`, `KnowledgeBaseFile`, `KnowledgeBaseChunk` to use `BaseModel`.
    - [x] Remove `Unscoped()` usage in all DB queries as it's no longer needed.
    - [x] Verify database migration or handling of existing tables.
- [x] Task 3: Implement File List Deletion
    - [x] Backend: Add `DeleteKBFile` in `internal/db/sqlite.go` (ensure physical file and chunks are deleted).
    - [x] Backend: Add `DeleteKBFile` handler in `internal/server/handler_kb.go`.
    - [x] Backend: Register `DELETE /api/kb/files/:id` route in `internal/server/server.go`.
    - [x] Frontend: Update `loadKBFiles` in `web/static/script.js` to render delete button and handle click.
- [x] Task 4: Implement Clickable Uploaded Files
    - [x] Backend: Add `DownloadKBFile` handler in `internal/server/handler_kb.go` to serve file content.
    - [x] Backend: Register `GET /api/kb/download` route.
    - [x] Frontend: Update `sendMessage` or message rendering in `web/static/script.js` to wrap file name in a link pointing to the download API.
- [x] Task 5: Implement Frontend Model Switching
    - [x] Frontend: Add model selector dropdown HTML to `web/index.html` (inside settings modal).
    - [x] Frontend: Add `loadModels` and `switchModel` logic in `web/static/script.js`.
    - [x] Frontend: Call `loadModels` when opening settings.

# Task Dependencies
- Task 3 depends on Task 2 (Hard Delete) to ensure file deletion is permanent.
