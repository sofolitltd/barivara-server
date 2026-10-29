package handler

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/sofolitltd/barivara-server/internal/middleware"
	"github.com/sofolitltd/barivara-server/internal/models"
	"github.com/sofolitltd/barivara-server/internal/types"
)

type SubmitLandlordRequestReq struct {
	UserName           string `json:"userName" binding:"required"`
	Phone              string `json:"phone" binding:"required"`
	NidNumber          string `json:"nidNumber"`
	OwnerProofImageURL string `json:"ownerProofImageUrl"`
}

func (h *Handler) HandleSubmitLandlordRequest(c *gin.Context) {
	var req SubmitLandlordRequestReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, types.ErrorResp{Error: err.Error()})
		return
	}

	userID := middleware.UserID(c)
	created, err := h.Store.CreateLandlordRequest(c.Request.Context(), models.LandlordRequest{
		UserID: userID, UserName: req.UserName, Phone: req.Phone,
		NidNumber: req.NidNumber, OwnerProofImageURL: req.OwnerProofImageURL,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, types.ErrorResp{Error: err.Error()})
		return
	}

	// Best-effort: never fail the request over a notification issue.
	go func() {
		nid := created.NidNumber
		if nid == "" {
			nid = "—"
		}
		text := fmt.Sprintf(
			"🔔 <b>New landlord request</b>\n\nName: %s\nPhone: %s\nNID: %s\n\nReview in the admin panel.",
			created.UserName, created.Phone, nid,
		)
		if err := h.Telegram.SendMessage(text); err != nil {
			log.Printf("telegram notify (landlord request %s): %v", created.ID, err)
		}
	}()

	c.JSON(http.StatusCreated, created)
}

func (h *Handler) HandleListPendingLandlordRequests(c *gin.Context) {
	reqs, err := h.Store.ListPendingLandlordRequests(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, types.ErrorResp{Error: err.Error()})
		return
	}
	if reqs == nil {
		reqs = []models.LandlordRequest{}
	}
	c.JSON(http.StatusOK, reqs)
}

// HandleListAllLandlordRequests returns every landlord application
// (pending/approved/rejected) for the admin "Landlords" page, which
// filters by status client-side instead of calling three endpoints.
func (h *Handler) HandleListAllLandlordRequests(c *gin.Context) {
	reqs, err := h.Store.ListAllLandlordRequests(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, types.ErrorResp{Error: err.Error()})
		return
	}
	if reqs == nil {
		reqs = []models.LandlordRequest{}
	}
	c.JSON(http.StatusOK, reqs)
}

func (h *Handler) HandleApproveLandlordRequest(c *gin.Context) {
	req, err := h.Store.GetLandlordRequest(c.Request.Context(), c.Param("requestId"))
	if err != nil {
		c.JSON(http.StatusNotFound, types.ErrorResp{Error: "request not found"})
		return
	}
	if err := h.Store.UpdateUserRoleStatus(c.Request.Context(), req.UserID, "landlord", "approved"); err != nil {
		c.JSON(http.StatusInternalServerError, types.ErrorResp{Error: err.Error()})
		return
	}
	if err := h.Store.SetLandlordRequestStatus(c.Request.Context(), req.ID, "approved"); err != nil {
		c.JSON(http.StatusInternalServerError, types.ErrorResp{Error: err.Error()})
		return
	}

	// Best-effort: never fail the approval over a notification issue.
	go func() {
		token, err := h.Store.GetFcmToken(context.Background(), req.UserID)
		if err != nil {
			log.Printf("push notify (approve landlord request %s): fetch fcm token: %v", req.ID, err)
			return
		}
		if err := h.Push.Send(
			context.Background(), token,
			"অ্যাকাউন্ট অনুমোদিত হয়েছে!",
			"অভিনন্দন! আপনার বাড়িওয়ালা অ্যাকাউন্ট অনুমোদিত হয়েছে। এখন ড্যাশবোর্ড ব্যবহার করতে পারবেন।",
			map[string]string{"type": "landlord_approved"},
		); err != nil {
			log.Printf("push notify (approve landlord request %s): %v", req.ID, err)
		}
	}()

	c.Status(http.StatusNoContent)
}

func (h *Handler) HandleRejectLandlordRequest(c *gin.Context) {
	req, err := h.Store.GetLandlordRequest(c.Request.Context(), c.Param("requestId"))
	if err != nil {
		c.JSON(http.StatusNotFound, types.ErrorResp{Error: "request not found"})
		return
	}
	if err := h.Store.UpdateUserRoleStatus(c.Request.Context(), req.UserID, "rejected_landlord", "rejected"); err != nil {
		c.JSON(http.StatusInternalServerError, types.ErrorResp{Error: err.Error()})
		return
	}
	if err := h.Store.SetLandlordRequestStatus(c.Request.Context(), req.ID, "rejected"); err != nil {
		c.JSON(http.StatusInternalServerError, types.ErrorResp{Error: err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) HandleListLandlords(c *gin.Context) {
	users, err := h.Store.ListLandlords(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, types.ErrorResp{Error: err.Error()})
		return
	}
	if users == nil {
		users = []models.User{}
	}
	c.JSON(http.StatusOK, users)
}

type SetPlanReq struct {
	Plan string `json:"plan" binding:"required"`
}

func (h *Handler) HandleSetUserPlan(c *gin.Context) {
	var req SetPlanReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, types.ErrorResp{Error: err.Error()})
		return
	}
	if err := h.Store.SetUserPlan(c.Request.Context(), c.Param("userId"), req.Plan); err != nil {
		c.JSON(http.StatusInternalServerError, types.ErrorResp{Error: err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) HandleAdminStats(c *gin.Context) {
	landlords, err := h.Store.ListLandlords(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, types.ErrorResp{Error: err.Error()})
		return
	}
	pending, err := h.Store.CountPendingLandlordRequests(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, types.ErrorResp{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"totalLandlords":       len(landlords),
		"pendingVerifications": pending,
	})
}
