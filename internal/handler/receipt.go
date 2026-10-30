package handler

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sofolitltd/barivara-server/internal/store"
	"github.com/sofolitltd/barivara-server/internal/types"
)

func (h *Handler) HandleSendReceipt(c *gin.Context) {
	var req types.SendReceiptReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, types.SendResp{Error: err.Error()})
		return
	}

	resp, err := h.sendReceipt(c.Request.Context(), req.RenterID, req.InvoiceID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, types.SendResp{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (h *Handler) sendReceipt(ctx context.Context, renterID, invoiceID string) (types.SendResp, error) {
	inv, err := h.Store.GetInvoice(ctx, invoiceID)
	if err != nil {
		return types.SendResp{}, fmt.Errorf("invoice not found: %w", err)
	}

	renter, err := h.Store.GetRenter(ctx, renterID)
	if err != nil {
		return types.SendResp{}, fmt.Errorf("renter not found: %w", err)
	}

	prop, err := h.Store.GetProperty(ctx, renter.PropertyID)
	if err != nil {
		return types.SendResp{}, fmt.Errorf("property not found: %w", err)
	}

	if !prop.SmsEnabled {
		return types.SendResp{}, fmt.Errorf("sms is disabled for this property")
	}
	if !renter.SmsEnabled {
		return types.SendResp{}, fmt.Errorf("sms is disabled for this renter")
	}
	if h.Sender == nil {
		return types.SendResp{}, fmt.Errorf("sms service is not configured")
	}

	if err := h.Store.ConsumeSMS(ctx, prop.OwnerID); err != nil {
		return types.SendResp{}, err
	}

	msg := fmt.Sprintf("Dear %s,\nThank you! Payment of %d BDT for %s at %s received.\nReceipt: %s/%s\nHave a great month!",
		renter.Name, inv.TotalAmount, inv.MonthYear, prop.Name, types.InvoiceBaseURL, inv.ID)

	if err := h.Sender.SendText(renter.Phone, msg); err != nil {
		h.Store.LogSMS(ctx, store.SMSLog{
			RenterID: renterID, InvoiceID: invoiceID, Type: "receipt",
			Phone: renter.Phone, Message: msg, Status: "failed",
			Error: err.Error(), SentAt: time.Now(),
		})
		return types.SendResp{}, err
	}

	h.Store.LogSMS(ctx, store.SMSLog{
		RenterID: renterID, InvoiceID: invoiceID, Type: "receipt",
		Phone: renter.Phone, Message: msg, Status: "sent", SentAt: time.Now(),
	})

	return types.SendResp{Success: true, SentTo: renter.Phone, Message: "Receipt sent"}, nil
}
