package api

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"kvui/internal/image"
)

func (s *Server) listImages(w http.ResponseWriter, r *http.Request) {
	c, ok := s.clientsFor(w, r)
	if !ok {
		return
	}
	imgs, err := image.List(r.Context(), c.dyn, c.namespace)
	if err != nil {
		kubeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, imgs)
}

func (s *Server) createImage(w http.ResponseWriter, r *http.Request) {
	var req image.CreateRequest
	if !decode(w, r, &req) {
		return
	}
	c, ok := s.clientsFor(w, r)
	if !ok {
		return
	}
	u := currentUser(r)
	err := image.Create(r.Context(), c.dyn, c.namespace, u.Username, &req)
	if image.IsValidation(err) {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err != nil {
		kubeError(w, r, err)
		return
	}
	s.audit(r, u, "image.create", c.namespace, req.Name)
	writeJSON(w, http.StatusCreated, map[string]string{"name": req.Name})
}

func (s *Server) deleteImage(w http.ResponseWriter, r *http.Request) {
	c, ok := s.clientsFor(w, r)
	if !ok {
		return
	}
	name := chi.URLParam(r, "image")
	err := image.Delete(r.Context(), c.dyn, c.namespace, name)
	if image.IsValidation(err) {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err != nil {
		kubeError(w, r, err)
		return
	}
	s.audit(r, currentUser(r), "image.delete", c.namespace, name)
	w.WriteHeader(http.StatusNoContent)
}

// uploadImage streams the request body (the raw file) into an image that
// is UploadReady.
func (s *Server) uploadImage(w http.ResponseWriter, r *http.Request) {
	if s.UploadProxy == nil {
		writeError(w, http.StatusNotImplemented, "uploads are not configured")
		return
	}
	if r.ContentLength <= 0 {
		writeError(w, http.StatusLengthRequired, "Content-Length is required")
		return
	}
	c, ok := s.clientsFor(w, r)
	if !ok {
		return
	}
	name := chi.URLParam(r, "image")
	err := s.UploadProxy.Upload(r.Context(), c.dyn, c.namespace, name, r.Body, r.ContentLength)
	var rejected *image.UploadError
	switch {
	case image.IsValidation(err):
		writeError(w, http.StatusBadRequest, err.Error())
		return
	case errors.As(err, &rejected):
		writeError(w, http.StatusBadGateway, "CDI refused the upload: "+rejected.Message)
		return
	case err != nil:
		kubeError(w, r, err)
		return
	}
	s.audit(r, currentUser(r), "image.upload", c.namespace, name)
	w.WriteHeader(http.StatusNoContent)
}
