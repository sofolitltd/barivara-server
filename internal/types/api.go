package types

import "github.com/sofolitltd/barivara-server/internal/models"

type SignUpReq struct {
	Name     string `json:"name" binding:"required"`
	Phone    string `json:"phone" binding:"required"`
	Email    string `json:"email"`
	Password string `json:"password" binding:"required,min=6"`
	Role     string `json:"role"` // "landlord" | "renter"; defaults to "renter"
}

type SignInReq struct {
	Phone    string `json:"phone" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type ChangePasswordReq struct {
	CurrentPassword string `json:"currentPassword" binding:"required"`
	NewPassword     string `json:"newPassword" binding:"required,min=6"`
}

type AuthResp struct {
	Token string      `json:"token"`
	User  models.User `json:"user"`
}

type AdminSignInReq struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

type AdminInfo struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

type AdminAuthResp struct {
	Token string    `json:"token"`
	Admin AdminInfo `json:"admin"`
}

type ErrorResp struct {
	Error string `json:"error"`
}

type LinkByCodeReq struct {
	Code string `json:"code" binding:"required"`
}

type LinkByEmailReq struct {
	Email string `json:"email" binding:"required"`
}

type LinkByPhoneReq struct {
	Phone string `json:"phone" binding:"required"`
}

type RecordPaymentReq struct {
	Amount    int      `json:"amount" binding:"required"`
	Note      string   `json:"note"`
	PaidItems []string `json:"paidItems"`
}

type UpdateStatusReq struct {
	Status string `json:"status" binding:"required"`
}

type VacateReq struct {
	MoveOutDate string `json:"moveOutDate" binding:"required"`
}

type PresignUploadReq struct {
	Folder      string `json:"folder" binding:"required"`
	ContentType string `json:"contentType" binding:"required"`
}

type PresignUploadResp struct {
	UploadURL string `json:"uploadUrl"`
	PublicURL string `json:"publicUrl"`
}
