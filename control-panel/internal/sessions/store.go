// Package sessions owns the control-plane session contract. Kubernetes
// provisioning is deliberately behind the Provisioner seam so this package
// can be tested without a cluster.
package sessions

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"
)

type Status string

const (
	StatusProvisioning Status = "provisioning"
	StatusReady        Status = "ready"
	StatusStopping     Status = "stopping"
	StatusStopped      Status = "stopped"
	StatusError        Status = "error"
)

// Session identifies one private workspace owned by one user.
type Session struct {
	ID           string    `json:"id"`
	Username     string    `json:"username"`
	WorkloadName string    `json:"workloadName"`
	ServiceName  string    `json:"serviceName"`
	RosDomainID  int       `json:"rosDomainId"`
	Status       Status    `json:"status"`
	NodeName     string    `json:"nodeName,omitempty"`
	Error        string    `json:"error,omitempty"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

var (
	ErrSessionExists = errors.New("user already has an active session")
	ErrNotFound      = errors.New("session not found")
	ErrForbidden     = errors.New("session does not belong to user")
	ErrInvalidStatus = errors.New("invalid session status transition")
)

// Provisioner creates and removes the Kubernetes resources for a session.
type Provisioner interface {
	Apply(Session) error
	Delete(Session) error
}

// StatusProvider lets a Kubernetes-backed provisioner reconcile workload
// readiness before the gateway accepts browser traffic.
type StatusProvider interface {
	Ready(Session) (bool, error)
}

// Store tracks session ownership and lifecycle. With a path set, every
// mutation is saved to a JSON file (same crash-safe-ish pattern as the
// nodes store: write the whole map under lock); without a path the store
// is purely in-memory. Production persistence can be added without
// changing the API.
type Store struct {
	mu          sync.RWMutex
	path        string
	sessions    map[string]Session
	byUser      map[string]string
	provisioner Provisioner
}

func NewStore() *Store {
	return NewStoreWithProvisioner(nil)
}

func NewStoreWithProvisioner(provisioner Provisioner) *Store {
	return &Store{
		sessions:    make(map[string]Session),
		byUser:      make(map[string]string),
		provisioner: provisioner,
	}
}

// NewStoreWithPath loads prior sessions from path (absent file starts
// empty) and persists every mutation there. A corrupt file is a hard
// error: silently starting empty would orphan live workloads.
func NewStoreWithPath(path string) (*Store, error) {
	return NewStoreWithProvisionerAndPath(nil, path)
}

// NewStoreWithProvisionerAndPath combines a provisioner with file persistence.
func NewStoreWithProvisionerAndPath(provisioner Provisioner, path string) (*Store, error) {
	s := &Store{
		path:        path,
		sessions:    make(map[string]Session),
		byUser:      make(map[string]string),
		provisioner: provisioner,
	}
	if path == "" {
		return s, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return s, nil
		}
		return nil, fmt.Errorf("read sessions db: %w", err)
	}
	var saved struct {
		Sessions map[string]Session `json:"sessions"`
	}
	if err := json.Unmarshal(data, &saved); err != nil {
		return nil, fmt.Errorf("parse sessions db: %w", err)
	}
	for id, session := range saved.Sessions {
		s.sessions[id] = session
		if session.Status != StatusStopped && session.Status != StatusError {
			if _, taken := s.byUser[session.Username]; !taken {
				s.byUser[session.Username] = id
			}
		}
	}
	return s, nil
}

// Reconcile compares loaded sessions against live workloads: records whose
// Deployment still exists are adopted as-is (readiness re-probes lazily on
// next Current), records whose workload is gone are tombstoned to stopped
// with a message — never silently deleted, never auto-reprovisioned.
// exists reports workload presence; it must fail open (true on error) so a
// transient API outage cannot mass-tombstone live sessions.
func (s *Store) Reconcile(exists func(Session) bool) (adopted, tombstoned int) {
	s.mu.RLock()
	live := make([]Session, 0, len(s.sessions))
	for _, session := range s.sessions {
		switch session.Status {
		case StatusProvisioning, StatusReady, StatusStopping:
			live = append(live, session)
		}
	}
	s.mu.RUnlock()
	for _, session := range live {
		if exists(session) {
			adopted++
			continue
		}
		if _, err := s.setStatus(session.Username, session.ID, StatusStopped, "workload gone at restart"); err == nil {
			tombstoned++
		}
	}
	return adopted, tombstoned
}

// saveLocked writes the full session map. Caller must hold the lock,
// mirroring the nodes store pattern.
func (s *Store) saveLocked() error {
	if s.path == "" {
		return nil
	}
	data, err := json.Marshal(struct {
		Sessions map[string]Session `json:"sessions"`
	}{Sessions: s.sessions})
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, data, 0o600)
}

func (s *Store) Create(username string) (Session, error) {
	if username == "" {
		return Session{}, errors.New("username is required")
	}

	s.mu.Lock()
	if id, ok := s.byUser[username]; ok {
		existing := s.sessions[id]
		if existing.Status != StatusStopped && existing.Status != StatusError {
			s.mu.Unlock()
			return Session{}, ErrSessionExists
		}
	}
	id, err := newID()
	if err != nil {
		s.mu.Unlock()
		return Session{}, fmt.Errorf("generate session id: %w", err)
	}
	domainID, err := s.nextDomainIDLocked()
	if err != nil {
		s.mu.Unlock()
		return Session{}, err
	}
	now := time.Now().UTC()
	session := Session{
		ID:           id,
		Username:     username,
		WorkloadName: "ros2-session-" + id,
		ServiceName:  "ros2-session-" + id,
		RosDomainID:  domainID,
		Status:       StatusProvisioning,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	s.sessions[id] = session
	prevID, hadPrev := s.byUser[username]
	s.byUser[username] = id
	if err := s.saveLocked(); err != nil {
		delete(s.sessions, id)
		if hadPrev {
			s.byUser[username] = prevID
		} else {
			delete(s.byUser, username)
		}
		s.mu.Unlock()
		return Session{}, fmt.Errorf("persist session: %w", err)
	}
	provisioner := s.provisioner
	s.mu.Unlock()

	if provisioner != nil {
		if err := provisioner.Apply(session); err != nil {
			_, _ = s.setStatus(username, id, StatusError, err.Error())
			return Session{}, fmt.Errorf("provision session: %w", err)
		}
	}
	return session, nil
}

func (s *Store) List(username string) []Session {
	s.mu.RLock()
	defer s.mu.RUnlock()
	id, ok := s.byUser[username]
	if !ok {
		return []Session{}
	}
	return []Session{s.sessions[id]}
}

func (s *Store) Get(username, id string) (Session, error) {
	s.mu.RLock()
	session, ok := s.sessions[id]
	if !ok {
		s.mu.RUnlock()
		return Session{}, ErrNotFound
	}
	if session.Username != username {
		s.mu.RUnlock()
		return Session{}, ErrForbidden
	}
	provisioner := s.provisioner
	needsProbe := session.Status == StatusProvisioning
	s.mu.RUnlock()
	// Same lazy readiness as Current: the launch stepper polls this
	// endpoint, so it must heal provisioning -> ready (R4 prerequisite).
	if needsProbe {
		if statusProvider, ok := provisioner.(StatusProvider); ok {
			ready, err := statusProvider.Ready(session)
			if err != nil {
				return Session{}, fmt.Errorf("check session readiness: %w", err)
			}
			if ready {
				return s.setStatus(username, id, StatusReady, "")
			}
		}
	}
	return session, nil
}

// Current returns the authenticated user's active session.
func (s *Store) Current(username string) (Session, error) {
	s.mu.RLock()
	id, ok := s.byUser[username]
	if !ok {
		s.mu.RUnlock()
		return Session{}, ErrNotFound
	}
	session := s.sessions[id]
	provisioner := s.provisioner
	s.mu.RUnlock()
	if statusProvider, ok := provisioner.(StatusProvider); ok && session.Status == StatusProvisioning {
		ready, err := statusProvider.Ready(session)
		if err != nil {
			return Session{}, fmt.Errorf("check session readiness: %w", err)
		}
		if ready {
			session, err = s.setStatus(username, id, StatusReady, "")
			if err != nil {
				return Session{}, err
			}
		}
	}
	if session.Status == StatusStopped || session.Status == StatusError {
		return Session{}, ErrNotFound
	}
	return session, nil
}

func (s *Store) SetStatus(username, id string, status Status, message string) (Session, error) {
	return s.setStatus(username, id, status, message)
}

func (s *Store) setStatus(username, id string, status Status, message string) (Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	session, ok := s.sessions[id]
	if !ok {
		return Session{}, ErrNotFound
	}
	if session.Username != username {
		return Session{}, ErrForbidden
	}
	if !validTransition(session.Status, status) {
		return Session{}, fmt.Errorf("%w: %s -> %s", ErrInvalidStatus, session.Status, status)
	}
	session.Status = status
	session.Error = message
	session.UpdatedAt = time.Now().UTC()
	s.sessions[id] = session
	// Durability is best-effort after the in-memory truth: on save failure
	// the mutation stands (callers see an error) and the next mutation
	// re-saves. Rolling back real state over a disk error would be worse.
	if err := s.saveLocked(); err != nil {
		return session, fmt.Errorf("persist session: %w", err)
	}
	return session, nil
}

func (s *Store) Delete(username, id string) (Session, error) {
	session, err := s.Get(username, id)
	if err != nil {
		return Session{}, err
	}
	if session.Status == StatusStopped {
		return session, nil
	}
	if s.provisioner != nil {
		if err := s.provisioner.Delete(session); err != nil {
			return Session{}, fmt.Errorf("delete session: %w", err)
		}
	}
	return s.setStatus(username, id, StatusStopped, "")
}

func (s *Store) nextDomainIDLocked() (int, error) {
	used := make(map[int]bool)
	for _, session := range s.sessions {
		if session.Status != StatusStopped && session.Status != StatusError {
			used[session.RosDomainID] = true
		}
	}
	for id := 100; id <= 232; id++ {
		if !used[id] {
			return id, nil
		}
	}
	return 0, errors.New("no ROS domain IDs available")
}

func validTransition(from, to Status) bool {
	switch from {
	case StatusProvisioning:
		return to == StatusReady || to == StatusError || to == StatusStopping || to == StatusStopped
	case StatusReady:
		return to == StatusStopping || to == StatusError || to == StatusStopped
	case StatusStopping:
		return to == StatusStopped || to == StatusError
	default:
		return false
	}
}

func newID() (string, error) {
	var bytes [6]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", bytes), nil
}
