package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/sofolitltd/barivara-server/internal/auth"
	"github.com/sofolitltd/barivara-server/internal/middleware"
	"github.com/sofolitltd/barivara-server/internal/store"
	"github.com/sofolitltd/barivara-server/internal/types"
)

func (h *Handler) HandleSignUp(c *gin.Context) {
	var req types.SignUpReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, types.ErrorResp{Error: err.Error()})
		return
	}

	role := "renter"
	status := "approved" // self-serve tenant signup needs no admin review
	if req.Role == "landlord" {
		// Landlord accounts only become real landlords through the
		// /api/landlord-requests approval flow; signing up alone does not
		// grant landlord access.
		role = "pending_landlord"
		status = "pending"
	}

	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, types.ErrorResp{Error: err.Error()})
		return
	}

	user, err := h.Store.CreateUser(c.Request.Context(), req.Name, req.Phone, req.Email, hash, role, status)
	if err != nil {
		if errors.Is(err, store.ErrDuplicate) {
			c.JSON(http.StatusConflict, types.ErrorResp{Error: "এই ফোন নম্বর দিয়ে অ্যাকাউন্ট আগে থেকেই আছে"})
			return
		}
		c.JSON(http.StatusInternalServerError, types.ErrorResp{Error: err.Error()})
		return
	}

	token, err := h.Tokens.Issue(user.ID, user.Role)
	if err != nil {
		c.JSON(http.StatusInternalServerError, types.ErrorResp{Error: err.Error()})
		return
	}

	c.JSON(http.StatusCreated, types.AuthResp{Token: token, User: *user})
}

func (h *Handler) HandleSignIn(c *gin.Context) {
	var req types.SignInReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, types.ErrorResp{Error: err.Error()})
		return
	}

	userWithPass, err := h.Store.GetUserByPhone(c.Request.Context(), req.Phone)
	if err != nil {
		c.JSON(http.StatusUnauthorized, types.ErrorResp{Error: "ফোন নম্বর অথবা পাসওয়ার্ড ভুল"})
		return
	}

	if !auth.CheckPassword(userWithPass.PasswordHash, req.Password) {
		c.JSON(http.StatusUnauthorized, types.ErrorResp{Error: "ফোন নম্বর অথবা পাসওয়ার্ড ভুল"})
		return
	}

	token, err := h.Tokens.Issue(userWithPass.ID, userWithPass.Role)
	if err != nil {
		c.JSON(http.StatusInternalServerError, types.ErrorResp{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, types.AuthResp{Token: token, User: userWithPass.User})
}

// HandlePublicUser returns the narrow name/phone slice of a user needed
// for a public rent listing's landlord contact card — never email, role,
// or status.
func (h *Handler) HandlePublicUser(c *gin.Context) {
	user, err := h.Store.GetUserByID(c.Request.Context(), c.Param("userId"))
	if err != nil {
		c.JSON(http.StatusNotFound, types.ErrorResp{Error: "user not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"name": user.Name, "phone": user.Phone})
}

func (h *Handler) HandleMe(c *gin.Context) {
	user, err := h.Store.GetUserByID(c.Request.Context(), middleware.UserID(c))
	if err != nil {
		c.JSON(http.StatusNotFound, types.ErrorResp{Error: "user not found"})
		return
	}
	c.JSON(http.StatusOK, user)
}

// HandleRefreshToken re-issues a JWT off the user's *current* DB role —
// needed because the role is baked into the token at sign-in and the
// auth middleware checks it with no DB hit, so a role change (e.g. an
// admin approving a pending_landlord) only takes effect for a client
// once it swaps its stale token for a fresh one via this endpoint.
func (h *Handler) HandleRefreshToken(c *gin.Context) {
	user, err := h.Store.GetUserByID(c.Request.Context(), middleware.UserID(c))
	if err != nil {
		c.JSON(http.StatusNotFound, types.ErrorResp{Error: "user not found"})
		return
	}

	token, err := h.Tokens.Issue(user.ID, user.Role)
	if err != nil {
		c.JSON(http.StatusInternalServerError, types.ErrorResp{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, types.AuthResp{Token: token, User: *user})
}

type SaveFcmTokenReq struct {
	Token string `json:"token" binding:"required"`
}

func (h *Handler) HandleSaveFcmToken(c *gin.Context) {
	var req SaveFcmTokenReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, types.ErrorResp{Error: err.Error()})
		return
	}
	if err := h.Store.UpdateFcmToken(c.Request.Context(), middleware.UserID(c), req.Token); err != nil {
		c.JSON(http.StatusInternalServerError, types.ErrorResp{Error: err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}
