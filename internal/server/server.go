package server

import (
	"context"
	"log"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/sofolitltd/barivara-server/internal/firebase"
	"github.com/sofolitltd/barivara-server/internal/handler"
	"github.com/sofolitltd/barivara-server/internal/sms"
)

func Run() {
	godotenv.Load()

	apiKey := os.Getenv("BULKSMSBD_API_KEY")
	senderID := os.Getenv("BULKSMSBD_SENDER_ID")
	if apiKey == "" || senderID == "" {
		log.Fatal("BULKSMSBD_API_KEY and BULKSMSBD_SENDER_ID must be set")
	}

	fb, err := firebase.New(context.Background())
	if err != nil {
		log.Fatalf("Firebase init failed: %v", err)
	}
	defer fb.Close()

	sender := sms.New(apiKey, senderID)
	h := handler.New(fb, sender)

	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(corsMiddleware())

	r.GET("/", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	r.GET("/api/cron/check-reminders", h.HandleCronReminders)
	r.POST("/api/send-reminder", h.HandleSendReminder)
	r.POST("/api/send-receipt", h.HandleSendReceipt)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("Starting server on :%s", port)
	if err := r.Run(":" + port); err != nil {
		log.Fatal(err)
	}
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
