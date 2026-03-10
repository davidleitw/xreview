package codex

import "github.com/davidleitw/xreview/internal/session"

// ShouldResume determines whether to resume an existing codex session
// or start a fresh one.
func ShouldResume(sess *session.Session, fullRescan bool) bool {
	if fullRescan {
		return false
	}
	return sess.CodexSessionID != ""
}
