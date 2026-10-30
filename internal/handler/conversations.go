package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/sofolitltd/barivara-server/internal/middleware"
	"github.com/sofolitltd/barivara-server/internal/store"
	"github.com/sofolitltd/barivara-server/internal/types"
)

func (h *Handler) HandleGetOrCreateConversation(c *gin.Context) {
	conversation, err := h.Store.GetOrCreateConversation(c.Request.Context(), middleware.UserID(c), c.Param("renterId"))
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			c.JSON(http.StatusNotFound, types.ErrorResp{Error: "active renter not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, types.ErrorResp{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, conversation)
}

func (h *Handler) HandleListLandlordConversations(c *gin.Context) {
	conversations, err := h.Store.ListLandlordConversations(c.Request.Context(), middleware.UserID(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, types.ErrorResp{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, conversations)
}

func (h *Handler) HandleGetTenantConversation(c *gin.Context) {
	conversation, err := h.Store.GetTenantConversation(c.Request.Context(), middleware.UserID(c))
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			c.JSON(http.StatusOK, nil)
			return
		}
		c.JSON(http.StatusInternalServerError, types.ErrorResp{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, conversation)
}

func (h *Handler) HandleListConversationMessages(c *gin.Context) {
	messages, err := h.Store.ListConversationMessages(c.Request.Context(), middleware.UserID(c), c.Param("conversationId"))
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			c.JSON(http.StatusNotFound, types.ErrorResp{Error: "conversation not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, types.ErrorResp{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, messages)
}

func (h *Handler) HandleSendConversationMessage(c *gin.Context) {
	var req struct {
		Body string `json:"body"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, types.ErrorResp{Error: err.Error()})
		return
	}
	message, err := h.Store.SendConversationMessage(c.Request.Context(), middleware.UserID(c), c.Param("conversationId"), req.Body)
	if err != nil {
		if err.Error() == "message is required" {
			c.JSON(http.StatusBadRequest, types.ErrorResp{Error: err.Error()})
			return
		}
		if errors.Is(err, store.ErrNotFound) {
			c.JSON(http.StatusNotFound, types.ErrorResp{Error: "conversation not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, types.ErrorResp{Error: err.Error()})
		return
	}
	c.JSON(http.StatusCreated, message)
}
