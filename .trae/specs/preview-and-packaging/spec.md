# File Preview and MacOS App Packaging Spec

## Why
The user wants to improve the usability of the application by allowing direct file preview from the file list and simplifying the distribution process by packaging the application as a standalone MacOS executable.

## What Changes
1.  **File Preview Feature**:
    - Update the file list UI to make file names clickable or add a "Preview" button.
    - Implement a preview modal or new tab mechanism to display the content of the selected file.
    - Support preview for common text-based formats (e.g., .txt, .md) and potentially PDF/images if supported by the browser.
2.  **MacOS Packaging**:
    - Create a script or use a tool (like `appify` or a manual bundle structure) to package the compiled Go binary into a standard `.app` bundle.
    - Ensure the packaged app includes necessary resources (static files, models if needed, or configuration to find them).
    - Add an icon for the application.

## Impact
- **Affected files**:
    - `web/static/script.js`: Update file list rendering and handle preview click events.
    - `web/index.html`: Add a modal container for file preview (optional, or reuse existing).
    - `web/static/style.css`: Style the preview modal and file list links.
    - `Makefile` (or new script `build_mac_app.sh`): Add build steps for the MacOS app bundle.

## ADDED Requirements

### Requirement: File Preview
The system SHALL allow users to view the content of files in the knowledge base.
- **WHEN** the user clicks on a file name in the file list
- **THEN** the system should open a preview window (modal or new tab) displaying the file content.
- **AND** the preview should support at least plain text and markdown rendering.

### Requirement: MacOS Application Bundle
The system SHALL provide a build mechanism to generate a `.app` bundle for MacOS.
- **WHEN** the user runs the build command (e.g., `make app`)
- **THEN** a `Knowledge.app` (or similar) bundle should be created in the `bin` or `dist` directory.
- **AND** the app should be executable via double-click in Finder.

## MODIFIED Requirements
### Requirement: File List UI
The file list items should be interactive.
- **Change**: File names are no longer just text; they are clickable links or have an associated action.

## REMOVED Requirements
N/A
