package models

import "time"

type User struct {
	ID        string    `json:"uid"`
	Name      string    `json:"name"`
	Phone     string    `json:"phone"`
	Email     string    `json:"email"`
	Contacts  []Contact `json:"contacts"`
	Role      string    `json:"role"` // renter | landlord | admin | pending_landlord | rejected_landlord
	Status    string    `json:"status"`
	Plan      string    `json:"plan"`
	PhotoURL  *string   `json:"photoUrl,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
}

type Contact struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

type Property struct {
	ID            string   `json:"id"`
	OwnerID       string   `json:"ownerId"`
	Name          string   `json:"name"`
	Address       string   `json:"address"`
	ReminderDay   int      `json:"reminderDay"`
	ReminderHour  int      `json:"reminderHour"`
	SmsEnabled    bool     `json:"smsEnabled"`
	GoogleMapsURL *string  `json:"googleMapsUrl,omitempty"`
	PropertyType  *string  `json:"propertyType,omitempty"`
	ImageURL      *string  `json:"imageUrl,omitempty"`
	ImageURLs     []string `json:"imageUrls"`
	VideoURL      *string  `json:"videoUrl,omitempty"`
}

type RentConfig struct {
	Amount    int       `json:"amount"`
	StartDate time.Time `json:"startDate"`
	Reason    *string   `json:"reason,omitempty"`
}

type DepositConfig struct {
	Amount    int       `json:"amount"`
	StartDate time.Time `json:"startDate"`
	Reason    *string   `json:"reason,omitempty"`
}

type Unit struct {
	ID               string            `json:"id"`
	PropertyID       string            `json:"propertyId"`
	UnitNumber       string            `json:"unitNumber"`
	BaseRent         int               `json:"baseRent"`
	IsOccupied       bool              `json:"isOccupied"`
	DefaultUtilities map[string]int    `json:"defaultUtilities"`
	RentHistory      []RentConfig      `json:"rentHistory"`
	DepositHistory   []DepositConfig   `json:"depositHistory"`
	FloorLevel       *int              `json:"floorLevel,omitempty"`
	UnitSize         *string           `json:"unitSize,omitempty"`
	UnitType         *string           `json:"unitType,omitempty"`
	SecurityDeposit  *int              `json:"securityDeposit,omitempty"`
	Status           string            `json:"status"`
	Meters           map[string]string `json:"meters"`
	ImageURL         *string           `json:"imageUrl,omitempty"`
	VideoURL         *string           `json:"videoUrl,omitempty"`
}

type RenterDocument struct {
	Title      string    `json:"title"`
	URL        string    `json:"url"`
	UploadedAt time.Time `json:"uploadedAt"`
}

type FamilyMember struct {
	Name               string           `json:"name"`
	Relation           string           `json:"relation"`
	Age                *string          `json:"age,omitempty"`
	Nid                *string          `json:"nid,omitempty"`
	Phone              *string          `json:"phone,omitempty"`
	Email              *string          `json:"email,omitempty"`
	Occupation         *string          `json:"occupation,omitempty"`
	PhotoURL           *string          `json:"photoUrl,omitempty"`
	Documents          []RenterDocument `json:"documents"`
	IsEmergencyContact bool             `json:"isEmergencyContact"`
}

type TenantRentConfig struct {
	Amount    int       `json:"amount"`
	StartDate time.Time `json:"startDate"`
	Reason    *string   `json:"reason,omitempty"`
}

type TenantDepositConfig struct {
	Amount    int       `json:"amount"`
	StartDate time.Time `json:"startDate"`
	Reason    *string   `json:"reason,omitempty"`
}

type Renter struct {
	ID             string                `json:"id"`
	PropertyID     string                `json:"propertyId"`
	UnitID         string                `json:"unitId"`
	Name           string                `json:"name"`
	Phone          string                `json:"phone"`
	Nid            string                `json:"nid"`
	AdvanceDeposit int                   `json:"advanceDeposit"`
	MoveInDate     time.Time             `json:"moveInDate"`
	Email          *string               `json:"email,omitempty"`
	Occupation     *string               `json:"occupation,omitempty"`
	AlternatePhone *string               `json:"alternatePhone,omitempty"`
	PhotoURL       *string               `json:"photoUrl,omitempty"`
	MoveOutDate    *time.Time            `json:"moveOutDate,omitempty"`
	IsActive       bool                  `json:"isActive"`
	SmsEnabled     bool                  `json:"smsEnabled"`
	FamilyMembers  []FamilyMember        `json:"familyMembers"`
	Documents      []RenterDocument      `json:"documents"`
	LandlordNotes  *string               `json:"landlordNotes,omitempty"`
	JoinCode       *string               `json:"joinCode,omitempty"`
	LinkedUserID   *string               `json:"linkedUserId,omitempty"`
	RentHistory    []TenantRentConfig    `json:"rentHistory"`
	DepositHistory []TenantDepositConfig `json:"depositHistory"`
}

type Payment struct {
	Amount    int       `json:"amount"`
	PaidAt    time.Time `json:"paidAt"`
	Note      *string   `json:"note,omitempty"`
	PaidItems []string  `json:"paidItems"`
}

type Invoice struct {
	ID            string         `json:"id"`
	PropertyID    string         `json:"propertyId"`
	PropertyName  string         `json:"propertyName,omitempty"`
	UnitID        string         `json:"unitId"`
	UnitNumber    string         `json:"unitNumber,omitempty"`
	RenterID      *string        `json:"renterId,omitempty"`
	RenterName    *string        `json:"renterName,omitempty"`
	OwnerName     string         `json:"ownerName,omitempty"`
	OwnerPhone    string         `json:"ownerPhone,omitempty"`
	InvoiceNumber string         `json:"invoiceNumber"`
	MonthYear     string         `json:"monthYear"`
	BaseRent      int            `json:"baseRent"`
	Utilities     map[string]int `json:"utilities"`
	OtherCharges  int            `json:"otherCharges"`
	TotalAmount   int            `json:"totalAmount"`
	Status        string         `json:"status"`
	Payments      []Payment      `json:"payments"`
	CreatedAt     *time.Time     `json:"createdAt,omitempty"`
	PaidAt        *time.Time     `json:"paidAt,omitempty"`
}

type LandlordRequest struct {
	ID                 string    `json:"id"`
	UserID             string    `json:"userId"`
	UserName           string    `json:"userName"`
	Phone              string    `json:"phone"`
	NidNumber          string    `json:"nidNumber"`
	OwnerProofImageURL string    `json:"ownerProofImageUrl"`
	Status             string    `json:"status"`
	CreatedAt          time.Time `json:"createdAt"`
}

type RentPost struct {
	ID           string    `json:"id"`
	UnitID       string    `json:"unitId"`
	PropertyID   string    `json:"propertyId"`
	LandlordID   string    `json:"landlordId"`
	Title        string    `json:"title"`
	Description  string    `json:"description"`
	Location     string    `json:"location"`
	RentAmount   int       `json:"rentAmount"`
	ImageURLs    []string  `json:"imageUrls"`
	CreatedAt    time.Time `json:"createdAt"`
	IsActive     bool      `json:"isActive"`
	PropertyType *string   `json:"propertyType,omitempty"`
	Beds         *int      `json:"beds,omitempty"`
	Baths        *int      `json:"baths,omitempty"`
	AreaSqft     *string   `json:"areaSqft,omitempty"`
}

type TenantLinkRequest struct {
	ID           string    `json:"id"`
	RenterID     string    `json:"renterId"`
	PropertyID   string    `json:"propertyId"`
	UnitID       string    `json:"unitId"`
	TenantUserID string    `json:"tenantUserId"`
	TenantName   string    `json:"tenantName"`
	TenantEmail  string    `json:"tenantEmail"`
	TenantPhone  string    `json:"tenantPhone"`
	Status       string    `json:"status"`
	CreatedAt    time.Time `json:"createdAt"`
}

type SMSPackage struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Description  string    `json:"description"`
	MessageCount int       `json:"messageCount"`
	Price        int       `json:"price"`
	IsActive     bool      `json:"isActive"`
	CreatedAt    time.Time `json:"createdAt"`
}

type SMSPurchaseRequest struct {
	ID           string     `json:"id"`
	LandlordID   string     `json:"landlordId"`
	PackageID    string     `json:"packageId"`
	PackageName  string     `json:"packageName"`
	MessageCount int        `json:"messageCount"`
	Amount       int        `json:"amount"`
	Note         string     `json:"note"`
	Status       string     `json:"status"`
	CreatedAt    time.Time  `json:"createdAt"`
	ProcessedAt  *time.Time `json:"processedAt,omitempty"`
}

type Notice struct {
	ID             string     `json:"id"`
	LandlordID     string     `json:"landlordId"`
	PropertyID     string     `json:"propertyId"`
	PropertyName   string     `json:"propertyName,omitempty"`
	Title          string     `json:"title"`
	Body           string     `json:"body"`
	Priority       string     `json:"priority"`
	PublishedAt    time.Time  `json:"publishedAt"`
	ExpiresAt      *time.Time `json:"expiresAt,omitempty"`
	CreatedAt      time.Time  `json:"createdAt"`
	ReadAt         *time.Time `json:"readAt,omitempty"`
	RecipientCount int        `json:"recipientCount"`
}

type Conversation struct {
	ID              string     `json:"id"`
	Type            string     `json:"type"`
	Status          string     `json:"status"`
	LandlordID      string     `json:"landlordId"`
	RenterID        string     `json:"renterId"`
	RenterName      string     `json:"renterName,omitempty"`
	PeerName        string     `json:"peerName,omitempty"`
	RequestSentByMe bool       `json:"requestSentByMe"`
	PropertyID      string     `json:"propertyId"`
	PropertyName    string     `json:"propertyName,omitempty"`
	UnitID          string     `json:"unitId"`
	UnitNumber      string     `json:"unitNumber,omitempty"`
	CreatedAt       time.Time  `json:"createdAt"`
	UpdatedAt       time.Time  `json:"updatedAt"`
	LastMessage     string     `json:"lastMessage,omitempty"`
	LastAt          *time.Time `json:"lastAt,omitempty"`
	UnreadCount     int        `json:"unreadCount"`
}

type TenantContact struct {
	UserID             string `json:"userId"`
	Name               string `json:"name"`
	UnitNumber         string `json:"unitNumber"`
	ConversationID     string `json:"conversationId,omitempty"`
	ConversationStatus string `json:"conversationStatus,omitempty"`
	RequestSentByMe    bool   `json:"requestSentByMe"`
}

type ConversationMessage struct {
	ID                 string     `json:"id"`
	ConversationID     string     `json:"conversationId"`
	SenderUserID       string     `json:"senderUserId"`
	Body               string     `json:"body"`
	CreatedAt          time.Time  `json:"createdAt"`
	ReadAt             *time.Time `json:"readAt,omitempty"`
	EditedAt           *time.Time `json:"editedAt,omitempty"`
	DeletedForEveryone bool       `json:"deletedForEveryone"`
}

type ConversationPushTarget struct {
	Token      string
	SenderName string
	PeerName   string
}
