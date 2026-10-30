package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/sofolitltd/barivara-server/internal/models"
)

const invoiceCols = `id, property_id, unit_id, renter_id, renter_name, invoice_number, month_year,
	base_rent, utilities, other_charges, total_amount, status, payments, created_at, paid_at`

func scanInvoice(row pgx.Row) (*models.Invoice, error) {
	var inv models.Invoice
	var utilities, payments []byte

	if err := row.Scan(&inv.ID, &inv.PropertyID, &inv.UnitID, &inv.RenterID, &inv.RenterName,
		&inv.InvoiceNumber, &inv.MonthYear, &inv.BaseRent, &utilities, &inv.OtherCharges, &inv.TotalAmount,
		&inv.Status, &payments, &inv.CreatedAt, &inv.PaidAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	inv.Utilities = map[string]int{}
	inv.Payments = []models.Payment{}
	_ = json.Unmarshal(utilities, &inv.Utilities)
	_ = json.Unmarshal(payments, &inv.Payments)
	return &inv, nil
}

func collectInvoices(rows pgx.Rows) ([]models.Invoice, error) {
	out := []models.Invoice{}
	for rows.Next() {
		inv, err := scanInvoice(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *inv)
	}
	return out, nil
}

func normalizeInvoice(inv *models.Invoice) {
	if inv.Utilities == nil {
		inv.Utilities = map[string]int{}
	}
	if inv.Payments == nil {
		inv.Payments = []models.Payment{}
	}
}

// CreateInvoice looks up the active renter for the unit, stamps the next
// sequential invoice number for the property, and inserts the invoice.
func (s *Store) CreateInvoice(ctx context.Context, inv models.Invoice) (*models.Invoice, error) {
	normalizeInvoice(&inv)
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	var renterID, renterName string
	err = tx.QueryRow(ctx, `SELECT id, name FROM renters WHERE property_id=$1 AND unit_id=$2 AND is_active=true LIMIT 1`,
		inv.PropertyID, inv.UnitID).Scan(&renterID, &renterName)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("no active renter found for this unit; add a renter before creating a bill")
		}
		return nil, err
	}

	var current int
	if err := tx.QueryRow(ctx, `
		INSERT INTO counters (property_id, invoice_current) VALUES ($1, 1)
		ON CONFLICT (property_id) DO UPDATE SET invoice_current = counters.invoice_current + 1
		RETURNING invoice_current
	`, inv.PropertyID).Scan(&current); err != nil {
		return nil, fmt.Errorf("increment invoice counter: %w", err)
	}
	invoiceNumber := fmt.Sprintf("%04d", current)

	row := tx.QueryRow(ctx, `
		INSERT INTO invoices (property_id, unit_id, renter_id, renter_name, invoice_number, month_year,
			base_rent, utilities, other_charges, total_amount, status, payments)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
		RETURNING `+invoiceCols,
		inv.PropertyID, inv.UnitID, renterID, renterName, invoiceNumber, inv.MonthYear,
		inv.BaseRent, toJSON(inv.Utilities), inv.OtherCharges, inv.TotalAmount,
		orDefaultStr(inv.Status, "unpaid"), toJSON(inv.Payments))

	created, err := scanInvoice(row)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return created, nil
}

func (s *Store) GetInvoice(ctx context.Context, id string) (*models.Invoice, error) {
	row := s.Pool.QueryRow(ctx, `SELECT `+invoiceCols+` FROM invoices WHERE id = $1`, id)
	return scanInvoice(row)
}

func (s *Store) GetInvoiceWithDisplayDetails(ctx context.Context, id string) (*models.Invoice, error) {
	inv, err := s.GetInvoice(ctx, id)
	if err != nil {
		return nil, err
	}

	err = s.Pool.QueryRow(ctx, `
		SELECT p.name, u.unit_number, landlord_user.name, landlord_user.phone
		FROM invoices i
		JOIN properties p ON p.id = i.property_id
		JOIN units u ON u.id = i.unit_id
		JOIN users landlord_user ON landlord_user.id = p.owner_id
		WHERE i.id = $1
	`, id).Scan(&inv.PropertyName, &inv.UnitNumber, &inv.OwnerName, &inv.OwnerPhone)
	if err != nil {
		return nil, fmt.Errorf("load invoice display details: %w", err)
	}
	return inv, nil
}

