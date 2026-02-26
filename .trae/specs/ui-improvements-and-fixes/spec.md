# UI Improvements and Fixes Spec

## Why
Users have reported several UI issues and missing features that hinder the usability of the application. Specifically, the chat list UI has overlapping elements, file management lacks deletion capabilities, uploaded files in chat are not clickable, model switching is not exposed in the frontend, and the database soft delete mechanism is not desired.

## What Changes
- **Fix Chat List UI**: Adjust CSS to prevent the delete button from overlapping the conversation title.
- **File List Deletion**: Add a delete button to each file in the knowledge base file list and implement backend logic for hard deletion.
- **Clickable Uploaded Files**: Make uploaded files in the chat interface clickable to view/download.
- **Frontend Model Switching**: Add a model selection dropdown in the settings modal.
- **Hard Delete Implementation**: Remove `deleted_at` field from database models and ensure all delete operations are permanent (hard delete).

## Impact
- **Affected specs**: None directly, but improves `add-chat-file-upload` and `implement-model-switching`.
- **Affected code**:
    - `web/static/style.css`: CSS fixes.
    - `web/static/script.js`: Frontend logic for file list, model switching, and chat rendering.
    - `web/index.html`: Add model selector HTML.
    - `internal/server/handler_kb.go`: Add delete and download endpoints.
    - `internal/server/handler_chat.go`: Ensure model list/select APIs are ready.
    - `internal/db/sqlite.go`: Database model changes and delete logic.

## ADDED Requirements
### Requirement: File Management
The system SHALL allow users to manually delete files from the knowledge base file list.
- **Scenario: Delete File**
    - **WHEN** user clicks the delete button next to a file in the settings modal
    - **THEN** the file is permanently removed from the database and disk, and the list is refreshed.

### Requirement: Clickable Files in Chat
The system SHALL render uploaded file names in the chat as clickable links.
- **Scenario: View File**
    - **WHEN** user clicks on an uploaded file link in the chat
    - **THEN** the file content is displayed or downloaded.

### Requirement: Frontend Model Switching
The system SHALL provide a UI for selecting the LLM model.
- **Scenario: Switch Model**
    - **WHEN** user selects a different model from the dropdown in settings
    - **THEN** the backend switches the active model.

## MODIFIED Requirements
### Requirement: Database Deletion
The system SHALL use hard delete instead of soft delete.
- **Reason**: User request to remove `deleted_at` usage.
- **Migration**: Existing `deleted_at` columns will be ignored or dropped (if possible), and new deletes will be permanent.

### Requirement: Chat List UI
The system SHALL ensure the delete button does not obscure the conversation title.
