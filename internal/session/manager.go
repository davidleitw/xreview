package session

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"github.com/davidleitw/xreview/internal/config"
	"github.com/davidleitw/xreview/internal/version"
)

var validSessionID = regexp.MustCompile(`^xr-[0-9]{8}-[0-9a-f]{6}$`)

func validateSessionID(id string) error {
	if !validSessionID.MatchString(id) {
		return fmt.Errorf("invalid session ID format: %q", id)
	}
	return nil
}

// Manager handles session CRUD operations.
type Manager interface {
	Create(targets []string, targetMode, context string, cfg *config.Config) (*Session, error)
	Load(sessionID string) (*Session, error)
	Update(sess *Session) error
	Delete(sessionID string) error
	List() ([]string, error)
}

type manager struct {
	sessionsDir string
}

// NewManager creates a Manager that stores sessions in the given workdir.
func NewManager(workdir string) Manager {
	return &manager{
		sessionsDir: config.SessionsDir(workdir),
	}
}

func (m *manager) Create(targets []string, targetMode, ctx string, cfg *config.Config) (*Session, error) {
	id, err := generateSessionID()
	if err != nil {
		return nil, err
	}

	sess := &Session{
		Version:        CurrentSessionVersion,
		SessionID:      id,
		XReviewVersion: version.Version,
		CreatedAt:      time.Now().UTC(),
		UpdatedAt:      time.Now().UTC(),
		Status:         StatusInitialized,
		Round:          0,
		CodexModel:     cfg.CodexModel,
		Context:        ctx,
		Targets:        targets,
		TargetMode:     targetMode,
		Findings:       []Finding{},
	}

	dir := filepath.Join(m.sessionsDir, id)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create session dir: %w", err)
	}

	if err := m.write(sess); err != nil {
		return nil, err
	}

	return sess, nil
}

func (m *manager) Load(sessionID string) (*Session, error) {
	if err := validateSessionID(sessionID); err != nil {
		return nil, err
	}
	path := filepath.Join(m.sessionsDir, sessionID, "session.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("session %q not found", sessionID)
		}
		return nil, err
	}

	var sess Session
	if err := json.Unmarshal(data, &sess); err != nil {
		return nil, fmt.Errorf("parse session.json: %w", err)
	}

	if sess.SessionID != sessionID {
		return nil, fmt.Errorf("session ID mismatch: file contains %q but requested %q", sess.SessionID, sessionID)
	}

	if sess.Version != CurrentSessionVersion {
		return nil, fmt.Errorf("session %s uses schema version %d (current: %d); please start a new review",
			sessionID, sess.Version, CurrentSessionVersion)
	}

	return &sess, nil
}

func (m *manager) Update(sess *Session) error {
	sess.UpdatedAt = time.Now().UTC()
	return m.write(sess)
}

func (m *manager) Delete(sessionID string) error {
	if err := validateSessionID(sessionID); err != nil {
		return err
	}
	dir := filepath.Join(m.sessionsDir, sessionID)
	return os.RemoveAll(dir)
}

func (m *manager) List() ([]string, error) {
	entries, err := os.ReadDir(m.sessionsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var ids []string
	for _, e := range entries {
		if e.IsDir() {
			ids = append(ids, e.Name())
		}
	}
	return ids, nil
}

func (m *manager) write(sess *Session) error {
	data, err := json.MarshalIndent(sess, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal session: %w", err)
	}

	path := filepath.Join(m.sessionsDir, sess.SessionID, "session.json")
	return os.WriteFile(path, data, 0o644)
}

func generateSessionID() (string, error) {
	b := make([]byte, 3)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate random bytes: %w", err)
	}
	date := time.Now().Format("20060102")
	return fmt.Sprintf("xr-%s-%s", date, hex.EncodeToString(b)), nil
}
