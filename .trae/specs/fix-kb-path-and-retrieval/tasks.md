# Tasks

- [x] Task 1: Fix Folder Path Selection
  - [x] SubTask 1.1: Modify `internal/server/router.go` to use `POSIX path of` in the AppleScript for `/settings/select-folder`.
  - [x] SubTask 1.2: Remove or update the HFS-to-POSIX path conversion logic in Go, as the AppleScript will return the correct path directly.
  - [x] SubTask 1.3: Verify the returned path format (e.g., `/Users/name/docs`).

- [x] Task 2: Fix Knowledge Base Answering
  - [x] SubTask 2.1: Add logging to `augmentHistoryWithKB` (in `internal/server/router.go`) to print the number of retrieved chunks and the query.
  - [x] SubTask 2.2: Modify `augmentHistoryWithKB` to format the retrieved chunks using explicit XML tags (e.g., `<knowledge_base_context>`) and clear instructions.
  - [x] SubTask 2.3: Verify that the LLM receives the augmented prompt (via logs or test output).

# Task Dependencies
- Task 2 depends on successful retrieval logic, which is already present but needs debugging/logging.