func (s *Store) ListInvoicesByUnit(ctx context.Context, propertyID, unitID string) ([]models.Invoice, error) {
	rows, err := s.Pool.Query(ctx, `SELECT `+invoiceCols+` FROM invoices WHERE property_id=$1 AND unit_id=$2 ORDER BY created_at DESC`, propertyID, unitID)
	if err != nil {
		return nil, fmt.Errorf("list invoices by unit: %w", err)
	}
	defer rows.Close()
	return collectInvoices(rows)
}

func (s *Store) ListInvoicesByProperty(ctx context.Context, propertyID string) ([]models.Invoice, error) {
	rows, err := s.Pool.Query(ctx, `SELECT `+invoiceCols+` FROM invoices WHERE property_id=$1 ORDER BY created_at DESC`, propertyID)
	if err != nil {
		return nil, fmt.Errorf("list invoices by property: %w", err)
	}
	defer rows.Close()
	return collectInvoices(rows)
}

func (s *Store) ListInvoicesByRenter(ctx context.Context, renterID string) ([]models.Invoice, error) {
	rows, err := s.Pool.Query(ctx, `SELECT `+invoiceCols+` FROM invoices WHERE renter_id=$1 ORDER BY created_at DESC`, renterID)
	if err != nil {
		return nil, fmt.Errorf("list invoices by renter: %w", err)
	}
	defer rows.Close()
	return collectInvoices(rows)
}

func (s *Store) ListUnpaidInvoicesByRenter(ctx context.Context, renterID string) ([]models.Invoice, error) {
	rows, err := s.Pool.Query(ctx, `SELECT `+invoiceCols+` FROM invoices WHERE renter_id=$1 AND status IN ('unpaid','due')`, renterID)
	if err != nil {
		return nil, fmt.Errorf("list unpaid invoices by renter: %w", err)
	}
	defer rows.Close()
	return collectInvoices(rows)
}

