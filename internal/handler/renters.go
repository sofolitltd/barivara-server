package handler

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sofolitltd/barivara-server/internal/models"
	"github.com/sofolitltd/barivara-server/internal/types"
)

func (h *Handler) HandleCreateRenter(c *gin.Context) {
	propertyID, unitID := c.Param("propertyId"), c.Param("unitId")
	if _, ok := h.ownsProperty(c, propertyID); !ok {
		return
	}
	var r models.Renter
	if err := c.ShouldBindJSON(&r); err != nil {
		c.JSON(http.StatusBadRequest, types.ErrorResp{Error: err.Error()})
		return
	}
	joinCode, err := randomCode()
	if err != nil {
		c.JSON(http.StatusInternalServerError, types.ErrorResp{Error: "could not generate join code"})
		return
	}
	r.JoinCode = &joinCode
	r.PropertyID, r.UnitID = propertyID, unitID
	created, err := h.Store.CreateRenter(c.Request.Context(), r)
	if err != nil {
		c.JSON(http.StatusInternalServerError, types.ErrorResp{Error: err.Error()})
		return
	}
	c.JSON(http.StatusCreated, created)
}

func (h *Handler) HandleListActiveRenters(c *gin.Context) {
	propertyID, unitID := c.Param("propertyId"), c.Param("unitId")
	if _, ok := h.ownsProperty(c, propertyID); !ok {
		return
	}
	renters, err := h.Store.ListActiveRentersByUnit(c.Request.Context(), propertyID, unitID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, types.ErrorResp{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, renters)
}

func (h *Handler) HandleListRenterHistory(c *gin.Context) {
	propertyID, unitID := c.Param("propertyId"), c.Param("unitId")
	if _, ok := h.ownsProperty(c, propertyID); !ok {
		return
	}
	renters, err := h.Store.ListRenterHistory(c.Request.Context(), propertyID, unitID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, types.ErrorResp{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, renters)
}

// HandleGetRenterByID looks the renter up directly by id (no propertyId in
// the path) and verifies the requester owns the property it belongs to —
// used by UI flows that only have a renterId in hand.
func (h *Handler) HandleGetRenterByID(c *gin.Context) {
	renter, err := h.Store.GetRenter(c.Request.Context(), c.Param("renterId"))
	if err != nil {
		c.JSON(http.StatusNotFound, types.ErrorResp{Error: "renter not found"})
		return
	}
	if _, ok := h.ownsProperty(c, renter.PropertyID); !ok {
		return
	}
	c.JSON(http.StatusOK, renter)
}

func (h *Handler) HandleGetRenter(c *gin.Context) {
	if _, ok := h.ownsProperty(c, c.Param("propertyId")); !ok {
		return
	}
	renter, err := h.Store.GetRenter(c.Request.Context(), c.Param("renterId"))
	if err != nil {
		c.JSON(http.StatusNotFound, types.ErrorResp{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, renter)
}

func (h *Handler) HandleUpdateRenter(c *gin.Context) {
	if _, ok := h.ownsProperty(c, c.Param("propertyId")); !ok {
		return
	}
	var r models.Renter
	if err := c.ShouldBindJSON(&r); err != nil {
		c.JSON(http.StatusBadRequest, types.ErrorResp{Error: err.Error()})
		return
	}
	r.ID = c.Param("renterId")
	updated, err := h.Store.UpdateRenter(c.Request.Context(), r)
	if err != nil {
		c.JSON(http.StatusInternalServerError, types.ErrorResp{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, updated)
}

func (h *Handler) HandleScheduleRentChange(c *gin.Context) {
	if _, ok := h.ownsProperty(c, c.Param("propertyId")); !ok {
		return
	}
	var req struct {
		Amount    int       `json:"amount" binding:"required"`
		StartDate time.Time `json:"startDate" binding:"required"`
		Reason    *string   `json:"reason"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, types.ErrorResp{Error: err.Error()})
		return
	}
	renterID := c.Param("renterId")
	updated, err := h.Store.ScheduleRentChange(c.Request.Context(), renterID, req.Amount, req.StartDate, req.Reason)
	if err != nil {
		c.JSON(http.StatusBadRequest, types.ErrorResp{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, updated)
}

func (h *Handler) HandleVacateRenter(c *gin.Context) {
	if _, ok := h.ownsProperty(c, c.Param("propertyId")); !ok {
		return
	}
	var req types.VacateReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, types.ErrorResp{Error: err.Error()})
		return
	}
	moveOutDate, err := time.Parse(time.RFC3339, req.MoveOutDate)
	if err != nil {
		c.JSON(http.StatusBadRequest, types.ErrorResp{Error: "invalid moveOutDate"})
		return
	}
	if err := h.Store.VacateRenter(c.Request.Context(), c.Param("unitId"), c.Param("renterId"), moveOutDate); err != nil {
		c.JSON(http.StatusInternalServerError, types.ErrorResp{Error: err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) HandleRestoreRenter(c *gin.Context) {
	if _, ok := h.ownsProperty(c, c.Param("propertyId")); !ok {
		return
	}
	if err := h.Store.RestoreRenter(c.Request.Context(), c.Param("unitId"), c.Param("renterId")); err != nil {
		c.JSON(http.StatusInternalServerError, types.ErrorResp{Error: err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) HandleDeleteRenter(c *gin.Context) {
	if _, ok := h.ownsProperty(c, c.Param("propertyId")); !ok {
		return
	}
	renter, err := h.Store.GetRenter(c.Request.Context(), c.Param("renterId"))
	if err != nil {
		c.JSON(http.StatusNotFound, types.ErrorResp{Error: err.Error()})
		return
	}
	if err := h.Store.DeleteRenter(c.Request.Context(), c.Param("unitId"), renter.ID, renter.IsActive); err != nil {
		c.JSON(http.StatusInternalServerError, types.ErrorResp{Error: err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}
