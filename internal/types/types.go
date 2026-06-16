package types

type SendReminderReq struct {
	RenterID  string `json:"renterId" binding:"required"`
	InvoiceID string `json:"invoiceId" binding:"required"`
}

type SendReceiptReq struct {
	RenterID  string `json:"renterId" binding:"required"`
	InvoiceID string `json:"invoiceId" binding:"required"`
}

type CronResp struct {
	Skipped bool   `json:"skipped,omitempty"`
	Reason  string `json:"reason,omitempty"`
	Sent    int    `json:"sent,omitempty"`
	Failed  int    `json:"failed,omitempty"`
	Total   int    `json:"total,omitempty"`
	Message string `json:"message,omitempty"`
}

type SendResp struct {
	Success bool   `json:"success"`
	SentTo  string `json:"sent_to,omitempty"`
	Message string `json:"message,omitempty"`
	Error   string `json:"error,omitempty"`
}

const InvoiceBaseURL = "https://barivarabd.web.app/invoice"
