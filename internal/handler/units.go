package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/sofolitltd/barivara-server/internal/models"
	"github.com/sofolitltd/barivara-server/internal/types"
)

func (h *Handler) HandleCreateUnit(c *gin.Context) {
	propertyID := c.Param("propertyId")
	if _, ok := h.ownsProperty(c, propertyID); !ok {
		return
	}
	var u models.Unit
	if err := c.ShouldBindJSON(&u); err != nil {
		c.JSON(http.StatusBadRequest, types.ErrorResp{Error: err.Error()})
		return
	}
	u.PropertyID = propertyID
	created, err := h.Store.CreateUnit(c.Request.Context(), u)
	if err != nil {
		c.JSON(http.StatusInternalServerError, types.ErrorResp{Error: err.Error()})
		return
	}
	c.JSON(http.StatusCreated, created)
}

func (h *Handler) HandleListUnits(c *gin.Context) {
	propertyID := c.Param("propertyId")
	if _, ok := h.ownsProperty(c, propertyID); !ok {
		return
	}
	units, err := h.Store.ListUnitsByProperty(c.Request.Context(), propertyID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, types.ErrorResp{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, units)
}

func (h *Handler) HandleGetUnit(c *gin.Context) {
	if _, ok := h.ownsProperty(c, c.Param("propertyId")); !ok {
		return
	}
	unit, err := h.Store.GetUnit(c.Request.Context(), c.Param("unitId"))
	if err != nil {
		c.JSON(http.StatusNotFound, types.ErrorResp{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, unit)
}

func (h *Handler) HandleUpdateUnit(c *gin.Context) {
	if _, ok := h.ownsProperty(c, c.Param("propertyId")); !ok {
		return
	}
	var u models.Unit
	if err := c.ShouldBindJSON(&u); err != nil {
		c.JSON(http.StatusBadRequest, types.ErrorResp{Error: err.Error()})
		return
	}
	u.ID = c.Param("unitId")
	updated, err := h.Store.UpdateUnit(c.Request.Context(), u)
	if err != nil {
		c.JSON(http.StatusInternalServerError, types.ErrorResp{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, updated)
}

func (h *Handler) HandleDeleteUnit(c *gin.Context) {
	if _, ok := h.ownsProperty(c, c.Param("propertyId")); !ok {
		return
	}
	if err := h.Store.DeleteUnit(c.Request.Context(), c.Param("unitId")); err != nil {
		c.JSON(http.StatusInternalServerError, types.ErrorResp{Error: err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}
