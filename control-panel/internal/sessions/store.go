// Package sessions owns the control-plane session contract. Kubernetes
// provisioning is deliberately behind the Store seam so this package can be
// tested without a cluster.
package sessions

import (
	"crypto/rand"
	"errors"
	"fmt"
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

// Store is the session persistence/provisioning seam. The current
// implementation is in-memory; the Kubernetes-backed implementation will
// create the workload and service after this contract is accepted.
type Store struct {
	mu       sync.RWMutex
	sessions map[string]Session
	byUser   map[string]string
}

func NewStore() *Store {
	return &Store{sessions: make(map[string]Session), byUser: make(map[string]string)}
}

func (s *Store) Create(username string) (Session, error) {
	if username == "" {
		return Session{}, errors.New("username is required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if id, ok := s.byUser[username]; ok {
		existing := s.sessions[id]
		if existing.Status != StatusStopped && existing.Status != StatusError {
			return Session{}, ErrSessionExists
		}
	}
	id, err := newID()
	if err != nil {
		return Session{}, fmt.Errorf("generate session id: %w", err)
	}
	now := time.Now().UTC()
	session := Session{
		ID:           id,
		Username:     username,
		WorkloadName: "ros2-session-" + id,
		ServiceName:  "ros2-session-" + id,
		RosDomainID:  100 + len(s.sessions),
		Status:       StatusProvisioning,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	s.sessions[id] = session
	s.byUser[username] = id
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
	defer s.mu.RUnlock()
	session, ok := s.sessions[id]
	if !ok {
		return Session{}, ErrNotFound
	}
	if session.Username != username {
		return Session{}, ErrForbidden
	}
	return session, nil
}

func (s *Store) SetStatus(username, id string, status Status, message string) (Session, error) {
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
	return s.SetStatus(username, id, StatusStopped, "")
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
