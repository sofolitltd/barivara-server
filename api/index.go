package main

import (
	"context"
	"log"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/sofolitltd/barivara-server/internal/firebase"
	"github.com/sofolitltd/barivara-server/internal/handler"
	"github.com/sofolitltd/barivara-server/internal/sms"
)

var h http.Handler

func init() {
	apiKey := os.Getenv("BULKSMSBD_API_KEY")
	senderID := os.Getenv("BULKSMSBD_SENDER_ID")
	if apiKey == "" || senderID == "" {
		log.Fatal("BULKSMSBD_API_KEY and BULKSMSBD_SENDER_ID must be set")
	}

	fb, err := firebase.New(context.Background())
	if err != nil {
		log.Fatalf("Firebase init failed: %v", err)
	}

	sender := sms.New(apiKey, senderID)
	hdl := handler.New(fb, sender)

	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(corsMiddleware())

	r.GET("/", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	r.GET("/api/cron/check-reminders", hdl.HandleCronReminders)
	r.POST("/api/send-reminder", hdl.HandleSendReminder)
	r.POST("/api/send-receipt", hdl.HandleSendReceipt)

	h = r
}

func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

func Handler(w http.ResponseWriter, r *http.Request) {
	h.ServeHTTP(w, r)
}
