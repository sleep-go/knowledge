# Tasks

- [x] Task 1: Update Chat UI with File Upload Button
  - [x] SubTask 1.1: Add a file upload button (icon) to `web/index.html` within the chat input container.
  - [x] SubTask 1.2: Add a file preview container (to show selected file) above or within the input area in `web/index.html`.
  - [x] SubTask 1.3: Update `web/static/style.css` to style the button and preview container.

- [x] Task 2: Implement File Upload Logic in Chat
  - [x] SubTask 2.1: Modify `web/static/script.js` to handle file selection (click, change events).
  - [x] SubTask 2.2: Implement file preview logic (show filename, remove button).
  - [x] SubTask 2.3: Update `sendMessage` in `web/static/script.js` to:
    - Check if a file is selected.
    - If so, call `/api/kb/upload` first.
    - Handle success (clear file, show status) and failure (alert).
    - Proceed to send the text message.

# Task Dependencies
- Task 2 depends on Task 1.
