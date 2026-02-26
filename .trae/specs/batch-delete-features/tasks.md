# Tasks

- [ ] Task 1: Backend Implementation for Batch Delete
    - [ ] DB: Add `DeleteConversations(ids []uint) error` in `internal/db/sqlite.go`.
    - [ ] DB: Add `DeleteKBFiles(ids []uint) error` in `internal/db/sqlite.go` (ensure physical file deletion).
    - [ ] Handler: Add `BatchDeleteConversations` in `internal/server/handler_conversation.go`.
    - [ ] Handler: Add `BatchDeleteKBFiles` in `internal/server/handler_kb.go`.
    - [ ] Router: Register `POST /api/conversations/batch-delete` and `POST /api/kb/files/batch-delete` in `internal/server/router.go`.
- [ ] Task 2: Frontend Implementation for Conversation Batch Delete
    - [ ] HTML: Add "Manage" button and "Delete Selected" container in `web/index.html`.
    - [ ] CSS: Add styles for checkboxes and batch actions in `web/static/style.css`.
    - [ ] JS: Implement toggle logic for management mode in `web/static/script.js`.
    - [ ] JS: Update `renderConversationList` to show checkboxes in management mode.
    - [ ] JS: Implement batch delete API call.
- [ ] Task 3: Frontend Implementation for File Batch Delete
    - [ ] HTML: Add "Delete Selected" button for files in `web/index.html` (settings modal).
    - [ ] JS: Update `loadKBFiles` to render checkboxes.
    - [ ] JS: Implement batch delete API call for files.

# Task Dependencies
- Task 2 and Task 3 depend on Task 1.
