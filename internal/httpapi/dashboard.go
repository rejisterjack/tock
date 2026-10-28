package httpapi

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/rejisterjack/tock/internal/app"
	"github.com/rejisterjack/tock/internal/check"
	"github.com/rejisterjack/tock/internal/store"
)

func (s *Server) checksGET(w http.ResponseWriter, r *http.Request) {
	page, ok := s.view(w, r)
	if !ok {
		return
	}
	checks, err := s.svc.ListChecks(r.Context())
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	now := s.clock()
	page.Title = "Jobs"
	page.Nav = "checks"
	page.Empty = len(checks) == 0
	for _, c := range checks {
		page.Checks = append(page.Checks, s.presentCheck(c, now))
	}
	s.render(w, http.StatusOK, "checks", page)
}

func (s *Server) checkNewGET(w http.ResponseWriter, r *http.Request) {
	page, ok := s.view(w, r)
	if !ok {
		return
	}
	if s.cfg.Demo {
		http.Error(w, "demo mode is read-only", http.StatusForbidden)
		return
	}
	page.Title = "New check"
	page.Nav = "checks"
	page.Form = CheckForm{PeriodValue: 1, PeriodUnit: "days", GraceValue: 30, GraceUnit: "minutes"}
	s.render(w, http.StatusOK, "check_form", page)
}

func (s *Server) checkCreate(w http.ResponseWriter, r *http.Request) {
	page, ok := s.mutate(w, r)
	if !ok {
		return
	}
	in, form, err := parseCheckForm(r)
	if err != nil {
		page.Title = "New check"
		page.Nav = "checks"
		page.Form = form
		page.Error = err.Error()
		s.render(w, http.StatusBadRequest, "check_form", page)
		return
	}
	created, err := s.svc.CreateCheck(r.Context(), in)
	if err != nil {
		s.renderCheckErr(w, page, "New check", form, false, err)
		return
	}
	http.Redirect(w, r, "/checks/"+created.ID, http.StatusSeeOther)
}

