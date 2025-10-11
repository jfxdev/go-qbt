# Changelog

All notable changes to this project will be documented in this file.

## [Unreleased]

### Fixed
- **Session Expiration Handling**: Fixed issue where the client would continuously return "forbidden" errors after several hours when the qBittorrent Web UI session expired.
  - The client now detects 401 (Unauthorized) and 403 (Forbidden) HTTP status codes
  - Automatically invalidates cached cookies when authentication errors occur
  - Forces a new login on the next retry attempt
  - Operations now complete seamlessly without user intervention
  
  **Technical Details:**
  - Modified `doWithRetry()` in `sdk.go` to detect authentication errors
  - Added automatic cookie invalidation before retrying on 401/403 errors
  - This ensures the retry mechanism will perform a fresh login with valid credentials
  
  **Impact:**
  - Long-running applications will no longer fail after qBittorrent session timeout
  - Transparent re-authentication without exposing errors to the application layer
  - Works with any `WebUISessionTimeout` configuration in qBittorrent settings

### Added
- New unit test `TestInvalidateCookiesOnAuthError()` to verify session expiration handling
- Enhanced documentation in README about error handling and session management

### Changed
- Removed unused `ensureLoginSimple()` function from `client.go`

