package handler

import (
	"crypto/rand"
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/sofolitltd/barivara-server/internal/middleware"
	"github.com/sofolitltd/barivara-server/internal/models"
	"github.com/sofolitltd/barivara-server/internal/store"
	"github.com/sofolitltd/barivara-server/internal/types"
)

const codeChars = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789" // no 0/O/1/I confusion

func randomCode() string {
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	out := make([]byte, 6)
	for i, v := range b {
		out[i] = codeChars[int(v)%len(codeChars)]
	}
	return string(out)
}

// --- Landlord side: nested under /api/properties/:propertyId/units/:unitId/renters/:renterId ---

func (h *Handler) HandleGenerateJoinCode(c *gin.Context) {
	if _, ok := h.ownsProperty(c, c.Param("propertyId")); !ok {
		return
	}
	code, err := h.Store.GenerateJoinCode(c.Request.Context(), c.Param("renterId"), randomCode())
	if err != nil {
		c.JSON(http.StatusInternalServerError, types.ErrorResp{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"joinCode": code})
}

func (h *Handler) HandleListPendingLinkRequests(c *gin.Context) {
	if _, ok := h.ownsProperty(c, c.Param("propertyId")); !ok {
		return
	}
	reqs, err := h.Store.ListPendingLinkRequestsForRenter(c.Request.Context(), c.Param("renterId"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, types.ErrorResp{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, reqs)
}

func (h *Handler) HandleApproveLinkRequest(c *gin.Context) {
	if _, ok := h.ownsProperty(c, c.Param("propertyId")); !ok {
		return
	}
	req, err := h.Store.GetTenantLinkRequest(c.Request.Context(), c.Param("requestId"))
	if err != nil {
		c.JSON(http.StatusNotFound, types.ErrorResp{Error: "request not found"})
		return
	}
	if err := h.Store.LinkRenterToUser(c.Request.Context(), req.RenterID, req.TenantUserID); err != nil {
		c.JSON(http.StatusInternalServerError, types.ErrorResp{Error: err.Error()})
		return
	}
	if err := h.Store.SetLinkRequestStatus(c.Request.Context(), req.ID, "approved"); err != nil {
		c.JSON(http.StatusInternalServerError, types.ErrorResp{Error: err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) HandleRejectLinkRequest(c *gin.Context) {
	if _, ok := h.ownsProperty(c, c.Param("propertyId")); !ok {
		return
	}
	if err := h.Store.SetLinkRequestStatus(c.Request.Context(), c.Param("requestId"), "rejected"); err != nil {
		c.JSON(http.StatusInternalServerError, types.ErrorResp{Error: err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) HandleLinkTenantByEmail(c *gin.Context) {
	if _, ok := h.ownsProperty(c, c.Param("propertyId")); !ok {
		return
	}
	var req types.LinkByEmailReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, types.ErrorResp{Error: err.Error()})
		return
	}

	tenant, err := h.Store.GetUserByEmail(c.Request.Context(), strings.ToLower(strings.TrimSpace(req.Email)))
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			c.JSON(http.StatusNotFound, types.ErrorResp{Error: "এই ইমেইলে কোনো অ্যাকাউন্ট পাওয়া যায়নি"})
			return
		}
		c.JSON(http.StatusInternalServerError, types.ErrorResp{Error: err.Error()})
		return
	}
	if tenant.Role != "renter" {
		c.JSON(http.StatusBadRequest, types.ErrorResp{Error: "এই অ্যাকাউন্টটি ভাড়াটিয়া অ্যাকাউন্ট নয়"})
		return
	}

	if err := h.Store.LinkRenterToUser(c.Request.Context(), c.Param("renterId"), tenant.ID); err != nil {
		c.JSON(http.StatusInternalServerError, types.ErrorResp{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, tenant)
}

func (h *Handler) HandleUnlinkTenant(c *gin.Context) {
	if _, ok := h.ownsProperty(c, c.Param("propertyId")); !ok {
		return
	}
	if err := h.Store.UnlinkRenter(c.Request.Context(), c.Param("renterId")); err != nil {
		c.JSON(http.StatusInternalServerError, types.ErrorResp{Error: err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

// --- Tenant side: top-level /api/tenant/* (RequireRole("renter")) ---

func (h *Handler) HandleRequestLinkByCode(c *gin.Context) {
	var req types.LinkByCodeReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, types.ErrorResp{Error: err.Error()})
		return
	}

	renter, err := h.Store.GetActiveRenterByJoinCode(c.Request.Context(), strings.ToUpper(strings.TrimSpace(req.Code)))
	if err != nil {
		c.JSON(http.StatusNotFound, types.ErrorResp{Error: "এই কোডের সাথে কোনো ভাড়াটিয়া তথ্য মেলেনি"})
		return
	}

	tenant, err := h.Store.GetUserByID(c.Request.Context(), middleware.UserID(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, types.ErrorResp{Error: err.Error()})
		return
	}

	created, err := h.Store.CreateTenantLinkRequest(c.Request.Context(), models.TenantLinkRequest{
		RenterID: renter.ID, PropertyID: renter.PropertyID, UnitID: renter.UnitID,
		TenantUserID: tenant.ID, TenantName: tenant.Name, TenantEmail: tenant.Email, TenantPhone: tenant.Phone,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, types.ErrorResp{Error: err.Error()})
		return
	}
	c.JSON(http.StatusCreated, created)
}

func (h *Handler) HandleMyPendingLinkRequest(c *gin.Context) {
	req, err := h.Store.GetMyPendingLinkRequest(c.Request.Context(), middleware.UserID(c))
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			c.JSON(http.StatusOK, nil)
			return
		}
		c.JSON(http.StatusInternalServerError, types.ErrorResp{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, req)
}

func (h *Handler) HandleMyLinkedRenter(c *gin.Context) {
	renter, err := h.Store.GetLinkedRenter(c.Request.Context(), middleware.UserID(c))
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			c.JSON(http.StatusOK, nil)
			return
		}
		c.JSON(http.StatusInternalServerError, types.ErrorResp{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, renter)
}

func (h *Handler) HandleMyInvoices(c *gin.Context) {
	renter, err := h.Store.GetLinkedRenter(c.Request.Context(), middleware.UserID(c))
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			c.JSON(http.StatusOK, []models.Invoice{})
			return
		}
		c.JSON(http.StatusInternalServerError, types.ErrorResp{Error: err.Error()})
		return
	}
	invoices, err := h.Store.ListInvoicesByRenter(c.Request.Context(), renter.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, types.ErrorResp{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, invoices)
}
