package store

import (
	"time"

	"github.com/rejisterjack/tock/internal/check"
	"github.com/rejisterjack/tock/internal/model"
)

// Pending is an alert row waiting for a delivery attempt, with the check and
// channel needed to send it.
type Pending struct {
	Alert   model.Alert
	Check   model.Check
	Channel model.Channel
}

// BuildAlertRows makes the alert rows for one event. Each enabled channel gets
// a pending row. With no channels, one failed row is recorded immediately so
// the dashboard shows why nobody was notified.
func BuildAlertRows(checkID, kind string, at time.Time, channels []model.Channel) ([]model.Alert, error) {
	at = at.UTC()
	if len(channels) == 0 {
		id, err := check.NewID()
		if err != nil {
			return nil, err
		}
		return []model.Alert{{
			ID: id, CheckID: checkID, Kind: kind, SentAt: at,
			Status: model.AlertFailed, Error: "no notification channels enabled",
		}}, nil
	}
	out := make([]model.Alert, 0, len(channels))
	for _, ch := range channels {
		id, err := check.NewID()
		if err != nil {
			return nil, err
		}
		chID := ch.ID
		out = append(out, model.Alert{
			ID: id, CheckID: checkID, Kind: kind,
			ChannelID: &chID, ChannelName: ch.Name,
			SentAt: at, Status: model.AlertPending,
		})
	}
	return out, nil
}
