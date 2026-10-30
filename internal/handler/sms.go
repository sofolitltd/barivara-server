package handler

import (
	"context"
	"fmt"
	"html"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sofolitltd/barivara-server/internal/middleware"
	"github.com/sofolitltd/barivara-server/internal/models"
)

const lowSMSBalanceThreshold = 100

type adminSMSBalanceResponse struct {
	Balance      float64 `json:"balance"`
	ExpiryDate   string  `json:"expiryDate,omitempty"`
	Status       string  `json:"status"`
	Message      string  `json:"message"`
	ProviderCode int     `json:"providerCode,omitempty"`
}

func (h *Handler) HandleListSMSPackages(c *gin.Context) {
	packages, err := h.Store.ListSMSPackages(c.Request.Context(), true)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, packages)
}

func (h *Handler) HandleAdminSMSBalance(c *gin.Context) {
	balance, err := h.Sender.GetBalance()
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}

	status := "healthy"
	message := fmt.Sprintf("উপলভ্য ব্যালেন্স: ৳%.2f", balance.Balance)
	if balance.Balance < float64(lowSMSBalanceThreshold) {
		status = "low"
		message = fmt.Sprintf("ব্যালেন্স মাত্র ৳%.2f। দ্রুত নতুন প্যাকেজ কিনুন।", balance.Balance)
	}
	if balance.ExpiryDate != "" {
		if expiry, ok := parseProviderExpiry(balance.ExpiryDate); ok && !expiry.IsZero() && !expiry.After(time.Now()) {
			status = "expired"
			message = "SMS অ্যাকাউন্টের বৈধতা শেষ হয়ে গেছে।"
		} else if status == "healthy" {
			status = "healthy"
			message = fmt.Sprintf("SMS অ্যাকাউন্ট বৈধ, ব্যালেন্স ৳%.2f।", balance.Balance)
		}
	}
	c.JSON(http.StatusOK, adminSMSBalanceResponse{
		Balance: balance.Balance, ExpiryDate: balance.ExpiryDate, Status: status,
		Message: message, ProviderCode: balance.ProviderCode,
	})
}

func parseProviderExpiry(value string) (time.Time, bool) {
	value = strings.TrimSpace(value)
	layouts := []string{"2006-01-02", "02-01-2006", "02/01/2006", "2006-01-02 15:04:05", time.RFC3339}
	for _, layout := range layouts {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed, true
		}
	}
	return time.Time{}, false
}

func (h *Handler) HandleSMSWallet(c *gin.Context) {
	userID := middleware.UserID(c)
	balance, err := h.Store.SMSBalance(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	requests, err := h.Store.ListSMSPurchaseRequests(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"balance": balance, "requests": requests})
}

func (h *Handler) HandleRequestSMSPackage(c *gin.Context) {
	var body struct {
		PackageID string `json:"packageId" binding:"required"`
		Note      string `json:"note"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	request, err := h.Store.RequestSMSPackage(c.Request.Context(), middleware.UserID(c), body.PackageID, body.Note)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	go func() {
		user, userErr := h.Store.GetUserByID(context.Background(), request.LandlordID)
		name, phone := "Unknown", "—"
		if userErr == nil {
			name, phone = user.Name, user.Phone
		}
		note := request.Note
		if note == "" {
			note = "—"
		}
		text := fmt.Sprintf(
			"🛒 <b>New SMS package request</b>\n\nLandlord: %s\nPhone: %s\nPackage: %s\nMessages: %d\nAmount: ৳%d\nPayment note: %s\n\nReview in the admin panel.",
			html.EscapeString(name), html.EscapeString(phone), html.EscapeString(request.PackageName),
			request.MessageCount, request.Amount, html.EscapeString(note),
		)
		if err := h.Telegram.SendMessage(text); err != nil {
			log.Printf("telegram notify (SMS purchase %s): %v", request.ID, err)
		}
	}()
	c.JSON(http.StatusCreated, request)
}

func (h *Handler) HandleAdminListSMSPackages(c *gin.Context) {
	packages, err := h.Store.ListSMSPackages(c.Request.Context(), false)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, packages)
}

func (h *Handler) HandleAdminCreateSMSPackage(c *gin.Context) {
	var p models.SMSPackage
	if err := c.ShouldBindJSON(&p); err != nil || p.MessageCount <= 0 || p.Price < 0 || p.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "package name, message count and valid price are required"})
		return
	}
	p.IsActive = true
	created, err := h.Store.CreateSMSPackage(c.Request.Context(), p)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, created)
}

func (h *Handler) HandleAdminUpdateSMSPackage(c *gin.Context) {
	var p models.SMSPackage
	if err := c.ShouldBindJSON(&p); err != nil || p.MessageCount <= 0 || p.Price < 0 || p.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid package"})
		return
	}
	p.ID = c.Param("packageId")
	updated, err := h.Store.UpdateSMSPackage(c.Request.Context(), p)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, updated)
}

func (h *Handler) HandleAdminDeleteSMSPackage(c *gin.Context) {
	if err := h.Store.DeleteSMSPackage(c.Request.Context(), c.Param("packageId")); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) HandleAdminListSMSPurchases(c *gin.Context) {
	requests, err := h.Store.ListSMSPurchaseRequests(c.Request.Context(), "")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, requests)
}

func (h *Handler) HandleAdminProcessSMSPurchase(c *gin.Context) {
	status := c.Param("status")
	if status != "approved" && status != "rejected" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid status"})
		return
	}
	request, err := h.Store.ProcessSMSPurchase(c.Request.Context(), c.Param("requestId"), status)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	go func() {
		ctx := context.Background()
		token, err := h.Store.GetFcmToken(ctx, request.LandlordID)
		if err != nil {
			log.Printf("push notify (SMS purchase %s): fetch fcm token: %v", request.ID, err)
			return
		}
		var title, body string
		if status == "approved" {
			title = "এসএমএস প্যাকেজ অনুমোদিত হয়েছে!"
			body = fmt.Sprintf(
				"%s প্যাকেজের %dটি মেসেজ আপনার অ্যাকাউন্টে যোগ হয়েছে।",
				request.PackageName, request.MessageCount,
			)
		} else {
			title = "এসএমএস প্যাকেজ অনুরোধ প্রত্যাখ্যাত হয়েছে"
			body = "আপনার পেমেন্ট অনুরোধ প্রত্যাখ্যাত হয়েছে। বিস্তারিত জানতে অ্যাডমিনের সাথে যোগাযোগ করুন।"
		}
		if err := h.Push.Send(
			ctx,
			token,
			title,
			body,
			map[string]string{"type": "sms_purchase_" + status, "requestId": request.ID},
		); err != nil {
			log.Printf("push notify (SMS purchase %s): %v", request.ID, err)
		}
	}()
	c.JSON(http.StatusOK, request)
}
