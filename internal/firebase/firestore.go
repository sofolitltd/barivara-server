package firebase

import (
	"context"
	"fmt"
	"os"
	"time"

	"cloud.google.com/go/firestore"
	firebase "firebase.google.com/go/v4"
	"google.golang.org/api/option"
)

type Invoice struct {
	ID         string `firestore:"-" json:"id"`
	PropertyID string `firestore:"propertyId" json:"propertyId"`
	UnitID     string `firestore:"unitId" json:"unitId"`
	RenterID   string `firestore:"renterId" json:"renterId"`
	RenterName string `firestore:"renterName" json:"renterName"`
	BaseRent   int    `firestore:"baseRent" json:"baseRent"`
	TotalAmount int   `firestore:"totalAmount" json:"totalAmount"`
	MonthYear  string `firestore:"monthYear" json:"monthYear"`
	Status     string `firestore:"status" json:"status"`
}

type Renter struct {
	ID         string `firestore:"-" json:"id"`
	Name       string `firestore:"name" json:"name"`
	Phone      string `firestore:"phone" json:"phone"`
	PropertyID string `firestore:"propertyId" json:"propertyId"`
	IsActive   bool   `firestore:"isActive" json:"isActive"`
}

type User struct {
	UID  string `firestore:"uid" json:"uid"`
	Plan string `firestore:"plan" json:"plan"`
}

type Property struct {
	ID           string `firestore:"-" json:"id"`
	Name         string `firestore:"name" json:"name"`
	OwnerID      string `firestore:"ownerId" json:"ownerId"`
	ReminderDay  int    `firestore:"reminderDay" json:"reminderDay"`
	ReminderHour int    `firestore:"reminderHour" json:"reminderHour"`
	SmsEnabled   bool   `firestore:"smsEnabled" json:"smsEnabled"`
}

type Client struct {
	firestore *firestore.Client
}

func New(ctx context.Context) (*Client, error) {
	var creds []byte

	if raw := os.Getenv("FIREBASE_CREDENTIALS"); raw != "" {
		creds = []byte(raw)
	} else if path := os.Getenv("FIREBASE_CREDENTIALS_PATH"); path != "" {
		var err error
		creds, err = os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read credentials file: %w", err)
		}
	} else {
		return nil, fmt.Errorf("FIREBASE_CREDENTIALS or FIREBASE_CREDENTIALS_PATH must be set")
	}

	app, err := firebase.NewApp(ctx, nil, option.WithCredentialsJSON(creds))
	if err != nil {
		return nil, fmt.Errorf("firebase init: %w", err)
	}

	f, err := app.Firestore(ctx)
	if err != nil {
		return nil, fmt.Errorf("firestore init: %w", err)
	}

	return &Client{firestore: f}, nil
}

func (c *Client) Close() error {
	return c.firestore.Close()
}

func (c *Client) GetInvoice(ctx context.Context, invoiceID string) (*Invoice, error) {
	doc, err := c.firestore.Collection("invoices").Doc(invoiceID).Get(ctx)
	if err != nil {
		return nil, fmt.Errorf("get invoice: %w", err)
	}
	var inv Invoice
	if err := doc.DataTo(&inv); err != nil {
		return nil, fmt.Errorf("parse invoice: %w", err)
	}
	inv.ID = doc.Ref.ID
	return &inv, nil
}

func (c *Client) GetRenter(ctx context.Context, renterID string) (*Renter, error) {
	doc, err := c.firestore.Collection("renters").Doc(renterID).Get(ctx)
	if err != nil {
		return nil, fmt.Errorf("get renter: %w", err)
	}
	var renter Renter
	if err := doc.DataTo(&renter); err != nil {
		return nil, fmt.Errorf("parse renter: %w", err)
	}
	renter.ID = doc.Ref.ID
	return &renter, nil
}

func (c *Client) GetProperty(ctx context.Context, propertyID string) (*Property, error) {
	doc, err := c.firestore.Collection("properties").Doc(propertyID).Get(ctx)
	if err != nil {
		return nil, fmt.Errorf("get property: %w", err)
	}
	var prop Property
	if err := doc.DataTo(&prop); err != nil {
		return nil, fmt.Errorf("parse property: %w", err)
	}
	prop.ID = doc.Ref.ID
	return &prop, nil
}

type SMSLog struct {
	RenterID   string    `firestore:"renterId"`
	InvoiceID  string    `firestore:"invoiceId"`
	Type       string    `firestore:"type"` // reminder / receipt
	Phone      string    `firestore:"phone"`
	Message    string    `firestore:"message"`
	Status     string    `firestore:"status"` // sent / failed
	Error      string    `firestore:"error,omitempty"`
	SentAt     time.Time `firestore:"sentAt"`
}

func (c *Client) LogSMS(ctx context.Context, log SMSLog) error {
	_, _, err := c.firestore.Collection("sms_logs").Add(ctx, log)
	return err
}

func (c *Client) QueryPropertiesByReminderDay(ctx context.Context, day int) ([]Property, error) {
	snap, err := c.firestore.Collection("properties").
		Where("reminderDay", "==", day).
		Documents(ctx).GetAll()
	if err != nil {
		return nil, fmt.Errorf("query properties: %w", err)
	}
	var props []Property
	for _, doc := range snap {
		var p Property
		if err := doc.DataTo(&p); err != nil {
			continue
		}
		p.ID = doc.Ref.ID
		props = append(props, p)
	}
	return props, nil
}

func (c *Client) QueryActiveRentersByProperty(ctx context.Context, propertyID string) ([]Renter, error) {
	snap, err := c.firestore.Collection("renters").
		Where("propertyId", "==", propertyID).
		Where("isActive", "==", true).
		Documents(ctx).GetAll()
	if err != nil {
		return nil, fmt.Errorf("query renters: %w", err)
	}
	var renters []Renter
	for _, doc := range snap {
		var r Renter
		if err := doc.DataTo(&r); err != nil {
			continue
		}
		r.ID = doc.Ref.ID
		renters = append(renters, r)
	}
	return renters, nil
}

func (c *Client) GetUserPlan(ctx context.Context, uid string) (string, error) {
	doc, err := c.firestore.Collection("users").Doc(uid).Get(ctx)
	if err != nil {
		return "", fmt.Errorf("get user: %w", err)
	}
	var u User
	if err := doc.DataTo(&u); err != nil {
		return "", fmt.Errorf("parse user: %w", err)
	}
	return u.Plan, nil
}

func (c *Client) QueryUnpaidInvoicesByRenter(ctx context.Context, renterID string) ([]Invoice, error) {
	snap, err := c.firestore.Collection("invoices").
		Where("renterId", "==", renterID).
		Where("status", "in", []string{"unpaid", "due"}).
		Documents(ctx).GetAll()
	if err != nil {
		return nil, fmt.Errorf("query invoices: %w", err)
	}
	var invoices []Invoice
	for _, doc := range snap {
		var inv Invoice
		if err := doc.DataTo(&inv); err != nil {
			continue
		}
		inv.ID = doc.Ref.ID
		invoices = append(invoices, inv)
	}
	return invoices, nil
}