func (s *Server) checkGET(w http.ResponseWriter, r *http.Request) {
	page, ok := s.view(w, r)
	if !ok {
		return
	}
	c, err := s.svc.GetCheck(r.Context(), r.PathValue("id"))
	if errors.Is(err, store.ErrNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	pings, err := s.svc.ListPings(r.Context(), c.ID, 50)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	alerts, err := s.svc.ListAlerts(r.Context(), c.ID, 50)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	page.Title = c.Name
	page.Nav = "checks"
	page.Check = s.presentCheck(c, s.clock())
	if len(pings) > 0 {
		page.Pings = pings
	}
	if len(alerts) > 0 {
		page.Alerts = alerts
	}
	s.render(w, http.StatusOK, "check_detail", page)
}

func (s *Server) checkEditGET(w http.ResponseWriter, r *http.Request) {
	page, ok := s.view(w, r)
	if !ok {
		return
	}
	if s.cfg.Demo {
		http.Error(w, "demo mode is read-only", http.StatusForbidden)
		return
	}
	c, err := s.svc.GetCheck(r.Context(), r.PathValue("id"))
	if errors.Is(err, store.ErrNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	view := s.presentCheck(c, s.clock())
	page.Title = "Edit " + c.Name
	page.Nav = "checks"
	page.Form = CheckForm{
		ID: c.ID, Name: c.Name, Editing: true,
		PeriodValue: view.PeriodValue, PeriodUnit: view.PeriodUnit,
		GraceValue: view.GraceValue, GraceUnit: view.GraceUnit,
	}
	s.render(w, http.StatusOK, "check_form", page)
}

func (s *Server) checkUpdate(w http.ResponseWriter, r *http.Request) {
	page, ok := s.mutate(w, r)
	if !ok {
		return
	}
	id := r.PathValue("id")
	in, form, err := parseCheckForm(r)
	form.ID = id
	form.Editing = true
	if err != nil {
		page.Title = "Edit check"
		page.Nav = "checks"
		page.Form = form
		page.Error = err.Error()
		s.render(w, http.StatusBadRequest, "check_form", page)
		return
	}
	if _, err := s.svc.UpdateCheck(r.Context(), id, in); err != nil {
		s.renderCheckErr(w, page, "Edit check", form, true, err)
		return
	}
	http.Redirect(w, r, "/checks/"+id, http.StatusSeeOther)
}

func (s *Server) checkPause(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.mutate(w, r); !ok {
		return
	}
	id := r.PathValue("id")
	if _, err := s.svc.Pause(r.Context(), id); err != nil {
		s.writePageErr(w, r, err)
		return
	}
	http.Redirect(w, r, "/checks/"+id, http.StatusSeeOther)
}

func (s *Server) checkResume(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.mutate(w, r); !ok {
		return
	}
	id := r.PathValue("id")
	if _, err := s.svc.Resume(r.Context(), id); err != nil {
		s.writePageErr(w, r, err)
		return
	}
	http.Redirect(w, r, "/checks/"+id, http.StatusSeeOther)
}

func (s *Server) checkDelete(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.mutate(w, r); !ok {
		return
	}
	if err := s.svc.DeleteCheck(r.Context(), r.PathValue("id")); err != nil {
		s.writePageErr(w, r, err)
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *Server) channelsGET(w http.ResponseWriter, r *http.Request) {
	page, ok := s.view(w, r)
	if !ok {
		return
	}
	channels, err := s.svc.ListChannels(r.Context())
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	page.Title = "Channels"
	page.Nav = "channels"
	for _, c := range channels {
		page.Channels = append(page.Channels, presentChannel(c))
	}
	s.render(w, http.StatusOK, "channels", page)
}

func (s *Server) channelCreate(w http.ResponseWriter, r *http.Request) {
	page, ok := s.mutate(w, r)
	if !ok {
		return
	}
	in, err := parseChannelForm(r)
	if err != nil {
		s.renderChannelsErr(w, r, page, err)
		return
	}
	if _, err := s.svc.CreateChannel(r.Context(), in); err != nil {
		s.renderChannelsErr(w, r, page, err)
		return
	}
	http.Redirect(w, r, "/channels", http.StatusSeeOther)
}

func (s *Server) channelUpdate(w http.ResponseWriter, r *http.Request) {
	page, ok := s.mutate(w, r)
	if !ok {
		return
	}
	in, err := parseChannelForm(r)
	if err != nil {
		s.renderChannelsErr(w, r, page, err)
		return
	}
	if _, err := s.svc.UpdateChannel(r.Context(), r.PathValue("id"), in); err != nil {
		s.renderChannelsErr(w, r, page, err)
		return
	}
	http.Redirect(w, r, "/channels", http.StatusSeeOther)
}

func (s *Server) channelDelete(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.mutate(w, r); !ok {
		return
	}
	if err := s.svc.DeleteChannel(r.Context(), r.PathValue("id")); err != nil {
		s.writePageErr(w, r, err)
		return
	}
	http.Redirect(w, r, "/channels", http.StatusSeeOther)
}

func parseCheckForm(r *http.Request) (app.CheckInput, CheckForm, error) {
	form := CheckForm{
		Name:       r.FormValue("name"),
		PeriodUnit: r.FormValue("period_unit"),
		GraceUnit:  r.FormValue("grace_unit"),
	}
	if form.PeriodUnit == "" {
		form.PeriodUnit = "hours"
	}
	if form.GraceUnit == "" {
		form.GraceUnit = "minutes"
	}
	pv, err := strconv.Atoi(blankZero(r.FormValue("period_value")))
	if err != nil {
		return app.CheckInput{}, form, &check.ValidationError{Message: "period must be a whole number"}
	}
	gv, err := strconv.Atoi(blankZero(r.FormValue("grace_value")))
	if err != nil {
		return app.CheckInput{}, form, &check.ValidationError{Message: "grace period must be a whole number"}
	}
	form.PeriodValue = pv
	form.GraceValue = gv
	period, err := check.ParseInterval(pv, form.PeriodUnit)
	if err != nil {
		return app.CheckInput{}, form, err
	}
	grace, err := check.ParseInterval(gv, form.GraceUnit)
	if err != nil {
		return app.CheckInput{}, form, err
	}
	in := app.CheckInput{Name: form.Name, PeriodSeconds: period, GraceSeconds: grace}
	if err := check.Validate(in.Name, in.PeriodSeconds, in.GraceSeconds); err != nil {
		return in, form, err
	}
	return in, form, nil
}

func parseChannelForm(r *http.Request) (app.ChannelInput, error) {
	kind := r.FormValue("kind")
	cfg, err := channelConfig(kind, r.FormValue("to"), r.FormValue("webhook_url"), r.FormValue("url"), r.FormValue("headers"))
	if err != nil {
		return app.ChannelInput{}, err
	}
	enabled := r.FormValue("enabled") == "true"
	return app.ChannelInput{
		Kind:    kind,
		Name:    r.FormValue("name"),
		Config:  cfg,
		Enabled: &enabled,
	}, nil
}

func (s *Server) renderCheckErr(w http.ResponseWriter, page Page, title string, form CheckForm, editing bool, err error) {
	form.Editing = editing
	page.Title = title
	page.Nav = "checks"
	page.Form = form
	var ve *check.ValidationError
	if errors.As(err, &ve) || errors.Is(err, app.ErrReadOnly) {
		page.Error = err.Error()
		status := http.StatusBadRequest
		if errors.Is(err, app.ErrReadOnly) {
			status = http.StatusForbidden
		}
		s.render(w, status, "check_form", page)
		return
	}
	if errors.Is(err, store.ErrNotFound) {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	s.log.Error("check form", "err", err)
	http.Error(w, "internal error", http.StatusInternalServerError)
}

func (s *Server) renderChannelsErr(w http.ResponseWriter, r *http.Request, page Page, err error) {
	channels, listErr := s.svc.ListChannels(r.Context())
	if listErr != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	page.Title = "Channels"
	page.Nav = "channels"
	page.Error = err.Error()
	for _, c := range channels {
		page.Channels = append(page.Channels, presentChannel(c))
	}
	status := http.StatusBadRequest
	if errors.Is(err, store.ErrNotFound) {
		status = http.StatusNotFound
	}
	s.render(w, status, "channels", page)
}

func (s *Server) writePageErr(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, store.ErrNotFound) {
		http.NotFound(w, r)
		return
	}
	s.log.Error("mutation", "err", err)
	http.Error(w, "internal error", http.StatusInternalServerError)
}

func blankZero(s string) string {
	if s == "" {
		return "0"
	}
	return s
}
