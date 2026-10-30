package server

import (
	"context"
	"log"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"

	appauth "github.com/sofolitltd/barivara-server/internal/auth"
	"github.com/sofolitltd/barivara-server/internal/db"
	"github.com/sofolitltd/barivara-server/internal/handler"
	"github.com/sofolitltd/barivara-server/internal/push"
	"github.com/sofolitltd/barivara-server/internal/sms"
	"github.com/sofolitltd/barivara-server/internal/storage"
	"github.com/sofolitltd/barivara-server/internal/store"
	"github.com/sofolitltd/barivara-server/internal/telegram"
)

func Run() {
	godotenv.Load()

	h, cleanup := build()
	defer cleanup()

	r := newRouter(h)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Starting server on :%s", port)
	if err := r.Run(":" + port); err != nil {
		log.Fatal(err)
	}
}

func firebaseServiceAccount() string {
	if value := os.Getenv("FIREBASE_SERVICE_ACCOUNT_JSON"); value != "" {
		return value
	}
	if path := os.Getenv("FIREBASE_CREDENTIALS_PATH"); path != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			log.Printf("FIREBASE_CREDENTIALS_PATH %q unavailable: %v — FCM push notifications disabled", path, err)
			return ""
		}
		return string(data)
	}
	return ""
}

// build wires the DB pool, object storage client, SMS sender, and JWT
// issuer into a Handler. Returns a cleanup func to close the pool.
func build() (*handler.Handler, func()) {
	ctx := context.Background()

	apiKey := os.Getenv("BULKSMSBD_API_KEY")
	senderID := os.Getenv("BULKSMSBD_SENDER_ID")
	sender := sms.New(apiKey, senderID)
	if sender == nil {
		log.Println("BULKSMSBD_API_KEY/BULKSMSBD_SENDER_ID not set — SMS delivery disabled")
	}

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		log.Fatal("DATABASE_URL must be set (run `neon env pull`)")
	}
	pool, err := db.New(ctx, databaseURL)
	if err != nil {
		log.Fatalf("postgres init failed: %v", err)
	}

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		log.Fatal("JWT_SECRET must be set")
	}

	// Must be a public_read bucket — uploaded images/documents get a
	// permanent public URL back, matching the old Cloudinary behavior.
	bucket := os.Getenv("OBJECT_STORAGE_BUCKET")
	if bucket == "" {
		bucket = "uploads"
	}
	storageClient, err := storage.New(ctx, bucket, os.Getenv("AWS_ENDPOINT_URL_S3"))
	if err != nil {
		log.Fatalf("object storage init failed: %v", err)
	}

	// Optional: admin landlord-request notifications. Logs a warning and
	// no-ops (rather than failing startup) when unset, since this isn't
	// load-bearing for the app to function.
	telegramNotifier := telegram.New(os.Getenv("TELEGRAM_BOT_TOKEN"), os.Getenv("TELEGRAM_ADMIN_CHAT_ID"))
	if telegramNotifier == nil {
		log.Println("TELEGRAM_BOT_TOKEN/TELEGRAM_ADMIN_CHAT_ID not set — admin Telegram notifications disabled")
	}

	// Optional: push notifications to the Android app (e.g. landlord
	// approval). Logs a warning and no-ops when unset, same as Telegram.
	serviceAccountJSON := firebaseServiceAccount()
	pushNotifier, err := push.New(ctx, serviceAccountJSON)
	if err != nil {
		log.Fatalf("firebase push init failed: %v", err)
	}
	if pushNotifier == nil {
		log.Println("FIREBASE_SERVICE_ACCOUNT_JSON/FIREBASE_CREDENTIALS_PATH not set — FCM push notifications disabled")
	}

	st := store.New(pool)
	tokens := appauth.NewTokenIssuer(jwtSecret)
	h := handler.New(st, sender, tokens, storageClient, telegramNotifier, pushNotifier)

	return h, func() { pool.Close() }
}

func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}
