package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/sofolitltd/barivara-server/internal/middleware"
	"github.com/sofolitltd/barivara-server/internal/models"
	"github.com/sofolitltd/barivara-server/internal/store"
	"github.com/sofolitltd/barivara-server/internal/types"
)

// ownsProperty loads the property and ensures the requesting user is its
// owner; writes an error response and returns ok=false if not.
func (h *Handler) ownsProperty(c *gin.Context, propertyID string) (*models.Property, bool) {
	prop, err := h.Store.GetProperty(c.Request.Context(), propertyID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			c.JSON(http.StatusNotFound, types.ErrorResp{Error: "property not found"})
		} else {
			c.JSON(http.StatusInternalServerError, types.ErrorResp{Error: err.Error()})
		}
		return nil, false
	}
	if prop.OwnerID != middleware.UserID(c) {
		c.JSON(http.StatusForbidden, types.ErrorResp{Error: "you do not own this property"})
		return nil, false
	}
	return prop, true
}

func (h *Handler) HandleCreateProperty(c *gin.Context) {
	var p models.Property
	if err := c.ShouldBindJSON(&p); err != nil {
		c.JSON(http.StatusBadRequest, types.ErrorResp{Error: err.Error()})
		return
	}
	created, err := h.Store.CreateProperty(c.Request.Context(), middleware.UserID(c), p)
	if err != nil {
		c.JSON(http.StatusInternalServerError, types.ErrorResp{Error: err.Error()})
		return
	}
	c.JSON(http.StatusCreated, created)
}

func (h *Handler) HandleListProperties(c *gin.Context) {
	props, err := h.Store.ListPropertiesByOwner(c.Request.Context(), middleware.UserID(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, types.ErrorResp{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, props)
}

func (h *Handler) HandleGetProperty(c *gin.Context) {
	prop, ok := h.ownsProperty(c, c.Param("propertyId"))
	if !ok {
		return
	}
	c.JSON(http.StatusOK, prop)
}

func (h *Handler) HandleUpdateProperty(c *gin.Context) {
	if _, ok := h.ownsProperty(c, c.Param("propertyId")); !ok {
		return
	}
	var p models.Property
	if err := c.ShouldBindJSON(&p); err != nil {
		c.JSON(http.StatusBadRequest, types.ErrorResp{Error: err.Error()})
		return
	}
	p.ID = c.Param("propertyId")
	updated, err := h.Store.UpdateProperty(c.Request.Context(), p)
	if err != nil {
		c.JSON(http.StatusInternalServerError, types.ErrorResp{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, updated)
}

func (h *Handler) HandleDeleteProperty(c *gin.Context) {
	if _, ok := h.ownsProperty(c, c.Param("propertyId")); !ok {
		return
	}
	if err := h.Store.DeleteProperty(c.Request.Context(), c.Param("propertyId")); err != nil {
		c.JSON(http.StatusInternalServerError, types.ErrorResp{Error: err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}
