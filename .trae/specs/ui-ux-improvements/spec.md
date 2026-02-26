# UI/UX Improvements Spec

## Why
The user has identified three areas where the current UI/UX can be improved:
1.  **Model Switching**: Currently, the user can interact with the UI while the model is switching, which might lead to inconsistent states.
2.  **Batch Delete Sessions**: The current batch delete UI is functional but lacks visual appeal and advanced styling.
3.  **File List**: Temporary files (starting with `.~`) are cluttering the file list.

## What Changes
1.  **Model Switching**:
    - Lock the UI (disable model selector, show loading state) during model switching.
    - Prevent other interactions until the switch is complete.
2.  **Batch Delete Sessions**:
    - Improve the visual design of the batch delete mode.
    - Add animations for entering/exiting management mode.
    - Style the checkboxes and the action bar to look more modern.
3.  **File List**:
    - Filter out files starting with `.~` from the knowledge base file list.

## Impact
- **Affected files**:
    - `web/static/script.js`: Logic for model switching, batch delete, and file list loading.
    - `web/static/style.css`: Styles for the new batch delete UI and locking state.
    - `web/index.html`: Structure updates for the batch delete UI if needed.

## ADDED Requirements

### Requirement: Model Switching Lock
The system SHALL lock the model selection UI when a switch is initiated.
- **WHEN** the user selects a new model
- **THEN** the model selector should be disabled
- **AND** a "Switching..." indicator should be shown
- **AND** the user should not be able to close the settings modal or interact with other elements until the switch completes.

### Requirement: Filter Temporary Files
The system SHALL exclude temporary files from the KB file list.
- **WHEN** loading the file list
- **THEN** any file whose name starts with `.~` should be excluded from the rendered list.

## MODIFIED Requirements

### Requirement: Batch Delete UI
The system SHALL provide a modern UI for batch deleting sessions.
- **WHEN** entering management mode
- **THEN** the conversation list should shift to reveal checkboxes (or checkboxes appear with animation).
- **AND** a floating or fixed action bar should appear with "Select All", "Delete", and "Cancel" options.
- **AND** the visual style should be consistent with a modern "iOS-like" or "Material" design.
