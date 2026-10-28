package httpapi

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/rejisterjack/tock/internal/app"
	"github.com/rejisterjack/tock/internal/model"
)

type checkBody struct {
	Name          *string `json:"name"`
	PeriodSeconds *int    `json:"period_seconds"`
	GraceSeconds  *int    `json:"grace_seconds"`
}

type checkJSON struct {
	ID            string     `json:"id"`
	Name          string     `json:"name"`
	Status        string     `json:"status"`
	DisplayStatus string     `json:"display_status"`
	PeriodSeconds int        `json:"period_seconds"`
	GraceSeconds  int        `json:"grace_seconds"`
	Paused        bool       `json:"paused"`
	LastPingAt    *time.Time `json:"last_ping_at"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
	PingURL       string     `json:"ping_url"`
	Curl          string     `json:"curl"`
}

type channelJSON struct {
	ID        string    `json:"id"`
	Kind      string    `json:"kind"`
	Name      string    `json:"name"`
	Enabled   bool      `json:"enabled"`
	Config    any       `json:"config"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (s *Server) apiListChecks(w http.ResponseWriter, r *http.Request) {
	if !s.apiAuth(w, r, false) {
		return
	}
	checks, err := s.svc.ListChecks(r.Context())
	if err != nil {
		s.writeErr(w, err)
		return
	}
	out := make([]checkJSON, 0, len(checks))
	now := s.clock()
	for _, c := range checks {
		out = append(out, s.checkJSON(c, now))
	}
	writeJSON(w, http.StatusOK, map[string]any{"checks": out})
}

func (s *Server) apiCreateCheck(w http.ResponseWriter, r *http.Request) {
	if !s.apiAuth(w, r, true) {
		return
	}
	var body checkBody
	if !s.readJSON(w, r, &body) {
		return
	}
	in := app.CheckInput{Name: deref(body.Name)}
	if body.PeriodSeconds != nil {
		in.PeriodSeconds = *body.PeriodSeconds
	}
	if body.GraceSeconds != nil {
		in.GraceSeconds = *body.GraceSeconds
	}
	created, err := s.svc.CreateCheck(r.Context(), in)
	if err != nil {
		s.writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, s.checkJSON(created, s.clock()))
}

func (s *Server) apiGetCheck(w http.ResponseWriter, r *http.Request) {
	if !s.apiAuth(w, r, false) {
		return
	}
	c, err := s.svc.GetCheck(r.Context(), r.PathValue("id"))
	if err != nil {
		s.writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, s.checkJSON(c, s.clock()))
}

func (s *Server) apiPatchCheck(w http.ResponseWriter, r *http.Request) {
	if !s.apiAuth(w, r, true) {
		return
	}
	current, err := s.svc.GetCheck(r.Context(), r.PathValue("id"))
	if err != nil {
		s.writeErr(w, err)
		return
	}
	var body checkBody
	if !s.readJSON(w, r, &body) {
		return
	}
	in := app.CheckInput{
		Name: current.Name, PeriodSeconds: current.PeriodSeconds, GraceSeconds: current.GraceSeconds,
	}
	if body.Name != nil {
		in.Name = *body.Name
	}
	if body.PeriodSeconds != nil {
		in.PeriodSeconds = *body.PeriodSeconds
	}
	if body.GraceSeconds != nil {
		in.GraceSeconds = *body.GraceSeconds
	}
	updated, err := s.svc.UpdateCheck(r.Context(), current.ID, in)
	if err != nil {
		s.writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, s.checkJSON(updated, s.clock()))
}

func (s *Server) apiDeleteCheck(w http.ResponseWriter, r *http.Request) {
	if !s.apiAuth(w, r, true) {
		return
	}
	if err := s.svc.DeleteCheck(r.Context(), r.PathValue("id")); err != nil {
		s.writeErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) apiPause(w http.ResponseWriter, r *http.Request) {
	if !s.apiAuth(w, r, true) {
		return
	}
	c, err := s.svc.Pause(r.Context(), r.PathValue("id"))
	if err != nil {
		s.writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, s.checkJSON(c, s.clock()))
}

func (s *Server) apiResume(w http.ResponseWriter, r *http.Request) {
	if !s.apiAuth(w, r, true) {
		return
	}
	c, err := s.svc.Resume(r.Context(), r.PathValue("id"))
	if err != nil {
		s.writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, s.checkJSON(c, s.clock()))
}

func (s *Server) apiPings(w http.ResponseWriter, r *http.Request) {
	if !s.apiAuth(w, r, false) {
		return
	}
	pings, err := s.svc.ListPings(r.Context(), r.PathValue("id"), queryLimit(r))
	if err != nil {
		s.writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"pings": pings})
}

func (s *Server) apiAlerts(w http.ResponseWriter, r *http.Request) {
	if !s.apiAuth(w, r, false) {
		return
	}
	alerts, err := s.svc.ListAlerts(r.Context(), r.PathValue("id"), queryLimit(r))
	if err != nil {
		s.writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"alerts": alerts})
}

