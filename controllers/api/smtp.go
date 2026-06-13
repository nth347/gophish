package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	ctx "github.com/gophish/gophish/context"
	log "github.com/gophish/gophish/logger"
	"github.com/gophish/gophish/models"
	"github.com/gorilla/mux"
	"github.com/jinzhu/gorm"
)

// SendingProfiles handles requests for the /api/smtp/ endpoint
func (as *Server) SendingProfiles(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.Method == "GET":
		ss, err := models.GetSMTPs(ctx.Get(r, "user_id").(int64))
		if err != nil {
			log.Error(err)
		}
		JSONResponse(w, ss, http.StatusOK)
	case r.Method == "POST":
		s := models.SMTP{}
		err := json.NewDecoder(r.Body).Decode(&s)
		if err != nil {
			JSONResponse(w, models.Response{Success: false, Message: "Invalid request"}, http.StatusBadRequest)
			return
		}
		_, err = models.GetSMTPByName(s.Name, ctx.Get(r, "user_id").(int64))
		if err != gorm.ErrRecordNotFound {
			JSONResponse(w, models.Response{Success: false, Message: "SMTP name already in use"}, http.StatusConflict)
			log.Error(err)
			return
		}
		if s.Interface == models.InterfaceTypeOutlookOAuth2 && s.OutlookTokenCacheInput != "" {
			if err := applyOutlookTokenInput(&s); err != nil {
				JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusBadRequest)
				return
			}
		}
		s.ModifiedDate = time.Now().UTC()
		s.UserId = ctx.Get(r, "user_id").(int64)
		err = models.PostSMTP(&s)
		if err != nil {
			JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusInternalServerError)
			return
		}
		JSONResponse(w, s, http.StatusCreated)
	}
}

// SendingProfile handles GET / DELETE / PUT for a single SMTP object.
func (as *Server) SendingProfile(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, _ := strconv.ParseInt(vars["id"], 0, 64)
	s, err := models.GetSMTP(id, ctx.Get(r, "user_id").(int64))
	if err != nil {
		JSONResponse(w, models.Response{Success: false, Message: "SMTP not found"}, http.StatusNotFound)
		return
	}
	switch {
	case r.Method == "GET":
		JSONResponse(w, s, http.StatusOK)
	case r.Method == "DELETE":
		err = models.DeleteSMTP(id, ctx.Get(r, "user_id").(int64))
		if err != nil {
			JSONResponse(w, models.Response{Success: false, Message: "Error deleting SMTP"}, http.StatusInternalServerError)
			return
		}
		JSONResponse(w, models.Response{Success: true, Message: "SMTP Deleted Successfully"}, http.StatusOK)
	case r.Method == "PUT":
		updated := models.SMTP{}
		err = json.NewDecoder(r.Body).Decode(&updated)
		if err != nil {
			log.Error(err)
		}
		if updated.Id != id {
			JSONResponse(w, models.Response{Success: false, Message: "/:id and /:smtp_id mismatch"}, http.StatusBadRequest)
			return
		}
		if updated.Interface == models.InterfaceTypeOutlookOAuth2 {
			if updated.OutlookTokenCacheInput != "" {
				if err := applyOutlookTokenInput(&updated); err != nil {
					JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusBadRequest)
					return
				}
			} else {
				// Keep the token that was already stored — the PUT body won't
				// include it because OutlookTokenCache is json:"-".
				updated.OutlookTokenCache = s.OutlookTokenCache
			}
		}
		err = updated.Validate()
		if err != nil {
			JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusBadRequest)
			return
		}
		updated.ModifiedDate = time.Now().UTC()
		updated.UserId = ctx.Get(r, "user_id").(int64)
		err = models.PutSMTP(&updated)
		if err != nil {
			JSONResponse(w, models.Response{Success: false, Message: "Error updating page"}, http.StatusInternalServerError)
			return
		}
		JSONResponse(w, updated, http.StatusOK)
	}
}

func applyOutlookTokenInput(s *models.SMTP) error {
	token, err := models.ImportOutlookTokenCache(s.OutlookTokenCacheInput)
	if err != nil {
		return err
	}
	tokenJSON, err := json.Marshal(token)
	if err != nil {
		return err
	}
	s.OutlookTokenCache = string(tokenJSON)
	if s.OutlookClientID == "" {
		s.OutlookClientID = models.ExtractClientIDFromMSAL(s.OutlookTokenCacheInput)
	}
	return nil
}
