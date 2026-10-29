package handler

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sofolitltd/barivara-server/internal/middleware"
	"github.com/sofolitltd/barivara-server/internal/models"
	"github.com/sofolitltd/barivara-server/internal/types"
)

func (h *Handler) HandleCreateInvoice(c *gin.Context) {
	propertyID, unitID := c.Param("propertyId"), c.Param("unitId")
	if _, ok := h.ownsProperty(c, propertyID); !ok {
		return
	}
	var inv models.Invoice
	if err := c.ShouldBindJSON(&inv); err != nil {
		c.JSON(http.StatusBadRequest, types.ErrorResp{Error: err.Error()})
		return
	}
	inv.PropertyID, inv.UnitID = propertyID, unitID
	created, err := h.Store.CreateInvoice(c.Request.Context(), inv)
	if err != nil {
		c.JSON(http.StatusInternalServerError, types.ErrorResp{Error: err.Error()})
		return
	}
	c.JSON(http.StatusCreated, created)
}

func (h *Handler) HandleListInvoicesByUnit(c *gin.Context) {
	propertyID, unitID := c.Param("propertyId"), c.Param("unitId")
	if _, ok := h.ownsProperty(c, propertyID); !ok {
		return
	}
	invoices, err := h.Store.ListInvoicesByUnit(c.Request.Context(), propertyID, unitID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, types.ErrorResp{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, invoices)
}

func (h *Handler) HandleListInvoicesByProperty(c *gin.Context) {
	propertyID := c.Param("propertyId")
	if _, ok := h.ownsProperty(c, propertyID); !ok {
		return
	}
	invoices, err := h.Store.ListInvoicesByProperty(c.Request.Context(), propertyID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, types.ErrorResp{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, invoices)
}

func (h *Handler) HandleListUnpaidInvoicesForOwner(c *gin.Context) {
	invoices, err := h.Store.ListUnpaidInvoicesForOwner(c.Request.Context(), middleware.UserID(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, types.ErrorResp{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, invoices)
}

// HandleGetInvoicePublic serves the unauthenticated invoice-view link
// shared over SMS (mirrors the existing /invoice/:id behavior).
func (h *Handler) HandleGetInvoicePublic(c *gin.Context) {
	inv, err := h.Store.GetInvoice(c.Request.Context(), c.Param("invoiceId"))
	if err != nil {
		c.JSON(http.StatusNotFound, types.ErrorResp{Error: "invoice not found"})
		return
	}
	c.JSON(http.StatusOK, inv)
}

// ownsInvoice looks the invoice up by id and verifies the requester owns
// the property it belongs to; writes an error response and returns
// ok=false if not. Used by every flat /api/invoices/:invoiceId route.
func (h *Handler) ownsInvoice(c *gin.Context) (*models.Invoice, bool) {
	inv, err := h.Store.GetInvoice(c.Request.Context(), c.Param("invoiceId"))
	if err != nil {
		c.JSON(http.StatusNotFound, types.ErrorResp{Error: "invoice not found"})
		return nil, false
	}
	if _, ok := h.ownsProperty(c, inv.PropertyID); !ok {
		return nil, false
	}
	return inv, true
}

func (h *Handler) HandleRecordPayment(c *gin.Context) {
	inv, ok := h.ownsInvoice(c)
	if !ok {
		return
	}
	var req types.RecordPaymentReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, types.ErrorResp{Error: err.Error()})
		return
	}
	updated, err := h.Store.RecordPayment(c.Request.Context(), inv.ID, req.Amount, time.Now(), req.Note, req.PaidItems)
	if err != nil {
		c.JSON(http.StatusInternalServerError, types.ErrorResp{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, updated)
}

func (h *Handler) HandleMarkInvoicePaid(c *gin.Context) {
	inv, ok := h.ownsInvoice(c)
	if !ok {
		return
	}
	updated, err := h.Store.MarkInvoicePaid(c.Request.Context(), inv.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, types.ErrorResp{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, updated)
}

func (h *Handler) HandleUpdateInvoiceStatus(c *gin.Context) {
	inv, ok := h.ownsInvoice(c)
	if !ok {
		return
	}
	var req types.UpdateStatusReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, types.ErrorResp{Error: err.Error()})
		return
	}
	if err := h.Store.UpdateInvoiceStatus(c.Request.Context(), inv.ID, req.Status); err != nil {
		c.JSON(http.StatusInternalServerError, types.ErrorResp{Error: err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

type RemovePaidItemReq struct {
	ItemName string `json:"itemName" binding:"required"`
}

func (h *Handler) HandleRemovePaidItem(c *gin.Context) {
	inv, ok := h.ownsInvoice(c)
	if !ok {
		return
	}
	var req RemovePaidItemReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, types.ErrorResp{Error: err.Error()})
		return
	}
	updated, err := h.Store.RemovePaidItem(c.Request.Context(), inv.ID, req.ItemName)
	if err != nil {
		c.JSON(http.StatusInternalServerError, types.ErrorResp{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, updated)
}

func (h *Handler) HandleClearPayments(c *gin.Context) {
	inv, ok := h.ownsInvoice(c)
	if !ok {
		return
	}
	if err := h.Store.ClearPayments(c.Request.Context(), inv.ID); err != nil {
		c.JSON(http.StatusInternalServerError, types.ErrorResp{Error: err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) HandleUpdateInvoice(c *gin.Context) {
	existing, ok := h.ownsInvoice(c)
	if !ok {
		return
	}
	var inv models.Invoice
	if err := c.ShouldBindJSON(&inv); err != nil {
		c.JSON(http.StatusBadRequest, types.ErrorResp{Error: err.Error()})
		return
	}
	inv.ID = existing.ID
	updated, err := h.Store.UpdateInvoice(c.Request.Context(), inv)
	if err != nil {
		c.JSON(http.StatusInternalServerError, types.ErrorResp{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, updated)
}

func (h *Handler) HandleDeleteInvoice(c *gin.Context) {
	inv, ok := h.ownsInvoice(c)
	if !ok {
		return
	}
	if err := h.Store.DeleteInvoice(c.Request.Context(), inv.ID); err != nil {
		c.JSON(http.StatusInternalServerError, types.ErrorResp{Error: err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}
