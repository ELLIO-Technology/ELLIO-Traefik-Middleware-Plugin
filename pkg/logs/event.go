package logs

import (
	"net/http"
	"sync"
	"time"
)

// BlockEvent represents a blocked access event
type BlockEvent struct {
	// Core event info
	Timestamp time.Time `json:"ts"`
	EventType string    `json:"event_type"` // Always "access_blocked"

	// Request info
	Request RequestDetails `json:"request"`
	Client  ClientInfo     `json:"client"`

	// Policy info
	Policy PolicyInfo `json:"policy"`

	// Response
	StatusCode int `json:"status_code"` // Always 403
}

type RequestDetails struct {
	Method string `json:"method"`
	Host   string `json:"host"`
	Path   string `json:"path"`
	Scheme string `json:"scheme"`
}

type ClientInfo struct {
	IP        string `json:"ip"`        // The extracted IP that was checked
	DirectIP  string `json:"direct_ip"` // RemoteAddr for debugging proxy issues
	UserAgent string `json:"user_agent,omitempty"`
}

type PolicyInfo struct {
	Mode string `json:"mode"` // "allowlist" or "blocklist"
}

// Event pool to reduce allocations
var eventPool = sync.Pool{
	New: func() interface{} {
		return &BlockEvent{}
	},
}

// NewBlockEvent creates a new blocked access event using pool
func NewBlockEvent(
	extractedIP string, // The IP that was checked against EDL
	directIP string, // The RemoteAddr
	method string,
	host string,
	path string,
	scheme string,
	userAgent string,
	edlMode string,
) *BlockEvent {
	// Get event from pool
	event := eventPool.Get().(*BlockEvent)

	// Reset and populate the event
	event.Timestamp = time.Now().UTC()
	event.EventType = "access_blocked"
	event.StatusCode = http.StatusForbidden

	event.Request.Method = method
	event.Request.Host = host
	event.Request.Path = path
	event.Request.Scheme = scheme

	event.Client.IP = extractedIP
	event.Client.DirectIP = directIP
	event.Client.UserAgent = userAgent

	event.Policy.Mode = edlMode

	return event
}

// ReturnToPool returns an event to the pool for reuse
func ReturnToPool(event *BlockEvent) {
	// Clear sensitive data before returning to pool
	event.Client.IP = ""
	event.Client.DirectIP = ""
	event.Client.UserAgent = ""
	event.Request.Host = ""
	event.Request.Path = ""
	eventPool.Put(event)
}