func (s *Store) ListUnpaidInvoicesForOwner(ctx context.Context, ownerID string) ([]models.Invoice, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT `+invoiceCols+` FROM invoices
		WHERE status IN ('unpaid','due') AND property_id IN (SELECT id FROM properties WHERE owner_id = $1)
		ORDER BY created_at DESC
	`, ownerID)
	if err != nil {
		return nil, fmt.Errorf("list unpaid invoices for owner: %w", err)
	}
	defer rows.Close()
	return collectInvoices(rows)
}

func (s *Store) UpdateInvoice(ctx context.Context, inv models.Invoice) (*models.Invoice, error) {
	normalizeInvoice(&inv)
	row := s.Pool.QueryRow(ctx, `
		UPDATE invoices SET month_year=$2, base_rent=$3, utilities=$4, other_charges=$5, total_amount=$6,
			status=$7, payments=$8, paid_at=$9
		WHERE id = $1
		RETURNING `+invoiceCols,
		inv.ID, inv.MonthYear, inv.BaseRent, toJSON(inv.Utilities), inv.OtherCharges,
		inv.TotalAmount, inv.Status, toJSON(inv.Payments), inv.PaidAt)
	return scanInvoice(row)
}

func (s *Store) RecordPayment(ctx context.Context, invoiceID string, amount int, paidAt time.Time, note string, paidItems []string) (*models.Invoice, error) {
	inv, err := s.GetInvoice(ctx, invoiceID)
	if err != nil {
		return nil, err
	}

	var notePtr *string
	if note != "" {
		notePtr = &note
	}
	if paidItems == nil {
		paidItems = []string{}
	}
	payment := models.Payment{Amount: amount, PaidAt: paidAt, Note: notePtr, PaidItems: paidItems}
	updatedPayments := append(inv.Payments, payment)

	paidAmount := 0
	for _, p := range updatedPayments {
		paidAmount += p.Amount
	}
	status := "unpaid"
	var paidAtPtr *time.Time
	if paidAmount >= inv.TotalAmount {
		status = "paid"
		paidAtPtr = &paidAt
	}

	row := s.Pool.QueryRow(ctx, `
		UPDATE invoices SET payments=$2, status=$3, paid_at=$4 WHERE id=$1 RETURNING `+invoiceCols,
		invoiceID, toJSON(updatedPayments), status, paidAtPtr)
	return scanInvoice(row)
}

func (s *Store) MarkInvoicePaid(ctx context.Context, invoiceID string) (*models.Invoice, error) {
	inv, err := s.GetInvoice(ctx, invoiceID)
	if err != nil {
		return nil, err
	}
	remaining := inv.TotalAmount
	for _, p := range inv.Payments {
		remaining -= p.Amount
	}
	note := "Full payment"
	return s.RecordPayment(ctx, invoiceID, remaining, time.Now(), note, nil)
}

func (s *Store) UpdateInvoiceStatus(ctx context.Context, invoiceID, status string) error {
	var paidAt interface{}
	if status == "paid" {
		paidAt = time.Now()
	}
	_, err := s.Pool.Exec(ctx, `UPDATE invoices SET status=$2, paid_at=$3 WHERE id=$1`, invoiceID, status, paidAt)
	return err
}

func (s *Store) ClearPayments(ctx context.Context, invoiceID string) error {
	_, err := s.Pool.Exec(ctx, `UPDATE invoices SET payments='[]', status='unpaid', paid_at=NULL WHERE id=$1`, invoiceID)
	return err
}

func chargeAmount(inv *models.Invoice, name string) int {
	switch name {
	case "Base Rent":
		return inv.BaseRent
	case "Other Charges":
		return inv.OtherCharges
	default:
		return inv.Utilities[name]
	}
}

// RemovePaidItem undoes marking a single charge line as paid, splitting
// or dropping payments that covered it and recomputing the invoice's
// paid/unpaid status — mirrors the Flutter client's prior Firestore logic.
func (s *Store) RemovePaidItem(ctx context.Context, invoiceID, itemName string) (*models.Invoice, error) {
	inv, err := s.GetInvoice(ctx, invoiceID)
	if err != nil {
		return nil, err
	}

	itemAmount := chargeAmount(inv, itemName)
	updatedPayments := []models.Payment{}
	for _, p := range inv.Payments {
		hasItem := false
		for _, it := range p.PaidItems {
			if it == itemName {
				hasItem = true
				break
			}
		}
		if !hasItem {
			updatedPayments = append(updatedPayments, p)
			continue
		}
		remainingItems := make([]string, 0, len(p.PaidItems))
		for _, it := range p.PaidItems {
			if it != itemName {
				remainingItems = append(remainingItems, it)
			}
		}
		if len(remainingItems) > 0 {
			p.PaidItems = remainingItems
			p.Amount -= itemAmount
			updatedPayments = append(updatedPayments, p)
		}
	}

	paidAmount := 0
	for _, p := range updatedPayments {
		paidAmount += p.Amount
	}
	status := "unpaid"
	var paidAt interface{}
	if paidAmount >= inv.TotalAmount {
		status = "paid"
	}

	row := s.Pool.QueryRow(ctx, `
		UPDATE invoices SET payments=$2, status=$3, paid_at=$4 WHERE id=$1 RETURNING `+invoiceCols,
		invoiceID, toJSON(updatedPayments), status, paidAt)
	return scanInvoice(row)
}

func (s *Store) DeleteInvoice(ctx context.Context, id string) error {
	_, err := s.Pool.Exec(ctx, `DELETE FROM invoices WHERE id = $1`, id)
	return err
}

func orDefaultStr(s, def string) string {
	if s == "" {
		return def
	}
	return s
}
