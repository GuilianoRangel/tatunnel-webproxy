package tunnel

import (
	"sync"

	"github.com/hashicorp/yamux"
)

// Registry manages active tunnel sessions
type Registry struct {
	sessions sync.Map // map[string]*yamux.Session
}

func NewRegistry() *Registry {
	return &Registry{}
}

// Register adds a new session. Returns false if subdomain is already taken.
func (r *Registry) Register(subdomain string, session *yamux.Session) bool {
	_, loaded := r.sessions.LoadOrStore(subdomain, session)
	return !loaded 
}

// Unregister removes a session by subdomain.
func (r *Registry) Unregister(subdomain string) {
	r.sessions.Delete(subdomain)
}

// Get retrieves an active session by subdomain.
func (r *Registry) Get(subdomain string) (*yamux.Session, bool) {
	val, ok := r.sessions.Load(subdomain)
	if !ok {
		return nil, false
	}
	return val.(*yamux.Session), true
}
