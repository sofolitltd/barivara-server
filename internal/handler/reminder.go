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

func (h *Handler) HandleSendReminder(c *gin.Context) {
	var req types.SendReminderReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, types.SendResp{Error: err.Error()})
		return
	}

	resp, err := h.sendReminder(c.Request.Context(), req.RenterID, req.InvoiceID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, types.SendResp{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (h *Handler) sendReminder(ctx context.Context, renterID, invoiceID string) (types.SendResp, error) {
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
	if h.Sender == nil {
		return types.SendResp{}, fmt.Errorf("sms service is not configured")
	}

	if err := h.Store.ConsumeSMS(ctx, prop.OwnerID); err != nil {
		return types.SendResp{}, err
	}

	msg := fmt.Sprintf("Dear %s,\nYour rent of %d BDT for %s at %s is due.\nView invoice: %s/%s\nPlease pay on time. Thank you.",
		renter.Name, inv.TotalAmount, inv.MonthYear, prop.Name, types.InvoiceBaseURL, inv.ID)

	if err := h.Sender.SendText(renter.Phone, msg); err != nil {
		h.Store.LogSMS(ctx, store.SMSLog{
			RenterID: renterID, InvoiceID: invoiceID, Type: "reminder",
			Phone: renter.Phone, Message: msg, Status: "failed",
			Error: err.Error(), SentAt: time.Now(),
		})
		return types.SendResp{}, err
	}

	h.Store.LogSMS(ctx, store.SMSLog{
		RenterID: renterID, InvoiceID: invoiceID, Type: "reminder",
		Phone: renter.Phone, Message: msg, Status: "sent", SentAt: time.Now(),
	})

	return types.SendResp{Success: true, SentTo: renter.Phone, Message: "Reminder sent"}, nil
}
