package handler

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sofolitltd/barivara-server/internal/models"
	"github.com/sofolitltd/barivara-server/internal/store"
	"github.com/sofolitltd/barivara-server/internal/types"
)

func (h *Handler) HandleSendRenterReminder(c *gin.Context) {
	ctx := c.Request.Context()
	renter, err := h.Store.GetRenter(ctx, c.Param("renterId"))
	if err != nil {
		c.JSON(http.StatusNotFound, types.SendResp{Error: "renter not found"})
		return
	}
	if _, ok := h.ownsProperty(c, renter.PropertyID); !ok {
		return
	}
	if !renter.IsActive {
		c.JSON(http.StatusBadRequest, types.SendResp{Error: "renter is not active"})
		return
	}
	if !renter.SmsEnabled {
		c.JSON(http.StatusBadRequest, types.SendResp{Error: "sms is disabled for this renter"})
		return
	}

	invoices, err := h.Store.ListUnpaidInvoicesByRenter(ctx, renter.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, types.SendResp{Error: err.Error()})
		return
	}
	outstanding := make([]models.Invoice, 0, len(invoices))
	totalDue := 0
	for _, invoice := range invoices {
		remaining := invoice.TotalAmount
		for _, payment := range invoice.Payments {
			remaining -= payment.Amount
		}
		if remaining <= 0 {
			continue
		}
		invoice.TotalAmount = remaining
		outstanding = append(outstanding, invoice)
		totalDue += remaining
	}
	if len(outstanding) == 0 {
		c.JSON(http.StatusBadRequest, types.SendResp{Error: "no outstanding bills for this renter"})
		return
	}

	property, err := h.Store.GetProperty(ctx, renter.PropertyID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, types.SendResp{Error: err.Error()})
		return
	}
	if !property.SmsEnabled {
		c.JSON(http.StatusBadRequest, types.SendResp{Error: "sms is disabled for this property"})
		return
	}
	if h.Sender == nil {
		c.JSON(http.StatusServiceUnavailable, types.SendResp{Error: "sms service is not configured"})
		return
	}
	if renter.Phone == "" {
		c.JSON(http.StatusBadRequest, types.SendResp{Error: "renter has no phone number"})
		return
	}
	if err := h.Store.ConsumeSMS(ctx, property.OwnerID); err != nil {
		c.JSON(http.StatusPaymentRequired, types.SendResp{Error: err.Error()})
		return
	}

	var message strings.Builder
	fmt.Fprintf(&message, "Dear %s,\nYour total outstanding rent at %s is %d BDT.\n",
		renter.Name, property.Name, totalDue)
	for _, invoice := range outstanding {
		fmt.Fprintf(&message, "- %s: %d BDT - %s/%s\n",
			invoice.MonthYear, invoice.TotalAmount, types.InvoiceBaseURL, invoice.ID)
	}
	message.WriteString("Please pay when convenient. Thank you.")

	text := message.String()
	if err := h.Sender.SendText(renter.Phone, text); err != nil {
		if logErr := h.Store.LogSMS(ctx, store.SMSLog{
			RenterID: renter.ID, InvoiceID: outstanding[0].ID, Type: "reminder",
			Phone: renter.Phone, Message: text, Status: "failed",
			Error: err.Error(), SentAt: time.Now(),
		}); logErr != nil {
			log.Printf("log failed renter reminder for %s: %v", renter.ID, logErr)
		}
		c.JSON(http.StatusBadGateway, types.SendResp{Error: err.Error()})
		return
	}
	if err := h.Store.LogSMS(ctx, store.SMSLog{
		RenterID: renter.ID, InvoiceID: outstanding[0].ID, Type: "reminder",
		Phone: renter.Phone, Message: text, Status: "sent", SentAt: time.Now(),
	}); err != nil {
		log.Printf("reminder sent to renter %s but failed to save SMS log: %v", renter.ID, err)
	}

	c.JSON(http.StatusOK, types.SendResp{
		Success: true, SentTo: renter.Phone, Message: "Reminder sent",
	})
}

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
	if !renter.SmsEnabled {
		return types.SendResp{}, fmt.Errorf("sms is disabled for this renter")
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
