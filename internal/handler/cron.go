package handler

import (
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

func (h *Handler) HandleCronReminders(c *gin.Context) {
	ctx := c.Request.Context()
	bangladesh, err := time.LoadLocation("Asia/Dhaka")
	if err != nil {
		bangladesh = time.FixedZone("Bangladesh", 6*60*60)
	}
	now := time.Now().In(bangladesh)
	today := now.Day()

	if today == 1 {
		monthYear := now.Format("2006-01")
		bills, err := h.Store.GenerateMonthlyBills(ctx, monthYear)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		log.Printf("monthly bills: created=%d skipped=%d failed=%d", bills.Created, bills.Skipped, bills.Failed)
	}

	props, err := h.Store.QueryPropertiesByReminderDay(ctx, today)
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
		// Bill generation normally runs on the 1st. If that cron was missed,
		// the property's configured reminder day safely retries current-month
		// invoices; existing unit/month rows are skipped by the unique index.
		if today != 1 {
			bills, err := h.Store.GenerateMonthlyBills(ctx, now.Format("2006-01"), prop.ID)
			if err != nil {
				log.Printf("monthly bills retry for property %s: %v", prop.ID, err)
			} else {
				log.Printf("monthly bills retry for property %s: created=%d skipped=%d failed=%d", prop.ID, bills.Created, bills.Skipped, bills.Failed)
			}
		}
		if !prop.SmsEnabled {
			log.Printf("skip property %s: sms disabled", prop.ID)
			continue
		}
		if h.Sender == nil {
			log.Printf("skip property %s: SMS service is not configured", prop.ID)
			continue
		}

		balance, err := h.Store.SMSBalance(ctx, prop.OwnerID)
		if err != nil {
			log.Printf("skip property %s: cannot check owner plan: %v", prop.ID, err)
			resp.Failed++
			continue
		}
		if balance <= 0 {
			log.Printf("skip property %s: SMS balance is empty", prop.ID)
			continue
		}

		renters, err := h.Store.ListActiveRentersByProperty(ctx, prop.ID)
		if err != nil {
			log.Printf("skip property %s: %v", prop.ID, err)
			continue
		}

		for _, renter := range renters {
			if renter.Phone == "" {
				resp.Failed++
				continue
			}

			invoices, err := h.Store.ListUnpaidInvoicesByRenter(ctx, renter.ID)
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
			if err := h.Store.ConsumeSMS(ctx, prop.OwnerID); err != nil {
				log.Printf("SMS balance exhausted for %s: %v", prop.OwnerID, err)
				break
			}

			if err := h.Sender.SendText(renter.Phone, msg); err != nil {
				log.Printf("SMS failed for %s: %v", renter.Phone, err)
				h.Store.LogSMS(ctx, store.SMSLog{
					RenterID: renter.ID, Type: "reminder", Phone: renter.Phone,
					Message: msg, Status: "failed", Error: err.Error(), SentAt: time.Now(),
				})
				resp.Failed++
			} else {
				resp.Sent++
				h.Store.LogSMS(ctx, store.SMSLog{
					RenterID: renter.ID, Type: "reminder", Phone: renter.Phone,
					Message: msg, Status: "sent", SentAt: time.Now(),
				})
			}
		}
	}
	resp.Total = resp.Sent + resp.Failed

	c.JSON(http.StatusOK, resp)
}

func buildReminderMsg(name, propName string, invoices []models.Invoice, totalDue int) string {
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
