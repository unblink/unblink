package database

import (
	"database/sql"
	"fmt"
)

// AssociateUserNode adds a user-node association (makes node private to that user)
func (c *Client) AssociateUserNode(userID, nodeID string) error {
	insertSQL := `
		INSERT INTO user_node (user_id, node_id)
		VALUES ($1, $2)
		ON CONFLICT (user_id, node_id) DO NOTHING
	`

	_, err := c.db.Exec(insertSQL, userID, nodeID)
	if err != nil {
		return fmt.Errorf("failed to associate user with node: %w", err)
	}

	return nil
}

// DissociateUserNode removes user-node association
func (c *Client) DissociateUserNode(userID, nodeID string) error {
	deleteSQL := `DELETE FROM user_node WHERE user_id = $1 AND node_id = $2`

	result, err := c.db.Exec(deleteSQL, userID, nodeID)
	if err != nil {
		return fmt.Errorf("failed to dissociate user from node: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check rows affected: %w", err)
	}
	if rows == 0 {
		return sql.ErrNoRows
	}

	return nil
}
