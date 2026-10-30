package server

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/sofolitltd/barivara-server/internal/handler"
	"github.com/sofolitltd/barivara-server/internal/middleware"
)

// newRouter registers every route once, shared by the local dev server
// (cmd/server) and the Vercel serverless entrypoint (api/index.go).
func newRouter(h *handler.Handler) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(corsMiddleware())

	r.GET("/", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "healthy"})
	})

	// --- Public ---
	r.POST("/api/auth/signup", h.HandleSignUp)
	r.POST("/api/auth/signin", h.HandleSignIn)
	r.POST("/api/admin/auth/signin", h.HandleAdminSignIn)
	r.GET("/api/invoices/:invoiceId", h.HandleGetInvoicePublic) // public share link
	r.GET("/api/users/:userId/public", h.HandlePublicUser)
	r.GET("/api/rent-posts", h.HandleListActiveRentPosts)
	r.GET("/api/rent-posts/:postId", h.HandleGetRentPost)
	r.GET("/api/units/:unitId/rent-post", h.HandleGetRentPostForUnit)
	r.GET("/api/sms/packages", h.HandleListSMSPackages)

	r.POST("/api/send-reminder", h.HandleSendReminder)
	r.POST("/api/send-receipt", h.HandleSendReceipt)
	r.GET("/api/cron/check-reminders", h.HandleCronReminders)

	auth := middleware.RequireAuth(h.Tokens)

	// --- Authenticated, any role ---
	authed := r.Group("/api", auth)
	authed.GET("/me", h.HandleMe)
	authed.PUT("/me", h.HandleUpdateProfile)
	authed.PUT("/me/password", h.HandleChangePassword)
	authed.POST("/auth/refresh", h.HandleRefreshToken)
	authed.POST("/users/fcm-token", h.HandleSaveFcmToken)
	authed.POST("/uploads/presign", h.HandleUploadPresign)
	authed.POST("/landlord-requests", h.HandleSubmitLandlordRequest)
	landlordWallet := r.Group("/api", auth, middleware.RequireRole("landlord"))
	landlordWallet.GET("/sms/wallet", h.HandleSMSWallet)
	landlordWallet.POST("/sms/purchases", h.HandleRequestSMSPackage)

	// --- Landlord ---
	landlord := r.Group("/api", auth, middleware.RequireRole("landlord"))
	landlord.POST("/properties", h.HandleCreateProperty)
	landlord.GET("/properties", h.HandleListProperties)
	landlord.GET("/properties/:propertyId", h.HandleGetProperty)
	landlord.GET("/properties/:propertyId/notices", h.HandleListPropertyNotices)
	landlord.POST("/properties/:propertyId/notices", h.HandleCreateNotice)
	landlord.PUT("/properties/:propertyId", h.HandleUpdateProperty)
	landlord.DELETE("/properties/:propertyId", h.HandleDeleteProperty)
	landlord.GET("/unpaid-invoices", h.HandleListUnpaidInvoicesForOwner)
	landlord.GET("/renters/:renterId", h.HandleGetRenterByID)
	landlord.POST("/renters/:renterId/conversation", h.HandleGetOrCreateConversation)
	landlord.GET("/conversations", h.HandleListLandlordConversations)

	landlord.POST("/properties/:propertyId/units", h.HandleCreateUnit)
	landlord.GET("/properties/:propertyId/units", h.HandleListUnits)
	landlord.GET("/properties/:propertyId/units/:unitId", h.HandleGetUnit)
	landlord.PUT("/properties/:propertyId/units/:unitId", h.HandleUpdateUnit)
	landlord.DELETE("/properties/:propertyId/units/:unitId", h.HandleDeleteUnit)

	units := landlord.Group("/properties/:propertyId/units/:unitId")
	units.POST("/renters", h.HandleCreateRenter)
	units.GET("/renters", h.HandleListActiveRenters)
	units.GET("/renters/history", h.HandleListRenterHistory)
	units.GET("/renters/:renterId", h.HandleGetRenter)
	units.PUT("/renters/:renterId", h.HandleUpdateRenter)
	units.POST("/renters/:renterId/schedule-rent", h.HandleScheduleRentChange)
	units.POST("/renters/:renterId/vacate", h.HandleVacateRenter)
	units.POST("/renters/:renterId/restore", h.HandleRestoreRenter)
	units.DELETE("/renters/:renterId", h.HandleDeleteRenter)

	units.POST("/renters/:renterId/join-code", h.HandleGenerateJoinCode)
	units.GET("/renters/:renterId/link-requests", h.HandleListPendingLinkRequests)
	units.POST("/renters/:renterId/link-requests/:requestId/approve", h.HandleApproveLinkRequest)
	units.POST("/renters/:renterId/link-requests/:requestId/reject", h.HandleRejectLinkRequest)
	units.POST("/renters/:renterId/link-by-email", h.HandleLinkTenantByEmail)
	units.POST("/renters/:renterId/link-by-phone", h.HandleLinkTenantByPhone)
	units.POST("/renters/:renterId/unlink", h.HandleUnlinkTenant)

	units.GET("/invoices", h.HandleListInvoicesByUnit)
	units.POST("/invoices", h.HandleCreateInvoice)
	units.POST("/rent-posts", h.HandleCreateRentPost)
	landlord.PUT("/rent-posts/:postId", h.HandleUpdateRentPost)
	landlord.DELETE("/rent-posts/:postId", h.HandleDeleteRentPost)
	landlord.GET("/properties/:propertyId/invoices", h.HandleListInvoicesByProperty)

	// Flat, invoice-id-only routes: the invoice already carries its own
	// propertyId, so these look it up and verify ownership from there
	// instead of requiring propertyId in the path (matches how the
	// Flutter client already had these calls shaped).
	landlord.PUT("/invoices/:invoiceId", h.HandleUpdateInvoice)
	landlord.DELETE("/invoices/:invoiceId", h.HandleDeleteInvoice)
	landlord.POST("/invoices/:invoiceId/payments", h.HandleRecordPayment)
	landlord.POST("/invoices/:invoiceId/mark-paid", h.HandleMarkInvoicePaid)
	landlord.POST("/invoices/:invoiceId/status", h.HandleUpdateInvoiceStatus)
	landlord.POST("/invoices/:invoiceId/clear-payments", h.HandleClearPayments)
	landlord.POST("/invoices/:invoiceId/remove-paid-item", h.HandleRemovePaidItem)

	// --- Tenant ---
	tenant := r.Group("/api/tenant", auth, middleware.RequireRole("renter"))
	tenant.POST("/link-requests", h.HandleRequestLinkByCode)
	tenant.GET("/link-requests/pending", h.HandleMyPendingLinkRequest)
	tenant.GET("/renter", h.HandleMyLinkedRenter)
	tenant.GET("/property", h.HandleMyProperty)
	tenant.GET("/unit", h.HandleMyUnit)
	tenant.GET("/invoices", h.HandleMyInvoices)
	tenant.GET("/conversation", h.HandleGetTenantConversation)
	tenant.GET("/notices", h.HandleListTenantNotices)
	tenant.POST("/notices/:noticeId/read", h.HandleMarkTenantNoticeRead)

	authed.GET("/conversations/:conversationId/messages", h.HandleListConversationMessages)
	authed.POST("/conversations/:conversationId/messages", h.HandleSendConversationMessage)

	// --- Admin ---
	admin := r.Group("/api/admin", auth, middleware.RequireRole("admin"))
	admin.GET("/stats", h.HandleAdminStats)
	admin.GET("/sms/balance", h.HandleAdminSMSBalance)
	admin.GET("/landlords", h.HandleListLandlords)
	admin.GET("/landlord-requests", h.HandleListPendingLandlordRequests)
	admin.GET("/landlord-requests/all", h.HandleListAllLandlordRequests)
	admin.POST("/landlord-requests/:requestId/approve", h.HandleApproveLandlordRequest)
	admin.POST("/landlord-requests/:requestId/reject", h.HandleRejectLandlordRequest)
	admin.POST("/users/:userId/plan", h.HandleSetUserPlan)
	admin.GET("/sms/packages", h.HandleAdminListSMSPackages)
	admin.POST("/sms/packages", h.HandleAdminCreateSMSPackage)
	admin.PUT("/sms/packages/:packageId", h.HandleAdminUpdateSMSPackage)
	admin.DELETE("/sms/packages/:packageId", h.HandleAdminDeleteSMSPackage)
	admin.GET("/sms/purchases", h.HandleAdminListSMSPurchases)
	admin.POST("/sms/purchases/:requestId/:status", h.HandleAdminProcessSMSPurchase)

	return r
}
