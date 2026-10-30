package store

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/sofolitltd/barivara-server/internal/models"
)

type MonthlyBillResult struct {
	Created       int                       `json:"created"`
	Skipped       int                       `json:"skipped"`
	Failed        int                       `json:"failed"`
	Notifications []MonthlyBillNotification `json:"-"`
}

type MonthlyBillNotification struct {
	InvoiceID    string
	RenterID     string
	RenterName   string
	Phone        string
	MonthYear    string
	TotalAmount  int
	FcmToken     string
	SmsEnabled   bool
	PropertyName string
}

// GenerateMonthlyBills creates one invoice for every occupied unit. It is
// safe to run repeatedly because the month/unit unique index makes reruns no-ops.
func (s *Store) GenerateMonthlyBills(ctx context.Context, monthYear string, propertyIDs ...string) (MonthlyBillResult, error) {
	propertyID := ""
	if len(propertyIDs) > 0 {
		propertyID = propertyIDs[0]
	}
	return s.generateMonthlyBills(ctx, monthYear, propertyID, "", nil)
}

func (s *Store) GenerateMonthlyBillsForOwner(ctx context.Context, ownerID, monthYear string, propertyIDs ...string) (MonthlyBillResult, error) {
	return s.generateMonthlyBills(ctx, monthYear, "", ownerID, propertyIDs)
}

