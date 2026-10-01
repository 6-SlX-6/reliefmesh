package assignments

import (
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/6-slx-6/reliefmesh/apps/api/internal/apperr"
	"github.com/6-slx-6/reliefmesh/apps/api/internal/httpx"
	"github.com/6-slx-6/reliefmesh/apps/api/internal/roles"
)

// Routes mounts assignment endpoints under /api/v1/assignments.
func (s *Service) Routes(r chi.Router) {
	r.Get("/", s.handleList)
	r.Post("/", s.handleCreate)
	r.Get("/{id}", s.handleGet)
	r.Patch("/{id}", s.handleUpdate)
	r.Post("/{id}/status", s.handleStatus)
	r.Post("/{id}/protected", s.handleReveal)
}

func principal(r *http.Request) roles.Principal {
	p, _ := roles.FromContext(r.Context())
	return p
}

func uuidQuery(r *http.Request, name string) (*uuid.UUID, error) {
	v := r.URL.Query().Get(name)
	if v == "" {
		return nil, nil
	}
	id, err := uuid.Parse(v)
	if err != nil {
		return nil, apperr.ValidationField(name, "Must be a UUID.")
	}
	return &id, nil
}

func (s *Service) handleList(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	f := ListFilter{ActiveOnly: q.Get("active") == "true", Mine: q.Get("mine") == "true"}
	var err error
	if f.RequestID, err = uuidQuery(r, "request_id"); err != nil {
		httpx.Error(w, r, err)
		return
	}
	if f.OfferID, err = uuidQuery(r, "offer_id"); err != nil {
		httpx.Error(w, r, err)
		return
	}
	if f.VolunteerID, err = uuidQuery(r, "volunteer_user_id"); err != nil {
		httpx.Error(w, r, err)
		return
	}
	if v := q.Get("status"); v != "" {
		f.Statuses = strings.Split(v, ",")
	}
	if v := q.Get("updated_since"); v != "" {
		t, err := time.Parse(time.RFC3339Nano, v)
		if err != nil {
			httpx.Error(w, r, apperr.ValidationField("updated_since", "Must be an RFC 3339 timestamp."))
			return
		}
		f.UpdatedSince = &t
	}
	if f.Limit, err = httpx.IntQuery(r, "limit", 100, 1, 500); err != nil {
		httpx.Error(w, r, err)
		return
	}
	if f.Offset, err = httpx.IntQuery(r, "offset", 0, 0, 1000000); err != nil {
		httpx.Error(w, r, err)
		return
	}
	res, err := s.List(r.Context(), principal(r), f)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, res)
}

func (s *Service) handleCreate(w http.ResponseWriter, r *http.Request) {
	var in CreateInput
	if err := httpx.Decode(r, &in); err != nil {
		httpx.Error(w, r, err)
		return
	}
	v, created, err := s.Create(r.Context(), principal(r), in)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	status := http.StatusCreated
	if !created {
		status = http.StatusOK
	}
	httpx.JSON(w, status, v)
}

func (s *Service) handleGet(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.UUIDParam(r, "id")
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	v, err := s.Get(r.Context(), principal(r), id)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, v)
}

func (s *Service) handleUpdate(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.UUIDParam(r, "id")
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	var in UpdateInput
	if err := httpx.Decode(r, &in); err != nil {
		httpx.Error(w, r, err)
		return
	}
	v, err := s.Update(r.Context(), principal(r), id, in)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, v)
}

func (s *Service) handleStatus(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.UUIDParam(r, "id")
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	var in StatusInput
	if err := httpx.Decode(r, &in); err != nil {
		httpx.Error(w, r, err)
		return
	}
	v, err := s.ChangeStatus(r.Context(), principal(r), id, in)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, v)
}

func (s *Service) handleReveal(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.UUIDParam(r, "id")
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	res, err := s.RevealProtected(r.Context(), principal(r), id)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, res)
}
