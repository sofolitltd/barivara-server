package store

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/sofolitltd/barivara-server/internal/models"
)

var ErrConversationPending = errors.New("conversation request is pending acceptance")

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
		SELECT c.id, c.conversation_type, c.status, c.landlord_id, c.renter_id, r.name,
		       CASE WHEN c.conversation_type = 'tenant' THEN
		            CASE WHEN c.participant_a_user_id = $1 THEN participant_b.name ELSE participant_a.name END
		            WHEN c.landlord_id = $1 THEN r.name ELSE landlord.name END,
		       coalesce(c.request_sender_user_id = $1, false),
		       c.property_id, p.name, c.unit_id, u.unit_number,
		       c.created_at, c.updated_at, coalesce(m.body, ''), m.created_at,
		       (SELECT count(*) FROM conversation_messages unread
		        WHERE unread.conversation_id = c.id AND unread.sender_user_id <> $1 AND unread.read_at IS NULL)::int
		FROM conversations c
		JOIN renters r ON r.id = c.renter_id
		JOIN properties p ON p.id = c.property_id
		JOIN units u ON u.id = c.unit_id
		JOIN users landlord ON landlord.id = c.landlord_id
		LEFT JOIN users participant_a ON participant_a.id = c.participant_a_user_id
		LEFT JOIN users participant_b ON participant_b.id = c.participant_b_user_id
		LEFT JOIN LATERAL (
			SELECT body, created_at FROM conversation_messages
			WHERE conversation_id = c.id ORDER BY created_at DESC LIMIT 1
		) m ON true
		WHERE c.id = $2 AND (
			(c.conversation_type = 'landlord' AND (c.landlord_id = $1 OR (r.linked_user_id = $1 AND r.is_active = true)))
			OR (c.conversation_type = 'tenant'
			    AND (c.participant_a_user_id = $1 OR c.participant_b_user_id = $1)
			    AND EXISTS (
			        SELECT 1 FROM renters active
			        WHERE active.linked_user_id = $1 AND active.property_id = c.property_id AND active.is_active = true
			    )
			    AND EXISTS (
			        SELECT 1 FROM renters peer
			        WHERE peer.linked_user_id = CASE
			            WHEN c.participant_a_user_id = $1 THEN c.participant_b_user_id
			            ELSE c.participant_a_user_id END
			          AND peer.property_id = c.property_id AND peer.is_active = true
			    ))
		)`,
		userID, conversationID,
	).Scan(&c.ID, &c.Type, &c.Status, &c.LandlordID, &c.RenterID, &c.RenterName, &c.PeerName,
		&c.RequestSentByMe, &c.PropertyID, &c.PropertyName,
		&c.UnitID, &c.UnitNumber, &c.CreatedAt, &c.UpdatedAt, &c.LastMessage, &c.LastAt, &c.UnreadCount)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &c, err
}

func (s *Store) GetConversationPushTarget(ctx context.Context, userID, conversationID string) (*models.ConversationPushTarget, error) {
	if _, err := s.GetConversation(ctx, userID, conversationID); err != nil {
		return nil, err
	}
	var target models.ConversationPushTarget
	err := s.Pool.QueryRow(ctx, `
		SELECT coalesce(recipient.fcm_token, ''), sender.name, coalesce(recipient.name, '')
		FROM conversations c
		JOIN users sender ON sender.id = $2
		LEFT JOIN renters renter ON renter.id = c.renter_id AND renter.is_active = true
		LEFT JOIN users recipient ON recipient.id = CASE
			WHEN c.conversation_type = 'tenant' THEN
				CASE WHEN c.participant_a_user_id = $2
					THEN c.participant_b_user_id ELSE c.participant_a_user_id END
			WHEN c.landlord_id = $2 THEN renter.linked_user_id
			ELSE c.landlord_id
		END
		WHERE c.id = $1 AND c.status = 'accepted'
		  AND (
		      (c.conversation_type = 'landlord'
		       AND (c.landlord_id = $2 OR renter.linked_user_id = $2))
		      OR (c.conversation_type = 'tenant'
		          AND (c.participant_a_user_id = $2 OR c.participant_b_user_id = $2))
		  )`, conversationID, userID).Scan(&target.Token, &target.SenderName, &target.PeerName)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &target, err
}

func (s *Store) ListLandlordConversations(ctx context.Context, landlordID string) ([]models.Conversation, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT c.id, c.conversation_type, c.status, c.landlord_id, c.renter_id, r.name,
		       r.name, coalesce(c.request_sender_user_id = $1, false),
		       c.property_id, p.name, c.unit_id, u.unit_number,
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
		WHERE c.landlord_id = $1 AND c.conversation_type = 'landlord'
		ORDER BY coalesce(m.created_at, c.updated_at) DESC`, landlordID)
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
		WHERE r.linked_user_id = $1 AND r.is_active = true AND c.conversation_type = 'landlord'
		ORDER BY c.updated_at DESC LIMIT 1`, tenantUserID).Scan(&id)
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
		if err := rows.Scan(&c.ID, &c.Type, &c.Status, &c.LandlordID, &c.RenterID, &c.RenterName, &c.PeerName,
			&c.RequestSentByMe, &c.PropertyID, &c.PropertyName,
			&c.UnitID, &c.UnitNumber, &c.CreatedAt, &c.UpdatedAt, &c.LastMessage, &c.LastAt, &c.UnreadCount); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, nil
}

func (s *Store) ListTenantConversations(ctx context.Context, tenantUserID string) ([]models.Conversation, error) {
	landlordConversation, err := s.GetTenantConversation(ctx, tenantUserID)
	if err != nil {
		return nil, err
	}
	rows, err := s.Pool.Query(ctx, `
		SELECT c.id, c.conversation_type, c.status, c.landlord_id, c.renter_id, renter.name,
		       CASE WHEN c.conversation_type = 'tenant' THEN
		            CASE WHEN c.participant_a_user_id = $1 THEN participant_b.name ELSE participant_a.name END
		            ELSE landlord.name END,
		       coalesce(c.request_sender_user_id = $1, false),
		       c.property_id, p.name, c.unit_id, u.unit_number,
		       c.created_at, c.updated_at, coalesce(m.body, ''), m.created_at,
		       (SELECT count(*) FROM conversation_messages unread
		        WHERE unread.conversation_id = c.id AND unread.sender_user_id <> $1 AND unread.read_at IS NULL)::int
		FROM conversations c
		JOIN renters renter ON renter.id = c.renter_id
		JOIN properties p ON p.id = c.property_id
		JOIN units u ON u.id = c.unit_id
		JOIN users landlord ON landlord.id = c.landlord_id
		LEFT JOIN users participant_a ON participant_a.id = c.participant_a_user_id
		LEFT JOIN users participant_b ON participant_b.id = c.participant_b_user_id
		LEFT JOIN LATERAL (
			SELECT body, created_at FROM conversation_messages
			WHERE conversation_id = c.id ORDER BY created_at DESC LIMIT 1
		) m ON true
		WHERE (c.conversation_type = 'landlord' AND c.id = $2)
		   OR (c.conversation_type = 'tenant'
		       AND (c.participant_a_user_id = $1 OR c.participant_b_user_id = $1)
		       AND EXISTS (
		           SELECT 1 FROM renters active
		           WHERE active.linked_user_id = $1 AND active.property_id = c.property_id AND active.is_active = true
		       )
		       AND EXISTS (
		           SELECT 1 FROM renters peer
		           WHERE peer.linked_user_id = CASE
		               WHEN c.participant_a_user_id = $1 THEN c.participant_b_user_id
		               ELSE c.participant_a_user_id END
		             AND peer.property_id = c.property_id AND peer.is_active = true
		       ))
		ORDER BY coalesce(m.created_at, c.updated_at) DESC`, tenantUserID, landlordConversation.ID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return collectConversations(rows)
}

func (s *Store) ListTenantContacts(ctx context.Context, tenantUserID string) ([]models.TenantContact, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT DISTINCT ON (peer.id) peer.id, peer.name, unit.unit_number,
		       coalesce(conversation.id::text, ''),
		       coalesce(conversation.status, ''),
		       coalesce(conversation.request_sender_user_id = $1, false)
		FROM renters mine
		JOIN renters other ON other.property_id = mine.property_id
			AND other.is_active = true AND other.linked_user_id IS NOT NULL
			AND other.linked_user_id <> $1
		JOIN users peer ON peer.id = other.linked_user_id
		JOIN units unit ON unit.id = other.unit_id
		LEFT JOIN LATERAL (
			SELECT c.id, c.status, c.request_sender_user_id
			FROM conversations c
			WHERE c.conversation_type = 'tenant'
			  AND c.property_id = mine.property_id
			  AND c.participant_a_user_id = LEAST($1::uuid, peer.id)
			  AND c.participant_b_user_id = GREATEST($1::uuid, peer.id)
			LIMIT 1
		) conversation ON true
		WHERE mine.linked_user_id = $1 AND mine.is_active = true
		ORDER BY peer.id, unit.unit_number`, tenantUserID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	contacts := []models.TenantContact{}
	for rows.Next() {
		var contact models.TenantContact
		if err := rows.Scan(&contact.UserID, &contact.Name, &contact.UnitNumber,
			&contact.ConversationID, &contact.ConversationStatus, &contact.RequestSentByMe); err != nil {
			return nil, err
		}
		contacts = append(contacts, contact)
	}
	return contacts, rows.Err()
}

