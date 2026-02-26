# Tasks

- [x] Task 1: Implement File Preview UI
  - [x] Update `web/index.html` to add a file preview modal structure.
  - [x] Update `web/static/style.css` to style the preview modal.
  - [x] Modify `loadKBFiles` in `web/static/script.js` to make file names clickable.
  - [x] Implement `previewFile(fileId)` function in `script.js` to fetch and display content.
    - [x] Reuse `/api/kb/download` endpoint to fetch content.
    - [x] Render text/markdown content in the modal.

- [x] Task 2: Create MacOS App Packaging Script
  - [x] Create a script `scripts/build_app.sh` to:
    - [x] Compile the Go binary.
    - [x] Create the `.app` directory structure (`Contents/MacOS`, `Contents/Resources`).
    - [x] Create `Info.plist`.
    - [x] Copy the binary and necessary assets (if any) into the bundle.
    - [x] (Optional) Add an app icon.
  - [x] Add a `make app` target to the `Makefile` (if exists) or document the build command.

- [x] Task 3: Verify Packaging
  - [x] Run the build script.
  - [x] Verify the `.app` launches correctly and the server starts.
