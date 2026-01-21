package relay

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"
)

// Node represents a node in the system
type Node struct {
	ID              string
	Token           string
	OwnerID         *string
	Hostname        *string
	MACAddresses    *string
	CreatedAt       time.Time
	AuthorizedAt    *time.Time
	LastConnectedAt *time.Time
}

// table_node provides CRUD operations for nodes
type table_node struct {
	db *sql.DB
}

// NewNodeTable creates a new node table accessor
func NewNodeTable(db *sql.DB) *table_node {
	return &table_node{db: db}
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
func (t *table_node) AuthorizeNode(nodeID string, ownerID string, hostname, token string) error {
	// Check if node exists
	var existingID string
	err := t.db.QueryRow("SELECT id FROM nodes WHERE id = ?", nodeID).Scan(&existingID)

	if err == sql.ErrNoRows {
		// Node doesn't exist, insert it
		_, err = t.db.Exec(
			"INSERT INTO nodes (id, token, owner_id, hostname, authorized_at) VALUES (?, ?, ?, ?, ?)",
			nodeID, token, ownerID, hostname, time.Now(),
		)
		if err != nil {
			return fmt.Errorf("insert node: %w", err)
		}
	} else if err != nil {
		return fmt.Errorf("check node existence: %w", err)
	} else {
		// Node exists, update it
		_, err = t.db.Exec(
			"UPDATE nodes SET token = ?, owner_id = ?, hostname = ?, authorized_at = ? WHERE id = ?",
			token, ownerID, hostname, time.Now(), nodeID,
		)
		if err != nil {
			return fmt.Errorf("update node: %w", err)
		}
	}

	// Check if junction table entry exists
	var existingRole string
	err = t.db.QueryRow("SELECT role FROM nodes_users WHERE node_id = ? AND user_id = ?", nodeID, ownerID).Scan(&existingRole)

	if err == sql.ErrNoRows {
		// Junction entry doesn't exist, insert it
		_, err = t.db.Exec(
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

	log.Printf("[table_node] Node %s authorized by user %s (hostname=%q)", nodeID, ownerID, hostname)
	return nil
}

// GetNodeByToken retrieves a node by its authorization token
func (t *table_node) GetNodeByToken(token string) (*Node, error) {
	var node Node
	var ownerID sql.NullString
	var hostname sql.NullString
	var macAddresses sql.NullString
	var authorizedAt, lastConnected sql.NullTime

	err := t.db.QueryRow(`
		SELECT id, token, owner_id, hostname, mac_addresses, created_at, authorized_at, last_connected_at
		FROM nodes WHERE token = ?
	`, token).Scan(
		&node.ID, &node.Token, &ownerID, &hostname, &macAddresses, &node.CreatedAt,
		&authorizedAt, &lastConnected,
	)

	if err == sql.ErrNoRows {
		return nil, errors.New("invalid token")
	}
	if err != nil {
		return nil, err
	}

	if ownerID.Valid {
		node.OwnerID = &ownerID.String
	}
	if hostname.Valid {
		node.Hostname = &hostname.String
	}
	if macAddresses.Valid {
		node.MACAddresses = &macAddresses.String
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
func (t *table_node) GetNodeByID(nodeID string) (*Node, error) {
	var node Node
	var ownerID sql.NullString
	var hostname sql.NullString
	var macAddresses sql.NullString
	var authorizedAt, lastConnected sql.NullTime

	err := t.db.QueryRow(`
		SELECT id, token, owner_id, hostname, mac_addresses, created_at, authorized_at, last_connected_at
		FROM nodes WHERE id = ?
	`, nodeID).Scan(
		&node.ID, &node.Token, &ownerID, &hostname, &macAddresses, &node.CreatedAt,
		&authorizedAt, &lastConnected,
	)

	if err == sql.ErrNoRows {
		return nil, errors.New("node not found")
	}
	if err != nil {
		return nil, err
	}

	if ownerID.Valid {
		node.OwnerID = &ownerID.String
	}
	if hostname.Valid {
		node.Hostname = &hostname.String
	}
	if macAddresses.Valid {
		node.MACAddresses = &macAddresses.String
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
func (t *table_node) GetNodesByUser(userID string) ([]*Node, error) {
	rows, err := t.db.Query(`
		SELECT id, token, owner_id, hostname, mac_addresses, created_at, authorized_at, last_connected_at
		FROM nodes WHERE owner_id = ?
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var nodes []*Node
	for rows.Next() {
		var node Node
		var ownerID sql.NullString
		var hostname sql.NullString
		var macAddresses sql.NullString
		var authorizedAt, lastConnected sql.NullTime

		err := rows.Scan(
			&node.ID, &node.Token, &ownerID, &hostname, &macAddresses, &node.CreatedAt,
			&authorizedAt, &lastConnected,
		)
		if err != nil {
			return nil, err
		}

		if ownerID.Valid {
			node.OwnerID = &ownerID.String
		}
		if hostname.Valid {
			node.Hostname = &hostname.String
		}
		if macAddresses.Valid {
			node.MACAddresses = &macAddresses.String
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

// UpdateLastConnected updates the last connected timestamp and stores system info (hostname, MACs)
func (t *table_node) UpdateLastConnected(nodeID, hostname string, macAddresses []string) error {
	// Convert MAC addresses to JSON string
	macJSON := "[]"
	if len(macAddresses) > 0 {
		bytes, err := json.Marshal(macAddresses)
		if err == nil {
			macJSON = string(bytes)
		}
	}

	log.Printf("[table_node] UpdateLastConnected: node=%s hostname=%q macs=%v", nodeID, hostname, macAddresses)

	_, err := t.db.Exec(
		"UPDATE nodes SET last_connected_at = ?, hostname = ?, mac_addresses = ? WHERE id = ?",
		time.Now(), hostname, macJSON, nodeID,
	)
	return err
}

// DeleteNode removes a node (unauthorizes it)
func (t *table_node) DeleteNode(nodeID string) error {
	_, err := t.db.Exec("DELETE FROM nodes WHERE id = ?", nodeID)
	return err
}

// UserOwnsNode checks if a user owns a specific node
func (t *table_node) UserOwnsNode(userID string, nodeID string) bool {
	var ownerID sql.NullString
	err := t.db.QueryRow("SELECT owner_id FROM nodes WHERE id = ?", nodeID).Scan(&ownerID)
	if err != nil {
		return false
	}
	return ownerID.Valid && ownerID.String == userID
}