func (s *Store) generateMonthlyBills(ctx context.Context, monthYear, propertyID, ownerID string, ownerPropertyIDs []string) (MonthlyBillResult, error) {
	query := `
		SELECT u.id, u.property_id, p.name, u.base_rent, COALESCE(u.default_utilities, '{}'::jsonb),
			r.id, r.name, r.rent_history, r.phone, r.sms_enabled, COALESCE(tenant.fcm_token, '')
		FROM units u
		JOIN properties p ON p.id=u.property_id
		JOIN renters r ON r.unit_id=u.id AND r.property_id=u.property_id AND r.is_active=true
		LEFT JOIN users tenant ON tenant.id=r.linked_user_id
		WHERE (u.status='occupied' OR u.is_occupied=true)`
	args := []any{}
	if propertyID != "" {
		args = append(args, propertyID)
		query += fmt.Sprintf(` AND u.property_id=$%d`, len(args))
	}
	if ownerID != "" {
		args = append(args, ownerID)
		query += fmt.Sprintf(` AND p.owner_id=$%d`, len(args))
	}
	if len(ownerPropertyIDs) > 0 {
		args = append(args, ownerPropertyIDs)
		query += fmt.Sprintf(` AND u.property_id::text = ANY($%d::text[])`, len(args))
	}
	rows, err := s.Pool.Query(ctx, query, args...)
	if err != nil {
		return MonthlyBillResult{}, err
	}
	defer rows.Close()

	var candidates []struct {
		unitID, propertyID, propertyName, renterID, renterName string
		baseRent                                               int
		utilitiesJSON, rentHistoryJSON                         []byte
		phone                                                  string
		renterSmsEnabled                                       bool
		fcmToken                                               string
	}
	for rows.Next() {
		var c struct {
			unitID, propertyID, propertyName, renterID, renterName string
			baseRent                                               int
			utilitiesJSON, rentHistoryJSON                         []byte
			phone                                                  string
			renterSmsEnabled                                       bool
			fcmToken                                               string
		}
		if err := rows.Scan(&c.unitID, &c.propertyID, &c.propertyName, &c.baseRent, &c.utilitiesJSON,
			&c.renterID, &c.renterName, &c.rentHistoryJSON, &c.phone, &c.renterSmsEnabled, &c.fcmToken); err != nil {
			return MonthlyBillResult{}, err
		}
		candidates = append(candidates, c)
	}
	if err := rows.Err(); err != nil {
		return MonthlyBillResult{}, err
	}

	result := MonthlyBillResult{}
	for _, c := range candidates {
		utilities := map[string]int{}
		_ = json.Unmarshal(c.utilitiesJSON, &utilities)
		baseRent := c.baseRent
		var history []models.TenantRentConfig
		if err := json.Unmarshal(c.rentHistoryJSON, &history); err == nil {
			if activeRent, ok := activeRentForMonth(history, monthYear); ok {
				baseRent = activeRent
			}
		}
		totalUtilities := 0
		for _, amount := range utilities {
			totalUtilities += amount
		}
		tx, err := s.Pool.Begin(ctx)
		if err != nil {
			result.Failed++
			continue
		}
		legacyMonthYear := legacyMonthKey(monthYear)
		var alreadyExists bool
		if err := tx.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT 1 FROM invoices
				WHERE unit_id=$1 AND (month_year=$2 OR month_year=$3)
			)`, c.unitID, monthYear, legacyMonthYear).Scan(&alreadyExists); err != nil {
			tx.Rollback(ctx)
			result.Failed++
			continue
		}
		if alreadyExists {
			tx.Rollback(ctx)
			result.Skipped++
			continue
		}
		var current int
		if err := tx.QueryRow(ctx, `
			INSERT INTO counters (property_id, invoice_current) VALUES ($1,1)
			ON CONFLICT (property_id) DO UPDATE SET invoice_current=counters.invoice_current+1
			RETURNING invoice_current`, c.propertyID).Scan(&current); err != nil {
			tx.Rollback(ctx)
			result.Failed++
			continue
		}
		tag, err := tx.Exec(ctx, `
			INSERT INTO invoices
			(property_id, unit_id, renter_id, renter_name, invoice_number, month_year, base_rent,
			 utilities, other_charges, total_amount, status, payments)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,0,$9,'unpaid','[]')
			ON CONFLICT (unit_id, month_year) DO NOTHING`,
			c.propertyID, c.unitID, c.renterID, c.renterName, fmt.Sprintf("%04d", current), monthYear,
			baseRent, toJSON(utilities), baseRent+totalUtilities)
		if err != nil {
			tx.Rollback(ctx)
			result.Failed++
			continue
		}
		if tag.RowsAffected() == 0 {
			tx.Rollback(ctx)
			result.Skipped++
			continue
		}
		var invoiceID string
		if err := tx.QueryRow(ctx, `SELECT id::text FROM invoices WHERE unit_id=$1 AND month_year=$2`,
			c.unitID, monthYear).Scan(&invoiceID); err != nil {
			tx.Rollback(ctx)
			result.Failed++
			continue
		}
		if err := tx.Commit(ctx); err != nil {
			result.Failed++
			continue
		}
		result.Created++
		result.Notifications = append(result.Notifications, MonthlyBillNotification{
			InvoiceID: invoiceID, RenterID: c.renterID, RenterName: c.renterName,
			Phone: c.phone, MonthYear: monthYear, TotalAmount: baseRent + totalUtilities,
			FcmToken: c.fcmToken, SmsEnabled: c.renterSmsEnabled, PropertyName: c.propertyName,
		})
	}
	return result, nil
}

func legacyMonthKey(monthYear string) string {
	month, err := time.Parse("January 2006", monthYear)
	if err != nil {
		return monthYear
	}
	return month.Format("2006-01")
}

func activeRentForMonth(history []models.TenantRentConfig, monthYear string) (int, bool) {
	parts := strings.Fields(monthYear)
	if len(parts) != 2 {
		return 0, false
	}
	month := map[string]int{
		"January": 1, "February": 2, "March": 3, "April": 4, "May": 5, "June": 6,
		"July": 7, "August": 8, "September": 9, "October": 10, "November": 11, "December": 12,
	}[parts[0]]
	year := 0
	fmt.Sscanf(parts[1], "%d", &year)
	if month == 0 || year == 0 {
		return 0, false
	}
	target := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)
	amount := 0
	var effective time.Time
	for _, config := range history {
		if !config.StartDate.After(target) && (effective.IsZero() || !config.StartDate.Before(effective)) {
			effective = config.StartDate
			amount = config.Amount
		}
	}
	return amount, amount > 0
}
