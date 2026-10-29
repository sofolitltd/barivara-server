package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/sofolitltd/barivara-server/internal/middleware"
	"github.com/sofolitltd/barivara-server/internal/models"
	"github.com/sofolitltd/barivara-server/internal/types"
)

func (h *Handler) HandleListActiveRentPosts(c *gin.Context) {
	posts, err := h.Store.ListActiveRentPosts(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, types.ErrorResp{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, posts)
}

func (h *Handler) HandleGetRentPost(c *gin.Context) {
	post, err := h.Store.GetRentPost(c.Request.Context(), c.Param("postId"))
	if err != nil {
		c.JSON(http.StatusNotFound, types.ErrorResp{Error: "post not found"})
		return
	}
	c.JSON(http.StatusOK, post)
}

func (h *Handler) HandleGetRentPostForUnit(c *gin.Context) {
	post, err := h.Store.GetActiveRentPostForUnit(c.Request.Context(), c.Param("unitId"))
	if err != nil {
		c.JSON(http.StatusOK, nil)
		return
	}
	c.JSON(http.StatusOK, post)
}

func (h *Handler) HandleCreateRentPost(c *gin.Context) {
	var p models.RentPost
	if err := c.ShouldBindJSON(&p); err != nil {
		c.JSON(http.StatusBadRequest, types.ErrorResp{Error: err.Error()})
		return
	}
	p.LandlordID = middleware.UserID(c)
	created, err := h.Store.CreateRentPost(c.Request.Context(), p)
	if err != nil {
		c.JSON(http.StatusInternalServerError, types.ErrorResp{Error: err.Error()})
		return
	}
	c.JSON(http.StatusCreated, created)
}

func (h *Handler) HandleUpdateRentPost(c *gin.Context) {
	var p models.RentPost
	if err := c.ShouldBindJSON(&p); err != nil {
		c.JSON(http.StatusBadRequest, types.ErrorResp{Error: err.Error()})
		return
	}
	p.ID = c.Param("postId")
	updated, err := h.Store.UpdateRentPost(c.Request.Context(), p)
	if err != nil {
		c.JSON(http.StatusInternalServerError, types.ErrorResp{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, updated)
}

func (h *Handler) HandleDeleteRentPost(c *gin.Context) {
	if err := h.Store.DeleteRentPost(c.Request.Context(), c.Param("postId")); err != nil {
		c.JSON(http.StatusInternalServerError, types.ErrorResp{Error: err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}
