package handler

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sofolitltd/barivara-server/internal/firebase"
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
	inv, err := h.FB.GetInvoice(ctx, invoiceID)
	if err != nil {
		return types.SendResp{}, fmt.Errorf("invoice not found: %w", err)
	}

	renter, err := h.FB.GetRenter(ctx, renterID)
	if err != nil {
		return types.SendResp{}, fmt.Errorf("renter not found: %w", err)
	}

	prop, err := h.FB.GetProperty(ctx, renter.PropertyID)
	if err != nil {
		return types.SendResp{}, fmt.Errorf("property not found: %w", err)
	}

	if !prop.SmsEnabled {
		return types.SendResp{}, fmt.Errorf("sms is disabled for this property")
	}

	plan, err := h.FB.GetUserPlan(ctx, prop.OwnerID)
	if err != nil {
		return types.SendResp{}, fmt.Errorf("cannot verify owner plan: %w", err)
	}
	if plan != "pro" {
		return types.SendResp{}, fmt.Errorf("owner plan is %s (pro required)", plan)
	}

	msg := fmt.Sprintf("Dear %s,\nThank you! Payment of %d BDT for %s at %s received.\nReceipt: %s/%s\nHave a great month!",
		renter.Name, inv.TotalAmount, inv.MonthYear, prop.Name, types.InvoiceBaseURL, inv.ID)

	if err := h.Sender.SendText(renter.Phone, msg); err != nil {
		h.FB.LogSMS(ctx, firebase.SMSLog{
			RenterID: renterID, InvoiceID: invoiceID, Type: "receipt",
			Phone: renter.Phone, Message: msg, Status: "failed",
			Error: err.Error(), SentAt: time.Now(),
		})
		return types.SendResp{}, err
	}

	h.FB.LogSMS(ctx, firebase.SMSLog{
		RenterID: renterID, InvoiceID: invoiceID, Type: "receipt",
		Phone: renter.Phone, Message: msg, Status: "sent", SentAt: time.Now(),
	})

	return types.SendResp{Success: true, SentTo: renter.Phone, Message: "Receipt sent"}, nil
}
