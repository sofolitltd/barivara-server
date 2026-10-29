package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/sofolitltd/barivara-server/internal/types"
)

// HandleUploadPresign issues a short-lived presigned PUT URL plus the
// object's permanent public URL (the uploads bucket is `public_read`),
// so the client uploads straight to Neon Object Storage and stores the
// permanent URL on the owning record (renter photo, document, property
// image, …) exactly like the old Cloudinary `secure_url`.
func (h *Handler) HandleUploadPresign(c *gin.Context) {
	var req types.PresignUploadReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, types.ErrorResp{Error: err.Error()})
		return
	}

	uploadURL, publicURL, err := h.Storage.PresignUpload(c.Request.Context(), req.Folder, req.ContentType)
	if err != nil {
		c.JSON(http.StatusInternalServerError, types.ErrorResp{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, types.PresignUploadResp{UploadURL: uploadURL, PublicURL: publicURL})
}