func (s *Server) apiListChannels(w http.ResponseWriter, r *http.Request) {
	if !s.apiAuth(w, r, false) {
		return
	}
	channels, err := s.svc.ListChannels(r.Context())
	if err != nil {
		s.writeErr(w, err)
		return
	}
	out := make([]channelJSON, 0, len(channels))
	for _, c := range channels {
		out = append(out, toChannelJSON(c))
	}
	writeJSON(w, http.StatusOK, map[string]any{"channels": out})
}

func (s *Server) apiCreateChannel(w http.ResponseWriter, r *http.Request) {
	if !s.apiAuth(w, r, true) {
		return
	}
	body, ok := s.readChannel(w, r)
	if !ok {
		return
	}
	created, err := s.svc.CreateChannel(r.Context(), body)
	if err != nil {
		s.writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, toChannelJSON(created))
}

func (s *Server) apiPatchChannel(w http.ResponseWriter, r *http.Request) {
	if !s.apiAuth(w, r, true) {
		return
	}
	existing, err := s.svc.GetChannel(r.Context(), r.PathValue("id"))
	if err != nil {
		s.writeErr(w, err)
		return
	}
	body, ok := s.readChannel(w, r)
	if !ok {
		return
	}
	if body.Kind == "" {
		body.Kind = existing.Kind
	}
	if body.Name == "" {
		body.Name = existing.Name
	}
	if len(body.Config) == 0 {
		body.Config = existing.Config
	}
	updated, err := s.svc.UpdateChannel(r.Context(), existing.ID, body)
	if err != nil {
		s.writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toChannelJSON(updated))
}

func (s *Server) apiDeleteChannel(w http.ResponseWriter, r *http.Request) {
	if !s.apiAuth(w, r, true) {
		return
	}
	if err := s.svc.DeleteChannel(r.Context(), r.PathValue("id")); err != nil {
		s.writeErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) readChannel(w http.ResponseWriter, r *http.Request) (app.ChannelInput, bool) {
	var body struct {
		Kind    string          `json:"kind"`
		Name    string          `json:"name"`
		Enabled *bool           `json:"enabled"`
		Config  json.RawMessage `json:"config"`
	}
	if !s.readJSON(w, r, &body) {
		return app.ChannelInput{}, false
	}
	raw := body.Config
	if string(raw) == "null" {
		raw = nil
	}
	return app.ChannelInput{Kind: body.Kind, Name: body.Name, Config: raw, Enabled: body.Enabled}, true
}

func (s *Server) checkJSON(c model.Check, now time.Time) checkJSON {
	v := s.presentCheck(c, now)
	return checkJSON{
		ID: c.ID, Name: c.Name, Status: c.Status, DisplayStatus: v.DisplayStatus,
		PeriodSeconds: c.PeriodSeconds, GraceSeconds: c.GraceSeconds, Paused: c.Paused,
		LastPingAt: c.LastPingAt, CreatedAt: c.CreatedAt, UpdatedAt: c.UpdatedAt,
		PingURL: v.PingURL, Curl: v.Curl,
	}
}

func toChannelJSON(c model.Channel) channelJSON {
	var cfg any
	if err := json.Unmarshal(c.Config, &cfg); err != nil {
		cfg = map[string]any{}
	}
	return channelJSON{
		ID: c.ID, Kind: c.Kind, Name: c.Name, Enabled: c.Enabled, Config: cfg,
		CreatedAt: c.CreatedAt, UpdatedAt: c.UpdatedAt,
	}
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
