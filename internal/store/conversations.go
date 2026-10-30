package store

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/sofolitltd/barivara-server/internal/models"
)

func (s *Store) GetOrCreateConversation(ctx context.Context, landlordID, renterID string) (*models.Conversation, error) {
	var conversation models.Conversation
	err := s.Pool.QueryRow(ctx, `
		INSERT INTO conversations (landlord_id, renter_id, property_id, unit_id)
		SELECT $1, r.id, r.property_id, r.unit_id
		FROM renters r
		JOIN properties p ON p.id = r.property_id
		WHERE r.id = $2 AND r.is_active = true AND p.owner_id = $1
		ON CONFLICT (landlord_id, renter_id) DO UPDATE SET updated_at = conversations.updated_at
		RETURNING id, landlord_id, renter_id, property_id, unit_id, created_at, updated_at`,
		landlordID, renterID,
	).Scan(&conversation.ID, &conversation.LandlordID, &conversation.RenterID, &conversation.PropertyID,
		&conversation.UnitID, &conversation.CreatedAt, &conversation.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return s.GetConversation(ctx, landlordID, conversation.ID)
}

func (s *Store) GetConversation(ctx context.Context, userID, conversationID string) (*models.Conversation, error) {
	var c models.Conversation
	err := s.Pool.QueryRow(ctx, `
		SELECT c.id, c.landlord_id, c.renter_id, r.name, c.property_id, p.name, c.unit_id, u.unit_number,
		       c.created_at, c.updated_at, coalesce(m.body, ''), m.created_at,
		       (SELECT count(*) FROM conversation_messages unread
		        WHERE unread.conversation_id = c.id AND unread.sender_user_id <> $1 AND unread.read_at IS NULL)::int
		FROM conversations c
		JOIN renters r ON r.id = c.renter_id
		JOIN properties p ON p.id = c.property_id
		JOIN units u ON u.id = c.unit_id
		LEFT JOIN LATERAL (
			SELECT body, created_at FROM conversation_messages
			WHERE conversation_id = c.id ORDER BY created_at DESC LIMIT 1
		) m ON true
		WHERE c.id = $2 AND (c.landlord_id = $1 OR r.linked_user_id = $1)`,
		userID, conversationID,
	).Scan(&c.ID, &c.LandlordID, &c.RenterID, &c.RenterName, &c.PropertyID, &c.PropertyName,
		&c.UnitID, &c.UnitNumber, &c.CreatedAt, &c.UpdatedAt, &c.LastMessage, &c.LastAt, &c.UnreadCount)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &c, err
}

func (s *Store) ListLandlordConversations(ctx context.Context, landlordID string) ([]models.Conversation, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT c.id, c.landlord_id, c.renter_id, r.name, c.property_id, p.name, c.unit_id, u.unit_number,
		       c.created_at, c.updated_at, coalesce(m.body, ''), m.created_at,
		       (SELECT count(*) FROM conversation_messages unread
		        WHERE unread.conversation_id = c.id AND unread.sender_user_id <> $1 AND unread.read_at IS NULL)::int
		FROM conversations c
		JOIN renters r ON r.id = c.renter_id
		JOIN properties p ON p.id = c.property_id
		JOIN units u ON u.id = c.unit_id
		LEFT JOIN LATERAL (
			SELECT body, created_at FROM conversation_messages
			WHERE conversation_id = c.id ORDER BY created_at DESC LIMIT 1
		) m ON true
		WHERE c.landlord_id = $1 ORDER BY coalesce(m.created_at, c.updated_at) DESC`, landlordID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return collectConversations(rows)
}

func (s *Store) GetTenantConversation(ctx context.Context, tenantUserID string) (*models.Conversation, error) {
	var id string
	err := s.Pool.QueryRow(ctx, `
		SELECT c.id FROM conversations c JOIN renters r ON r.id = c.renter_id
		WHERE r.linked_user_id = $1 ORDER BY c.updated_at DESC LIMIT 1`, tenantUserID).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		renterID := ""
		if err := s.Pool.QueryRow(ctx, `SELECT id FROM renters WHERE linked_user_id = $1 AND is_active = true LIMIT 1`, tenantUserID).Scan(&renterID); err == nil {
			var landlordID string
			if err := s.Pool.QueryRow(ctx, `SELECT owner_id FROM properties WHERE id = (SELECT property_id FROM renters WHERE id = $1)`, renterID).Scan(&landlordID); err == nil {
				return s.GetOrCreateConversation(ctx, landlordID, renterID)
			}
		}
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return s.GetConversation(ctx, tenantUserID, id)
}

func collectConversations(rows pgx.Rows) ([]models.Conversation, error) {
	out := []models.Conversation{}
	for rows.Next() {
		var c models.Conversation
		if err := rows.Scan(&c.ID, &c.LandlordID, &c.RenterID, &c.RenterName, &c.PropertyID, &c.PropertyName,
			&c.UnitID, &c.UnitNumber, &c.CreatedAt, &c.UpdatedAt, &c.LastMessage, &c.LastAt, &c.UnreadCount); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, nil
}

func (s *Store) SendConversationMessage(ctx context.Context, userID, conversationID, body string) (*models.ConversationMessage, error) {
	body = strings.TrimSpace(body)
	if body == "" {
		return nil, errors.New("message is required")
	}
	if _, err := s.GetConversation(ctx, userID, conversationID); err != nil {
		return nil, err
	}
	var message models.ConversationMessage
	err := s.Pool.QueryRow(ctx, `
		INSERT INTO conversation_messages (conversation_id, sender_user_id, body)
		VALUES ($1,$2,$3)
		RETURNING id, conversation_id, sender_user_id, body, created_at, read_at`,
		conversationID, userID, body,
	).Scan(&message.ID, &message.ConversationID, &message.SenderUserID, &message.Body,
		&message.CreatedAt, &message.ReadAt)
	if err == nil {
		_, _ = s.Pool.Exec(ctx, `UPDATE conversations SET updated_at = now() WHERE id = $1`, conversationID)
	}
	return &message, err
}

func (s *Store) ListConversationMessages(ctx context.Context, userID, conversationID string) ([]models.ConversationMessage, error) {
	if _, err := s.GetConversation(ctx, userID, conversationID); err != nil {
		return nil, err
	}
	rows, err := s.Pool.Query(ctx, `
		SELECT id, conversation_id, sender_user_id, body, created_at, read_at
		FROM conversation_messages WHERE conversation_id = $1
		ORDER BY created_at ASC`, conversationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	messages := []models.ConversationMessage{}
	for rows.Next() {
		var m models.ConversationMessage
		if err := rows.Scan(&m.ID, &m.ConversationID, &m.SenderUserID, &m.Body, &m.CreatedAt, &m.ReadAt); err != nil {
			return nil, err
		}
		messages = append(messages, m)
	}
	_, _ = s.Pool.Exec(ctx, `
		UPDATE conversation_messages SET read_at = now()
		WHERE conversation_id = $1 AND sender_user_id <> $2 AND read_at IS NULL`, conversationID, userID)
	return messages, nil
}
