package model

import (
	"encoding/json"
	"time"
)

const (
	StatusNew  = "new"
	StatusUp   = "up"
	StatusDown = "down"

	DisplayGrace  = "grace"
	DisplayPaused = "paused"

	KindEmail   = "email"
	KindSlack   = "slack"
	KindWebhook = "webhook"

	AlertDown      = "down"
	AlertRecovered = "recovered"
	AlertPending   = "pending"
	AlertSent      = "sent"
	AlertFailed    = "failed"
)

// Check is one monitored job. Status is the stored state; the dashboard
// derives "grace" and "paused" when it renders.
type Check struct {
	ID            string
	Name          string
	PingToken     string
	PeriodSeconds int
	GraceSeconds  int
	Status        string
	LastPingAt    *time.Time
	Paused        bool
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// Ping is one successful signal from a job.
type Ping struct {
	ID         int64     `json:"id"`
	CheckID    string    `json:"check_id"`
	ReceivedAt time.Time `json:"received_at"`
	RemoteIP   string    `json:"remote_ip"`
	UserAgent  string    `json:"user_agent"`
}

// PingResult is the outcome of accepting a ping.
type PingResult struct {
	Check         Check
	NeedsRecovery bool
}

// Channel is a notification destination. Config is kind-specific JSON.
type Channel struct {
	ID        string
	Kind      string
	Name      string
	Config    json.RawMessage
	Enabled   bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Alert is one delivery attempt for a down or recovered event.
type Alert struct {
	ID          string     `json:"id"`
	CheckID     string     `json:"check_id"`
	Kind        string     `json:"kind"`
	ChannelID   *string    `json:"channel_id,omitempty"`
	ChannelName string     `json:"channel_name"`
	SentAt      time.Time  `json:"sent_at"`
	Status      string     `json:"status"`
	Error       string     `json:"error,omitempty"`
	ResolvedAt  *time.Time `json:"resolved_at,omitempty"`
}
