package handler

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
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

	props, err := h.Store.QueryPropertiesByReminderSchedule(ctx, now.Day(), now.Hour())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if len(props) == 0 {
		c.JSON(http.StatusOK, types.CronResp{
			Skipped: true, Reason: "no properties due for monthly billing at this time",
		})
		return
	}

	resp := types.CronResp{}
	monthYear := now.Format("January 2006")
	for _, prop := range props {
		bills, err := h.Store.GenerateMonthlyBills(ctx, monthYear, prop.ID)
		if err != nil {
			log.Printf("monthly bills for property %s: %v", prop.ID, err)
			resp.Failed++
			continue
		}
		log.Printf("monthly bills for property %s: created=%d skipped=%d failed=%d",
			prop.ID, bills.Created, bills.Skipped, bills.Failed)
		resp.Failed += bills.Failed

		for _, notice := range bills.Notifications {
			pushErr := error(nil)
			if notice.FcmToken != "" {
				if h.Push == nil {
					pushErr = fmt.Errorf("FCM push service is not configured")
				} else {
					pushErr = h.Push.Send(ctx, notice.FcmToken, "নতুন মাসিক বিল",
						fmt.Sprintf("%s মাসের ভাড়া বিল ৳%d তৈরি হয়েছে।",
							notice.MonthYear, notice.TotalAmount),
						map[string]string{
							"type":      "invoice_generated",
							"invoiceId": notice.InvoiceID,
						})
				}
				if pushErr == nil {
					resp.Sent++
					continue
				}
				log.Printf("invoice push failed for renter %s: %v", notice.RenterID, pushErr)
			}

			if !shouldSendSMSFallback(notice.FcmToken != "", pushErr, prop.SmsEnabled, notice.SmsEnabled) {
				if pushErr != nil {
					resp.Failed++
				}
				continue
			}
			if notice.Phone == "" {
				log.Printf("skip invoice SMS for renter %s: phone is empty", notice.RenterID)
				resp.Failed++
				continue
			}
			if h.Sender == nil {
				log.Printf("skip invoice SMS for renter %s: SMS service is not configured", notice.RenterID)
				resp.Failed++
				continue
			}
			if err := h.Store.ConsumeSMS(ctx, prop.OwnerID); err != nil {
				log.Printf("invoice SMS balance unavailable for renter %s: %v", notice.RenterID, err)
				resp.Failed++
				continue
			}

			message := buildBillNotificationMsg(notice.RenterName, prop.Name,
				notice.MonthYear, notice.TotalAmount, notice.InvoiceID)
			if err := h.Sender.SendText(notice.Phone, message); err != nil {
				log.Printf("invoice SMS failed for renter %s: %v", notice.RenterID, err)
				if logErr := h.Store.LogSMS(ctx, store.SMSLog{
					RenterID: notice.RenterID, InvoiceID: notice.InvoiceID, Type: "invoice",
					Phone: notice.Phone, Message: message, Status: "failed",
					Error: err.Error(), SentAt: time.Now(),
				}); logErr != nil {
					log.Printf("failed to log invoice SMS failure for renter %s: %v", notice.RenterID, logErr)
				}
				resp.Failed++
				continue
			}
			if err := h.Store.LogSMS(ctx, store.SMSLog{
				RenterID: notice.RenterID, InvoiceID: notice.InvoiceID, Type: "invoice",
				Phone: notice.Phone, Message: message, Status: "sent", SentAt: time.Now(),
			}); err != nil {
				log.Printf("invoice SMS sent to renter %s but failed to save SMS log: %v", notice.RenterID, err)
			}
			resp.Sent++
		}
	}
	resp.Total = resp.Sent + resp.Failed
	c.JSON(http.StatusOK, resp)
}

func shouldSendSMSFallback(hasPushToken bool, pushErr error, propertySmsEnabled, renterSmsEnabled bool) bool {
	return propertySmsEnabled && renterSmsEnabled && (!hasPushToken || pushErr != nil)
}

func buildBillNotificationMsg(name, propertyName, monthYear string, amount int, invoiceID string) string {
	return fmt.Sprintf(
		"Dear %s, your rent bill of %d BDT for %s at %s has been generated. View bill: %s/%s",
		name, amount, monthYear, propertyName, types.InvoiceBaseURL, invoiceID,
	)
}
