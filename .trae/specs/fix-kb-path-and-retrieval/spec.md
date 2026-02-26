# Fix KB Path and Retrieval Spec

## Why
User reported two issues:
1. The folder path displayed after selection is incorrect (uses colon separator instead of slash).
2. The system fails to answer questions based on the configured knowledge base content.

## What Changes

### Fix KB Path Display
- Modify the AppleScript in `internal/server/router.go` (endpoint `/settings/select-folder`) to directly return the POSIX path using `POSIX path of`.
- Remove the fragile manual path conversion logic in Go.

### Fix Knowledge Base Retrieval/Answering
- **Enhance Prompt Construction**: In `augmentHistoryWithKB` (in `internal/server/router.go`), wrap the retrieved chunks in explicit XML tags (e.g., `<knowledge_base_context>`) to help the LLM distinguish between context and user input.
- **Strengthen Instructions**: Update the appended text to explicitly instruct the LLM to prioritize the provided context.
- **Add Debug Logging**: Add logs in `augmentHistoryWithKB` to print the number of chunks found and the generated prompt snippet (truncated) to verify if retrieval is working.

## Impact
- **Affected Code**:
    - `internal/server/router.go`: Folder selection logic and `augmentHistoryWithKB` function.
    - `internal/db/sqlite.go`: (Optional) Minor logging additions if needed, but primary changes are in router.
- **User Experience**:
    - Users will see correct file paths (e.g., `/Users/name/docs`).
    - Users will receive answers that actually reference their knowledge base files.

## ADDED Requirements
### Requirement: Correct Path Format
The `/settings/select-folder` endpoint SHALL return a valid POSIX path (slash-separated) on macOS.

### Requirement: Effective RAG
The system SHALL include retrieved knowledge base chunks in a structured format (XML tags) within the prompt.
The system SHALL log the number of retrieved chunks for every request to aid debugging.

## MODIFIED Requirements
### Requirement: Folder Selection
**Old**: Used `choose folder` returning HFS path, then converted manually.
**New**: Use `POSIX path of (choose folder ...)` to get the correct path directly from AppleScript.

### Requirement: Context Injection
**Old**: Appended "【本地知识库参考资料】..." to the user message.
**New**: Append `<knowledge_base_context>...</knowledge_base_context>` with stricter instructions to the user message.