func (s *Store) RequestTenantConversation(ctx context.Context, tenantUserID, peerUserID string) (*models.Conversation, error) {
	var conversationID string
	err := s.Pool.QueryRow(ctx, `
		INSERT INTO conversations (
			landlord_id, renter_id, property_id, unit_id, conversation_type,
			participant_a_user_id, participant_b_user_id, request_sender_user_id, status
		)
		SELECT property.owner_id, mine.id, mine.property_id, mine.unit_id, 'tenant',
		       LEAST(mine.linked_user_id, peer.linked_user_id),
		       GREATEST(mine.linked_user_id, peer.linked_user_id),
		       $1, 'pending'
		FROM renters mine
		JOIN properties property ON property.id = mine.property_id
		JOIN renters peer ON peer.property_id = mine.property_id
		WHERE mine.linked_user_id = $1 AND mine.is_active = true
		  AND peer.linked_user_id = $2 AND peer.is_active = true
		  AND $1 <> $2
		LIMIT 1
		ON CONFLICT (property_id, participant_a_user_id, participant_b_user_id)
		WHERE conversation_type = 'tenant'
		DO UPDATE SET
			status = CASE WHEN conversations.status = 'rejected' THEN 'pending' ELSE conversations.status END,
			request_sender_user_id = CASE
				WHEN conversations.status = 'rejected' THEN EXCLUDED.request_sender_user_id
				ELSE conversations.request_sender_user_id END,
			updated_at = CASE WHEN conversations.status = 'rejected' THEN now() ELSE conversations.updated_at END
		RETURNING id`, tenantUserID, peerUserID).Scan(&conversationID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return s.GetConversation(ctx, tenantUserID, conversationID)
}

func (s *Store) SetTenantConversationStatus(ctx context.Context, tenantUserID, conversationID, status string) error {
	result, err := s.Pool.Exec(ctx, `
		UPDATE conversations c SET status = $3, updated_at = now()
		WHERE c.id = $2 AND c.conversation_type = 'tenant' AND c.status = 'pending'
		  AND c.request_sender_user_id <> $1
		  AND (c.participant_a_user_id = $1 OR c.participant_b_user_id = $1)
		  AND EXISTS (
		      SELECT 1 FROM renters active
		      WHERE active.linked_user_id = $1 AND active.property_id = c.property_id AND active.is_active = true
		  )
		  AND EXISTS (
		      SELECT 1 FROM renters peer
		      WHERE peer.linked_user_id = CASE
		          WHEN c.participant_a_user_id = $1 THEN c.participant_b_user_id
		          ELSE c.participant_a_user_id END
		        AND peer.property_id = c.property_id AND peer.is_active = true
		  )`, tenantUserID, conversationID, status)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) SendConversationMessage(ctx context.Context, userID, conversationID, body string) (*models.ConversationMessage, error) {
	body = strings.TrimSpace(body)
	if body == "" {
		return nil, errors.New("message is required")
	}
	conversation, err := s.GetConversation(ctx, userID, conversationID)
	if err != nil {
		return nil, err
	}
	if conversation.Type == "tenant" && conversation.Status != "accepted" {
		return nil, ErrConversationPending
	}
	var message models.ConversationMessage
	err = s.Pool.QueryRow(ctx, `
		INSERT INTO conversation_messages (conversation_id, sender_user_id, body)
		VALUES ($1,$2,$3)
		RETURNING id, conversation_id, sender_user_id, body, created_at, read_at,
		          edited_at, deleted_at IS NOT NULL`,
		conversationID, userID, body,
	).Scan(&message.ID, &message.ConversationID, &message.SenderUserID, &message.Body,
		&message.CreatedAt, &message.ReadAt, &message.EditedAt, &message.DeletedForEveryone)
	if err == nil {
		_, _ = s.Pool.Exec(ctx, `UPDATE conversations SET updated_at = now() WHERE id = $1`, conversationID)
	}
	return &message, err
}

func (s *Store) EditConversationMessage(ctx context.Context, userID, conversationID, messageID, body string) error {
	body = strings.TrimSpace(body)
	if body == "" {
		return errors.New("message is required")
	}
	conversation, err := s.GetConversation(ctx, userID, conversationID)
	if err != nil {
		return err
	}
	if conversation.Type == "tenant" && conversation.Status != "accepted" {
		return ErrConversationPending
	}
	result, err := s.Pool.Exec(ctx, `
		UPDATE conversation_messages
		SET body = $4, edited_at = now()
		WHERE id = $1 AND conversation_id = $2 AND sender_user_id = $3
		  AND deleted_at IS NULL`, messageID, conversationID, userID, body)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return ErrNotFound
	}
	_, err = s.Pool.Exec(ctx, `UPDATE conversations SET updated_at = now() WHERE id = $1`, conversationID)
	return err
}

