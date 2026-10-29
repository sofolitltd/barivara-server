package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/sofolitltd/barivara-server/internal/auth"
	"github.com/sofolitltd/barivara-server/internal/types"
)

// HandleAdminSignIn logs in against the separate admins table — entirely
// independent of the app users table and its phone-based signin.
func (h *Handler) HandleAdminSignIn(c *gin.Context) {
	var req types.AdminSignInReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, types.ErrorResp{Error: err.Error()})
		return
	}

	admin, err := h.Store.GetAdminByEmail(c.Request.Context(), req.Email)
	if err != nil {
		c.JSON(http.StatusUnauthorized, types.ErrorResp{Error: "ইমেইল অথবা পাসওয়ার্ড ভুল"})
		return
	}

	if !auth.CheckPassword(admin.PasswordHash, req.Password) {
		c.JSON(http.StatusUnauthorized, types.ErrorResp{Error: "ইমেইল অথবা পাসওয়ার্ড ভুল"})
		return
	}

	token, err := h.Tokens.Issue(admin.ID, "admin")
	if err != nil {
		c.JSON(http.StatusInternalServerError, types.ErrorResp{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, types.AdminAuthResp{
		Token: token,
		Admin: types.AdminInfo{ID: admin.ID, Name: admin.Name, Email: admin.Email},
	})
}
