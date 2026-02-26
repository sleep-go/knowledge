# Tasks

- [x] Task 1: Filter Temporary Files
  - [x] Modify `loadKBFiles` in `web/static/script.js` to filter out files starting with `.~`.
  - [x] Verify that temporary files no longer appear in the list.

- [x] Task 2: Model Switching Lock
  - [x] Modify `modelSelect` event listener in `web/static/script.js`.
  - [x] Disable the select element during the request.
  - [x] Add a loading indicator (text change or spinner).
  - [x] Re-enable the element after success or failure.

- [x] Task 3: Improve Batch Delete UI
  - [x] Update `web/index.html` to structure the batch actions bar (if needed).
  - [x] Update `web/static/style.css` to add animations and modern styles for:
    - Checkboxes (custom style).
    - Batch action bar (positioning, shadows, buttons).
    - Conversation items in management mode (hover effects, selection state).
  - [x] Update `web/static/script.js` to toggle classes for animations instead of just `display: none`.