func (s *Store) DeleteConversationMessage(ctx context.Context, userID, conversationID, messageID, scope string) error {
	conversation, err := s.GetConversation(ctx, userID, conversationID)
	if err != nil {
		return err
	}
	if conversation.Type == "tenant" && conversation.Status != "accepted" {
		return ErrConversationPending
	}

	var result pgconn.CommandTag
	if scope == "me" {
		result, err = s.Pool.Exec(ctx, `
			INSERT INTO conversation_message_hidden (message_id, user_id)
			SELECT id, $3 FROM conversation_messages
			WHERE id = $1 AND conversation_id = $2 AND deleted_at IS NULL
			ON CONFLICT (message_id, user_id) DO NOTHING`,
			messageID, conversationID, userID)
	} else if scope == "everyone" {
		result, err = s.Pool.Exec(ctx, `
			UPDATE conversation_messages
			SET deleted_at = now(), body = ''
			WHERE id = $1 AND conversation_id = $2 AND sender_user_id = $3 AND deleted_at IS NULL`,
			messageID, conversationID, userID)
	} else {
		return errors.New("invalid delete scope")
	}
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return ErrNotFound
	}
	_, err = s.Pool.Exec(ctx, `UPDATE conversations SET updated_at = now() WHERE id = $1`, conversationID)
	return err
}

