package handler

import (
	"context"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

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

func (h *Handler) HandleGetConversation(c *gin.Context) {
	conversation, err := h.Store.GetConversation(
		c.Request.Context(),
		middleware.UserID(c),
		c.Param("conversationId"),
	)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			c.JSON(http.StatusNotFound, types.ErrorResp{Error: "conversation not found"})
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

func (h *Handler) HandleListTenantConversations(c *gin.Context) {
	conversations, err := h.Store.ListTenantConversations(c.Request.Context(), middleware.UserID(c))
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			c.JSON(http.StatusNotFound, types.ErrorResp{Error: "active renter not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, types.ErrorResp{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, conversations)
}

func (h *Handler) HandleListTenantContacts(c *gin.Context) {
	contacts, err := h.Store.ListTenantContacts(c.Request.Context(), middleware.UserID(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, types.ErrorResp{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, contacts)
}

func (h *Handler) HandleRequestTenantConversation(c *gin.Context) {
	var req struct {
		PeerUserID string `json:"peerUserId" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, types.ErrorResp{Error: err.Error()})
		return
	}
	conversation, err := h.Store.RequestTenantConversation(
		c.Request.Context(), middleware.UserID(c), req.PeerUserID,
	)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			c.JSON(http.StatusNotFound, types.ErrorResp{Error: "একই বাড়ির সক্রিয় ভাড়াটিয়া খুঁজে পাওয়া যায়নি"})
			return
		}
		c.JSON(http.StatusInternalServerError, types.ErrorResp{Error: err.Error()})
		return
	}
	c.JSON(http.StatusCreated, conversation)
}

func (h *Handler) HandleAcceptTenantConversation(c *gin.Context) {
	h.handleTenantConversationDecision(c, "accepted")
}

func (h *Handler) HandleRejectTenantConversation(c *gin.Context) {
	h.handleTenantConversationDecision(c, "rejected")
}

func (h *Handler) handleTenantConversationDecision(c *gin.Context, status string) {
	err := h.Store.SetTenantConversationStatus(
		c.Request.Context(), middleware.UserID(c), c.Param("conversationId"), status,
	)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			c.JSON(http.StatusNotFound, types.ErrorResp{Error: "অনুরোধটি পাওয়া যায়নি বা আর গ্রহণযোগ্য নয়"})
			return
		}
		c.JSON(http.StatusInternalServerError, types.ErrorResp{Error: err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
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
		if errors.Is(err, store.ErrConversationPending) {
			c.JSON(http.StatusConflict, types.ErrorResp{Error: "অনুরোধ গ্রহণ না হওয়া পর্যন্ত বার্তা পাঠানো যাবে না"})
			return
		}
		if errors.Is(err, store.ErrNotFound) {
			c.JSON(http.StatusNotFound, types.ErrorResp{Error: "conversation not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, types.ErrorResp{Error: err.Error()})
		return
	}
	userID := middleware.UserID(c)
	conversationID := c.Param("conversationId")
	go h.sendConversationPush(userID, conversationID, req.Body)
	c.JSON(http.StatusCreated, message)
}

func (h *Handler) sendConversationPush(senderID, conversationID, body string) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	target, err := h.Store.GetConversationPushTarget(ctx, senderID, conversationID)
	if err != nil {
		log.Printf("push notify (conversation %s): resolve recipient: %v", conversationID, err)
		return
	}
	if target.Token == "" {
		return
	}
	preview := strings.TrimSpace(body)
	previewRunes := []rune(preview)
	if len(previewRunes) > 160 {
		preview = string(previewRunes[:157]) + "..."
	}
	if err := h.Push.Send(
		ctx,
		target.Token,
		"নতুন বার্তা: "+target.SenderName,
		preview,
		map[string]string{
			"type":           "chat_message",
			"conversationId": conversationID,
			"peerName":       target.SenderName,
		},
	); err != nil {
		log.Printf("push notify (conversation %s): %v", conversationID, err)
	}
}

func (h *Handler) HandleEditConversationMessage(c *gin.Context) {
	var req struct {
		Body string `json:"body"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, types.ErrorResp{Error: err.Error()})
		return
	}
	err := h.Store.EditConversationMessage(
		c.Request.Context(),
		middleware.UserID(c),
		c.Param("conversationId"),
		c.Param("messageId"),
		req.Body,
	)
	if err != nil {
		if err.Error() == "message is required" {
			c.JSON(http.StatusBadRequest, types.ErrorResp{Error: err.Error()})
			return
		}
		if errors.Is(err, store.ErrConversationPending) {
			c.JSON(http.StatusConflict, types.ErrorResp{Error: "চ্যাট অনুরোধ গ্রহণ হওয়ার পরেই message edit করা যাবে"})
			return
		}
		if errors.Is(err, store.ErrNotFound) {
			c.JSON(http.StatusNotFound, types.ErrorResp{Error: "message পাওয়া যায়নি বা edit করা যাবে না"})
			return
		}
		c.JSON(http.StatusInternalServerError, types.ErrorResp{Error: err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) HandleDeleteConversationMessage(c *gin.Context) {
	var req struct {
		Scope string `json:"scope" binding:"required,oneof=me everyone"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, types.ErrorResp{Error: err.Error()})
		return
	}
	err := h.Store.DeleteConversationMessage(
		c.Request.Context(),
		middleware.UserID(c),
		c.Param("conversationId"),
		c.Param("messageId"),
		req.Scope,
	)
	if err != nil {
		if errors.Is(err, store.ErrConversationPending) {
			c.JSON(http.StatusConflict, types.ErrorResp{Error: "চ্যাট অনুরোধ গ্রহণ হওয়ার পরেই message delete করা যাবে"})
			return
		}
		if errors.Is(err, store.ErrNotFound) {
			c.JSON(http.StatusNotFound, types.ErrorResp{Error: "message পাওয়া যায়নি বা delete করা যাবে না"})
			return
		}
		c.JSON(http.StatusInternalServerError, types.ErrorResp{Error: err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}
