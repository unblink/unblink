package relay

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"time"
)

// Node represents a node in the system
type Node struct {
	ID              string
	Token           string
	OwnerID         *int64
	Name            *string
	CreatedAt       time.Time
	AuthorizedAt    *time.Time
	LastConnectedAt *time.Time
}

// NodeTable handles node data access
type NodeTable struct {
	db *sql.DB
}

// NewNodeTable creates a new NodeTable
func NewNodeTable(db *sql.DB) *NodeTable {
	return &NodeTable{db: db}
}

// generateSecureToken creates a cryptographically secure random token
func generateSecureToken() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// AuthorizeNode creates a node with token and links it to a user
func (s *NodeTable) AuthorizeNode(nodeID string, ownerID int64, name, token string) error {
	// Check if node exists
	var existingID string
	err := s.db.QueryRow("SELECT id FROM nodes WHERE id = ?", nodeID).Scan(&existingID)

	if err == sql.ErrNoRows {
		// Node doesn't exist, insert it
		_, err = s.db.Exec(
			"INSERT INTO nodes (id, token, owner_id, name, authorized_at) VALUES (?, ?, ?, ?, ?)",
			nodeID, token, ownerID, name, time.Now(),
		)
		if err != nil {
			return fmt.Errorf("insert node: %w", err)
		}
	} else if err != nil {
		return fmt.Errorf("check node existence: %w", err)
	} else {
		// Node exists, update it
		_, err = s.db.Exec(
			"UPDATE nodes SET token = ?, owner_id = ?, name = ?, authorized_at = ? WHERE id = ?",
			token, ownerID, name, time.Now(), nodeID,
		)
		if err != nil {
			return fmt.Errorf("update node: %w", err)
		}
	}

	// Check if junction table entry exists
	var existingRole string
	err = s.db.QueryRow("SELECT role FROM nodes_users WHERE node_id = ? AND user_id = ?", nodeID, ownerID).Scan(&existingRole)

	if err == sql.ErrNoRows {
		// Junction entry doesn't exist, insert it
		_, err = s.db.Exec(
			"INSERT INTO nodes_users (node_id, user_id, role) VALUES (?, ?, 'owner')",
			nodeID, ownerID,
		)
		if err != nil {
			return fmt.Errorf("insert nodes_users: %w", err)
		}
	} else if err != nil {
		return fmt.Errorf("check nodes_users existence: %w", err)
	}
	// If entry exists, no need to update it

	log.Printf("[NodeTable] Node %s authorized by user %d", nodeID, ownerID)
	return nil
}

// GetNodeByToken retrieves a node by its authorization token
func (s *NodeTable) GetNodeByToken(token string) (*Node, error) {
	var node Node
	var ownerID sql.NullInt64
	var name sql.NullString
	var authorizedAt, lastConnected sql.NullTime

	err := s.db.QueryRow(`
		SELECT id, token, owner_id, name, created_at, authorized_at, last_connected_at
		FROM nodes WHERE token = ?
	`, token).Scan(
		&node.ID, &node.Token, &ownerID, &name, &node.CreatedAt,
		&authorizedAt, &lastConnected,
	)

	if err == sql.ErrNoRows {
		return nil, errors.New("invalid token")
	}
	if err != nil {
		return nil, err
	}

	if ownerID.Valid {
		node.OwnerID = &ownerID.Int64
	}
	if name.Valid {
		node.Name = &name.String
	}
	if authorizedAt.Valid {
		node.AuthorizedAt = &authorizedAt.Time
	}
	if lastConnected.Valid {
		node.LastConnectedAt = &lastConnected.Time
	}

	return &node, nil
}

