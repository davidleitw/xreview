package formatter

// Error codes returned by xreview.
const (
	ErrCodexNotFound         = "CODEX_NOT_FOUND"
	ErrCodexNotAuthenticated = "CODEX_NOT_AUTHENTICATED"
	ErrCodexUnresponsive     = "CODEX_UNRESPONSIVE"
	ErrCodexTimeout          = "CODEX_TIMEOUT"
	ErrCodexError            = "CODEX_ERROR"
	ErrParseFailure          = "PARSE_FAILURE"
	ErrSessionNotFound       = "SESSION_NOT_FOUND"
	ErrNoTargets             = "NO_TARGETS"
	ErrInvalidFlags          = "INVALID_FLAGS"
	ErrFileNotFound          = "FILE_NOT_FOUND"
	ErrNotGitRepo            = "NOT_GIT_REPO"
	ErrUpdateFailed          = "UPDATE_FAILED"
	ErrVersionCheckFailed    = "VERSION_CHECK_FAILED"
)

// FormatError produces an XML error response.
func FormatError(action, code, message string) string {
	// TODO: implement — produce <xreview-result status="error" action="...">
	return ""
}
