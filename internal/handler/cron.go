package handler

import (
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sofolitltd/barivara-server/internal/firebase"
	"github.com/sofolitltd/barivara-server/internal/types"
)

func (h *Handler) HandleCronReminders(c *gin.Context) {
	ctx := c.Request.Context()
	today := time.Now().Day()

	props, err := h.FB.QueryPropertiesByReminderDay(ctx, today)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if len(props) == 0 {
		c.JSON(http.StatusOK, types.CronResp{Skipped: true, Reason: "no properties with reminder day today"})
		return
	}

	resp := types.CronResp{}
	for _, prop := range props {
		renters, err := h.FB.QueryActiveRentersByProperty(ctx, prop.ID)
		if err != nil {
			log.Printf("skip property %s: %v", prop.ID, err)
			continue
		}

		for _, renter := range renters {
			if renter.Phone == "" {
				resp.Failed++
				continue
			}

			invoices, err := h.FB.QueryUnpaidInvoicesByRenter(ctx, renter.ID)
			if err != nil {
				log.Printf("skip renter %s: %v", renter.ID, err)
				resp.Failed++
				continue
			}

			if len(invoices) == 0 {
				continue
			}

			totalDue := 0
			for _, inv := range invoices {
				totalDue += inv.TotalAmount
			}

			msg := buildReminderMsg(renter.Name, prop.Name, invoices, totalDue)

			if err := h.Sender.SendText(renter.Phone, msg); err != nil {
				log.Printf("SMS failed for %s: %v", renter.Phone, err)
				h.FB.LogSMS(ctx, firebase.SMSLog{
					RenterID: renter.ID, Type: "reminder", Phone: renter.Phone,
					Message: msg, Status: "failed", Error: err.Error(), SentAt: time.Now(),
				})
				resp.Failed++
			} else {
				resp.Sent++
				h.FB.LogSMS(ctx, firebase.SMSLog{
					RenterID: renter.ID, Type: "reminder", Phone: renter.Phone,
					Message: msg, Status: "sent", SentAt: time.Now(),
				})
			}
		}
	}
	resp.Total = resp.Sent + resp.Failed

	c.JSON(http.StatusOK, resp)
}

func buildReminderMsg(name, propName string, invoices []firebase.Invoice, totalDue int) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("Dear %s,\n", name))

	if len(invoices) == 1 {
		inv := invoices[0]
		b.WriteString(fmt.Sprintf("Your rent of %d BDT for %s at %s is due.\n", inv.TotalAmount, inv.MonthYear, propName))
		b.WriteString(fmt.Sprintf("View invoice: %s/%s\n", types.InvoiceBaseURL, inv.ID))
	} else {
		b.WriteString(fmt.Sprintf("Your total rent of %d BDT for %d months at %s is due.\n", totalDue, len(invoices), propName))
		for _, inv := range invoices {
			b.WriteString(fmt.Sprintf("  - %s (%d BDT) - %s/%s\n", inv.MonthYear, inv.TotalAmount, types.InvoiceBaseURL, inv.ID))
		}
	}
	b.WriteString("Please pay on time. Thank you.")
	return b.String()
}
