# Project TODOs & Future Improvements

## Backend Architecture
- [ ] **Robust Strava Upload Polling**: Implement robust async polling for Strava uploads using Cloud Tasks to decouple the upload request from the status check. Currently using a "soft poll" (10s wait) in the function.
- [ ] **Multi-Sport FIT Support**: Enhance FIT generator to support multiple sessions (e.g., Run + Weights) instead of assuming single session.
- [ ] **Heart Rate to FIT Record**: Populate actual `Record` messages in FIT file with HR data instead of just providing a summary stream.

## Infrastructure
- [ ] **Secret Manager Cleanup**: Some secrets still use underscore naming in code comments/docs, though implementation uses hyphens.

## Admin CLI
- [ ] Add explicit status check command for activity pipelines.
