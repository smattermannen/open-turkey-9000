// Package winagent is the session agent on Windows (ROADMAP §4 W1, W3):
// a service runs in session 0 and can't reach a desktop, so it starts
// `kc9k agent` as the signed-in user in each session; the agent does the
// desktop work (window titles, polite close, hotkeys, notifications, idle,
// gamma, wallpapers, the challenge window) and answers the service over
// its own pipe. The agent decides nothing: it does what the service asks
// and reports what it sees.
//
// This file, the messages, is shared so it is tested on Linux too: one
// JSON object per line each way. The service sends a Request with an ID;
// the agent answers with a Reply carrying the same ID. The agent may also
// send Events (ID 0): a hotkey pressed, the session locked.
package winagent

import (
	"encoding/json"
	"time"
)

// AgentPipe is the pipe agents connect to.
const AgentPipe = `\\.\pipe\kc9k-agent`

// ServicePipe is the pipe commands the service runs use to reach agents
// (SYSTEM only): only the service holds the agents' connections.
const ServicePipe = `\\.\pipe\kc9k-agents-internal`

// Request is something the service asks an agent to do.
type Request struct {
	ID   uint64          `json:"id"`
	Op   string          `json:"op"`
	Args json.RawMessage `json:"args,omitempty"`
}

// Reply answers a Request, or is an Event when ID is 0.
type Reply struct {
	ID     uint64          `json:"id"`
	Error  string          `json:"error,omitempty"`
	Result json.RawMessage `json:"result,omitempty"`
	Event  string          `json:"event,omitempty"`
}

// Hello is the agent's first message.
type Hello struct {
	Session uint32 `json:"session"`
	User    string `json:"user"`
	PID     uint32 `json:"pid"`
}

// Status is the answer to OpStatus.
type Status struct {
	IdleMillis uint32 `json:"idle_ms"`
	Locked     bool   `json:"locked"`
}

// Ops the agent answers.
const (
	OpStatus = "status"
)

// CallTimeout bounds one call: a hung agent must not hold the cycle up.
const CallTimeout = 5 * time.Second
