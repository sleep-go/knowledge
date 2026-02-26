# Add Chat File Upload Spec

## Why
Users currently have to go to Settings to upload files to the Knowledge Base. Integrating file upload directly into the chat interface provides a more seamless and intuitive experience for adding context to conversations.

## What Changes
- **Chat Input UI**: Add a file attachment button (paperclip icon) next to the message input.
- **File Selection**: Allow users to select a file, displaying a preview chip before sending.
- **Upload Flow**: When the user sends the message, automatically upload the selected file to the Knowledge Base first, then proceed with the message.

## Impact
- **Affected Specs**: `improve-kb-and-ux` (builds upon the API).
- **Affected Code**:
  - `web/index.html`: Add upload button and file preview container.
  - `web/static/script.js`: Handle file selection, upload process, and message sending sequence.
  - `web/static/style.css`: Style the new button and file preview.

## ADDED Requirements

### Requirement: Chat Interface File Upload
The system SHALL provide a file upload button within the chat input area.

#### Scenario: Select File
- **WHEN** user clicks the attachment button
- **THEN** a file picker dialog opens (accepting supported formats).
- **WHEN** a file is selected
- **THEN** a preview chip (filename + remove button) appears above the input box.

#### Scenario: Send with File
- **WHEN** user clicks Send with a file selected
- **THEN** the system first uploads the file to `/api/kb/upload`.
- **IF** upload is successful
    - **THEN** the file chip is removed.
    - **AND** a system/notification message "File [name] added to Knowledge Base" is shown (or logged).
    - **AND** the user's text message is sent to the chat.
- **IF** upload fails
    - **THEN** an error alert is shown, and the message is NOT sent.

## MODIFIED Requirements
### Requirement: Message Sending Logic
**Modified**: The `sendMessage` function must now check for a pending file upload and handle it before sending the text message.
