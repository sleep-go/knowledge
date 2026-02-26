# Batch Delete Features Spec

## Why
Users have requested the ability to delete multiple conversations and files at once to improve efficiency when managing large numbers of items.

## What Changes
- **Backend**:
    - Add batch delete API for conversations (`POST /api/conversations/batch-delete`).
    - Add batch delete API for knowledge base files (`POST /api/kb/files/batch-delete`).
    - Implement batch delete logic in the database layer.
- **Frontend**:
    - **Conversation List**:
        - Add a "Manage" (管理) button to the sidebar header.
        - When in "Manage" mode, show checkboxes for each conversation.
        - Show a "Delete Selected" (删除选中) button.
    - **File List (Settings)**:
        - Add checkboxes to each file item.
        - Add a "Delete Selected" (删除选中) button next to the file list header or footer.

## Impact
- **Affected specs**: None directly.
- **Affected code**:
    - `internal/db/sqlite.go`: Add `DeleteConversations` and `DeleteKBFiles`.
    - `internal/server/handler_conversation.go`: Add `BatchDeleteConversations`.
    - `internal/server/handler_kb.go`: Add `BatchDeleteKBFiles`.
    - `internal/server/router.go`: Register new routes.
    - `web/index.html`: Add UI elements for batch actions.
    - `web/static/style.css`: Add styles for checkboxes and batch action bars.
    - `web/static/script.js`: Implement frontend logic for batch selection and deletion.

## ADDED Requirements
### Requirement: Batch Delete Conversations
The system SHALL allow users to select multiple conversations and delete them in a single operation.
- **Scenario: Batch Delete**
    - **WHEN** user clicks "Manage", selects conversations, and clicks "Delete Selected"
    - **THEN** the selected conversations are permanently removed.

### Requirement: Batch Delete Files
The system SHALL allow users to select multiple files in the settings modal and delete them in a single operation.
- **Scenario: Batch Delete Files**
    - **WHEN** user selects files in the list and clicks "Delete Selected"
    - **THEN** the selected files and their chunks are permanently removed from database and disk.

## MODIFIED Requirements
### Requirement: Conversation List UI
The conversation list item SHALL support a selection state with a checkbox.

### Requirement: File List UI
The file list item SHALL support a selection state with a checkbox.