func (s *Store) ListConversationMessages(ctx context.Context, userID, conversationID string) ([]models.ConversationMessage, error) {
	if _, err := s.GetConversation(ctx, userID, conversationID); err != nil {
		return nil, err
	}
	rows, err := s.Pool.Query(ctx, `
		SELECT message.id, message.conversation_id, message.sender_user_id,
		       CASE WHEN message.deleted_at IS NOT NULL THEN '' ELSE message.body END,
		       message.created_at, message.read_at, message.edited_at,
		       message.deleted_at IS NOT NULL
		FROM conversation_messages message
		WHERE message.conversation_id = $1
		  AND NOT EXISTS (
		      SELECT 1 FROM conversation_message_hidden hidden
		      WHERE hidden.message_id = message.id AND hidden.user_id = $2
		  )
		ORDER BY message.created_at ASC`, conversationID, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	messages := []models.ConversationMessage{}
	for rows.Next() {
		var m models.ConversationMessage
		if err := rows.Scan(&m.ID, &m.ConversationID, &m.SenderUserID, &m.Body,
			&m.CreatedAt, &m.ReadAt, &m.EditedAt, &m.DeletedForEveryone); err != nil {
			return nil, err
		}
		messages = append(messages, m)
	}
	_, _ = s.Pool.Exec(ctx, `
		UPDATE conversation_messages SET read_at = now()
		WHERE conversation_id = $1 AND sender_user_id <> $2 AND read_at IS NULL`, conversationID, userID)
	return messages, nil
}