// GetNodeByID retrieves a node by its ID
func (s *NodeTable) GetNodeByID(nodeID string) (*Node, error) {
	var node Node
	var ownerID sql.NullInt64
	var name sql.NullString
	var authorizedAt, lastConnected sql.NullTime

	err := s.db.QueryRow(`
		SELECT id, token, owner_id, name, created_at, authorized_at, last_connected_at
		FROM nodes WHERE id = ?
	`, nodeID).Scan(
		&node.ID, &node.Token, &ownerID, &name, &node.CreatedAt,
		&authorizedAt, &lastConnected,
	)

	if err == sql.ErrNoRows {
		return nil, errors.New("node not found")
	}
	if err != nil {
		return nil, err
	}

	if ownerID.Valid {
		node.OwnerID = &ownerID.Int64
	}
	if name.Valid {
		node.Name = &name.String
	}
	if authorizedAt.Valid {
		node.AuthorizedAt = &authorizedAt.Time
	}
	if lastConnected.Valid {
		node.LastConnectedAt = &lastConnected.Time
	}

	return &node, nil
}

// GetNodesByUser retrieves all nodes owned by a user
func (s *NodeTable) GetNodesByUser(userID int64) ([]*Node, error) {
	rows, err := s.db.Query(`
		SELECT id, token, owner_id, name, created_at, authorized_at, last_connected_at
		FROM nodes WHERE owner_id = ?
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var nodes []*Node
	for rows.Next() {
		var node Node
		var ownerID sql.NullInt64
		var name sql.NullString
		var authorizedAt, lastConnected sql.NullTime

		err := rows.Scan(
			&node.ID, &node.Token, &ownerID, &name, &node.CreatedAt,
			&authorizedAt, &lastConnected,
		)
		if err != nil {
			return nil, err
		}

		if ownerID.Valid {
			node.OwnerID = &ownerID.Int64
		}
		if name.Valid {
			node.Name = &name.String
		}
		if authorizedAt.Valid {
			node.AuthorizedAt = &authorizedAt.Time
		}
		if lastConnected.Valid {
			node.LastConnectedAt = &lastConnected.Time
		}

		nodes = append(nodes, &node)
	}

	return nodes, nil
}

// UpdateLastConnected updates the last connected timestamp
func (s *NodeTable) UpdateLastConnected(nodeID string) error {
	_, err := s.db.Exec(
		"UPDATE nodes SET last_connected_at = ? WHERE id = ?",
		time.Now(), nodeID,
	)
	return err
}

// UpdateNodeName updates the name of a node
func (s *NodeTable) UpdateNodeName(nodeID string, name string) error {
	_, err := s.db.Exec(
		"UPDATE nodes SET name = ? WHERE id = ?",
		name, nodeID,
	)
	return err
}

// DeleteNode removes a node (unauthorizes it)
func (s *NodeTable) DeleteNode(nodeID string) error {
	_, err := s.db.Exec("DELETE FROM nodes WHERE id = ?", nodeID)
	return err
}

// UserOwnsNode checks if a user owns a specific node
func (s *NodeTable) UserOwnsNode(userID int64, nodeID string) bool {
	var ownerID sql.NullInt64
	err := s.db.QueryRow("SELECT owner_id FROM nodes WHERE id = ?", nodeID).Scan(&ownerID)
	if err != nil {
		return false
	}
	return ownerID.Valid && ownerID.Int64 == userID
}

// EnsureDevMockNode creates a mock node entry for dev mode with a generated token
// Returns the node and the generated token
func (s *NodeTable) EnsureDevMockNode(ownerID int64) (*Node, string, error) {
	nodeID := "mock-dev-1"

	// Check if already exists
	node, err := s.GetNodeByID(nodeID)
	if err == nil {
		// Node exists - return its existing token
		return node, node.Token, nil
	}

	// Generate a random token for the mock node
	devToken, err := generateSecureToken()
	if err != nil {
		return nil, "", fmt.Errorf("generate dev token: %w", err)
	}

	// Create node with generated token
	err = s.AuthorizeNode(nodeID, ownerID, "Dev Mock Node", devToken)
	if err != nil {
		return nil, "", err
	}

	node, err = s.GetNodeByID(nodeID)
	if err != nil {
		return nil, "", err
	}

	return node, devToken, nil
}
