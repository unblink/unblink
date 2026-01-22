package relay

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// StorageDB represents a stored item (frame or video) in the database
type StorageDB struct {
	ID          string
	ServiceID   string
	Type        string // "frame" or "video"
	StoragePath string
	Timestamp   time.Time
	FileSize    int64
	ContentType string
	CreatedAt   time.Time
	Metadata    map[string]interface{} // JSON
}

// table_storage provides CRUD operations for the unified storage table
type table_storage struct {
	db *Database
}

// NewStorageTable creates a new storage table accessor
func NewStorageTable(db *Database) *table_storage {
	return &table_storage{db: db}
}

// CreateStorage creates a new storage record
func (t *table_storage) CreateStorage(id, serviceID, storageType, storagePath string, timestamp time.Time, fileSize int64, contentType string, metadata map[string]interface{}) error {
	var metadataJSON sql.NullString
	if metadata != nil {
		metadataBytes, err := json.Marshal(metadata)
		if err != nil {
			return fmt.Errorf("failed to marshal metadata: %w", err)
		}
		metadataJSON = sql.NullString{String: string(metadataBytes), Valid: true}
	}

	_, err := t.db.Exec(`
		INSERT INTO storage (id, service_id, type, storage_path, timestamp, file_size, content_type, created_at, metadata)
		VALUES (?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, ?)
	`, id, serviceID, storageType, storagePath, timestamp, fileSize, contentType, metadataJSON)

	if err != nil {
		return fmt.Errorf("failed to create storage: %w", err)
	}

	return nil
}

// GetStorage retrieves a storage record by ID
func (t *table_storage) GetStorage(id string) (*StorageDB, error) {
	var storage StorageDB
	var metadataJSON sql.NullString

	err := t.db.QueryRow(`
		SELECT id, service_id, type, storage_path, timestamp, file_size, content_type, created_at, metadata
		FROM storage
		WHERE id = ?
	`, id).Scan(
		&storage.ID, &storage.ServiceID, &storage.Type, &storage.StoragePath,
		&storage.Timestamp, &storage.FileSize, &storage.ContentType, &storage.CreatedAt, &metadataJSON,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to get storage: %w", err)
	}

	if metadataJSON.Valid {
		if err := json.Unmarshal([]byte(metadataJSON.String), &storage.Metadata); err != nil {
			return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
		}
	}

	return &storage, nil
}

// ListStorageByService retrieves storage records for a service
// If serviceID is empty, returns all storage records (optionally filtered by type)
func (t *table_storage) ListStorageByService(serviceID string, storageType string, limit int) ([]*StorageDB, error) {
	if limit <= 0 {
		limit = 100
	}

	var query string
	var args []interface{}

	// Build WHERE clause conditions
	whereConditions := []string{}
	queryArgs := []interface{}{}

	if serviceID != "" {
		whereConditions = append(whereConditions, "service_id = ?")
		queryArgs = append(queryArgs, serviceID)
	}

	if storageType != "" {
		whereConditions = append(whereConditions, "type = ?")
		queryArgs = append(queryArgs, storageType)
	}

	whereClause := ""
	if len(whereConditions) > 0 {
		whereClause = "WHERE " + fmt.Sprintf("%s", whereConditions[0])
		for i := 1; i < len(whereConditions); i++ {
			whereClause += " AND " + whereConditions[i]
		}
	}

	queryArgs = append(queryArgs, limit)
	args = queryArgs

	query = `
		SELECT id, service_id, type, storage_path, timestamp, file_size, content_type, created_at, metadata
		FROM storage
		` + whereClause + `
		ORDER BY timestamp DESC
		LIMIT ?
	`

	rows, err := t.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list storage: %w", err)
	}
	defer rows.Close()

	var items []*StorageDB
	for rows.Next() {
		var storage StorageDB
		var metadataJSON sql.NullString

		err := rows.Scan(
			&storage.ID, &storage.ServiceID, &storage.Type, &storage.StoragePath,
			&storage.Timestamp, &storage.FileSize, &storage.ContentType, &storage.CreatedAt, &metadataJSON,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan storage: %w", err)
		}

		if metadataJSON.Valid {
			if err := json.Unmarshal([]byte(metadataJSON.String), &storage.Metadata); err != nil {
				return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
			}
		}

		items = append(items, &storage)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating storage: %w", err)
	}

	return items, nil
}

// UpdateStorage updates a storage record's file_size and metadata
func (t *table_storage) UpdateStorage(id string, fileSize int64, metadata map[string]interface{}) error {
	var metadataJSON sql.NullString
	if metadata != nil {
		metadataBytes, err := json.Marshal(metadata)
		if err != nil {
			return fmt.Errorf("failed to marshal metadata: %w", err)
		}
		metadataJSON = sql.NullString{String: string(metadataBytes), Valid: true}
	}

	_, err := t.db.Exec(`
		UPDATE storage
		SET file_size = ?, metadata = ?
		WHERE id = ?
	`, fileSize, metadataJSON, id)

	if err != nil {
		return fmt.Errorf("failed to update storage: %w", err)
	}

	return nil
}

// DeleteStorage deletes a storage record by ID
func (t *table_storage) DeleteStorage(id string) error {
	_, err := t.db.Exec(`DELETE FROM storage WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("failed to delete storage: %w", err)
	}
	return nil
}

// DeleteStorageByService deletes all storage records for a service
func (t *table_storage) DeleteStorageByService(serviceID string) error {
	_, err := t.db.Exec(`DELETE FROM storage WHERE service_id = ?`, serviceID)
	if err != nil {
		return fmt.Errorf("failed to delete storage for service: %w", err)
	}
	return nil
}

// DeleteStorageByServiceAndType deletes all storage records for a service of a specific type
func (t *table_storage) DeleteStorageByServiceAndType(serviceID, storageType string) error {
	_, err := t.db.Exec(`DELETE FROM storage WHERE service_id = ? AND type = ?`, serviceID, storageType)
	if err != nil {
		return fmt.Errorf("failed to delete storage for service and type: %w", err)
	}
	return nil
}
