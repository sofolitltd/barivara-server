package handler

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sofolitltd/barivara-server/internal/middleware"
	"github.com/sofolitltd/barivara-server/internal/store"
	"github.com/sofolitltd/barivara-server/internal/types"
)

type createNoticeRequest struct {
	Title     string     `json:"title"`
	Body      string     `json:"body"`
	Priority  string     `json:"priority"`
	ExpiresAt *time.Time `json:"expiresAt"`
}

func (h *Handler) HandleCreateNotice(c *gin.Context) {
	propertyID := c.Param("propertyId")
	if _, ok := h.ownsProperty(c, propertyID); !ok {
		return
	}
	var req createNoticeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, types.ErrorResp{Error: err.Error()})
		return
	}
	req.Title = strings.TrimSpace(req.Title)
	req.Body = strings.TrimSpace(req.Body)
	if req.Title == "" || req.Body == "" {
		c.JSON(http.StatusBadRequest, types.ErrorResp{Error: "title and body are required"})
		return
	}
	if req.Priority != "urgent" {
		req.Priority = "normal"
	}
	notice, recipients, err := h.Store.CreateNotice(c.Request.Context(), middleware.UserID(c), propertyID,
		req.Title, req.Body, req.Priority, req.ExpiresAt)
	if err != nil {
		c.JSON(http.StatusInternalServerError, types.ErrorResp{Error: err.Error()})
		return
	}

	for _, recipient := range recipients {
		if recipient.Token == "" {
			continue
		}
		_ = h.Push.Send(c.Request.Context(), recipient.Token, req.Title, req.Body, map[string]string{
			"type":     "notice",
			"noticeId": notice.ID,
		})
	}
	c.JSON(http.StatusCreated, notice)
}

func (h *Handler) HandleListPropertyNotices(c *gin.Context) {
	propertyID := c.Param("propertyId")
	if _, ok := h.ownsProperty(c, propertyID); !ok {
		return
	}
	notices, err := h.Store.ListNoticesByProperty(c.Request.Context(), propertyID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, types.ErrorResp{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, notices)
}

func (h *Handler) HandleListTenantNotices(c *gin.Context) {
	notices, err := h.Store.ListNoticesForTenant(c.Request.Context(), middleware.UserID(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, types.ErrorResp{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, notices)
}

func (h *Handler) HandleMarkTenantNoticeRead(c *gin.Context) {
	if err := h.Store.MarkNoticeRead(c.Request.Context(), middleware.UserID(c), c.Param("noticeId")); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			c.JSON(http.StatusNotFound, types.ErrorResp{Error: "notice not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, types.ErrorResp{Error: err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}
