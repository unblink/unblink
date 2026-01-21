package relay

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/unblink/unblink/shared"
)

// Service represents a service that can be reached through a node
type Service struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	NodeID      string    `json:"node_id"`
	ServiceURL  string    `json:"service_url"` // Single URL field (e.g., rtsp://user:pass@host:port/path)
	Tags        string    `json:"tags"`   // JSON array of tags
	Status      string    `json:"status"` // active, inactive, maintenance
	OwnerID     string    `json:"owner_id"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`

	// Runtime cache of parsed URL components (not stored in DB)
	parsedURL *shared.ServiceURL `json:"-"`
}

// Parsed returns the cached or freshly parsed URL components
func (s *Service) Parsed() *shared.ServiceURL {
	if s.parsedURL == nil {
		s.parsedURL, _ = shared.ParseServiceURL(s.ServiceURL)
	}
	return s.parsedURL
}

// Type returns the service type from the URL scheme
func (s *Service) Type() string {
	return s.Parsed().Scheme
}

// Addr returns the host address from the URL
func (s *Service) Addr() string {
	return s.Parsed().Host
}

// Port returns the port from the URL
func (s *Service) Port() int {
	return s.Parsed().Port
}

// Path returns the path from the URL
func (s *Service) Path() string {
	return s.Parsed().Path
}

// AuthUsername returns the username from the URL
func (s *Service) AuthUsername() string {
	return s.Parsed().Username
}

// AuthPassword returns the password from the URL
func (s *Service) AuthPassword() string {
	return s.Parsed().Password
}

// CreateServiceRequest is the request to create a new service
type CreateServiceRequest struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	NodeID      string   `json:"node_id"`
	ServiceURL  string   `json:"service_url"` // Service URL (e.g., rtsp://user:pass@host:port/path)
	Tags        []string `json:"tags,omitempty"`
}

// UpdateServiceRequest is the request to update a service
type UpdateServiceRequest struct {
	Name        *string  `json:"name,omitempty"`
	Description *string  `json:"description,omitempty"`
	ServiceURL  *string  `json:"service_url,omitempty"` // Service URL
	Tags        []string `json:"tags,omitempty"`
	Status      *string  `json:"status,omitempty"`
}

// table_service provides CRUD operations for services
type table_service struct {
	db *Database
}

// NewServiceTable creates a new service table accessor
func NewServiceTable(db *Database) *table_service {
	return &table_service{db: db}
}

// CreateService creates a new service in the database
func (s *table_service) CreateService(req *CreateServiceRequest, ownerID string) (*Service, error) {
	// Validate URL format
	_, err := shared.ParseServiceURL(req.ServiceURL)
	if err != nil {
		return nil, fmt.Errorf("invalid service URL: %w", err)
	}

	serviceID := uuid.New().String()
	now := time.Now()

	// Convert tags to JSON array string
	var tags sql.NullString
	if len(req.Tags) > 0 {
		tagsJSON, err := json.Marshal(req.Tags)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal tags: %w", err)
		}
		tags = sql.NullString{String: string(tagsJSON), Valid: true}
	}

	_, err = s.db.Exec(`
		INSERT INTO services (
			id, name, description, node_id, service_url, tags, status, owner_id,
			created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, serviceID, req.Name, req.Description, req.NodeID, req.ServiceURL, tags, "active", ownerID,
		now, now)
	if err != nil {
		return nil, fmt.Errorf("failed to create service: %w", err)
	}

	return s.GetService(serviceID)
}

// GetService retrieves a service by ID
func (s *table_service) GetService(serviceID string) (*Service, error) {
	var service Service
	var tags sql.NullString

	err := s.db.QueryRow(`
		SELECT id, name, description, node_id, service_url, tags, status, owner_id,
		       created_at, updated_at
		FROM services
		WHERE id = ?
	`, serviceID).Scan(
		&service.ID, &service.Name, &service.Description, &service.NodeID,
		&service.ServiceURL, &tags, &service.Status, &service.OwnerID,
		&service.CreatedAt, &service.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("service not found")
		}
		return nil, fmt.Errorf("failed to get service: %w", err)
	}

	// Parse tags JSON
	service.Tags = tags.String

	return &service, nil
}

// GetServicesByNode retrieves all services for a specific node
func (s *table_service) GetServicesByNode(nodeID string) ([]*Service, error) {
	rows, err := s.db.Query(`
		SELECT id, name, description, node_id, service_url, tags, status, owner_id,
		       created_at, updated_at
		FROM services
		WHERE node_id = ?
		ORDER BY name ASC
	`, nodeID)
	if err != nil {
		return nil, fmt.Errorf("failed to query services by node: %w", err)
	}
	defer rows.Close()

	var services []*Service
	for rows.Next() {
		service, err := s.scanService(rows)
		if err != nil {
			return nil, err
		}
		services = append(services, service)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating services: %w", err)
	}

	return services, nil
}

// GetServicesByOwner retrieves all services owned by a specific user
func (s *table_service) GetServicesByOwner(ownerID string) ([]*Service, error) {
	rows, err := s.db.Query(`
		SELECT id, name, description, node_id, service_url, tags, status, owner_id,
		       created_at, updated_at
		FROM services
		WHERE owner_id = ?
		ORDER BY name ASC
	`, ownerID)
	if err != nil {
		return nil, fmt.Errorf("failed to query services by owner: %w", err)
	}
	defer rows.Close()

	var services []*Service
	for rows.Next() {
		service, err := s.scanService(rows)
		if err != nil {
			return nil, err
		}
		services = append(services, service)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating services: %w", err)
	}

	return services, nil
}

// UpdateService updates an existing service
func (s *table_service) UpdateService(serviceID string, req *UpdateServiceRequest, ownerID string) (*Service, error) {
	// First verify ownership
	existing, err := s.GetService(serviceID)
	if err != nil {
		return nil, err
	}
	if existing.OwnerID != ownerID {
		return nil, fmt.Errorf("access denied")
	}

	// Build update query dynamically
	updates := []string{}
	args := []interface{}{}

	if req.Name != nil {
		updates = append(updates, "name = ?")
		args = append(args, *req.Name)
	}
	if req.Description != nil {
		updates = append(updates, "description = ?")
		args = append(args, *req.Description)
	}
	if req.ServiceURL != nil {
		// Validate URL format
		_, err := shared.ParseServiceURL(*req.ServiceURL)
		if err != nil {
			return nil, fmt.Errorf("invalid service URL: %w", err)
		}
		updates = append(updates, "service_url = ?")
		args = append(args, *req.ServiceURL)
	}
	if req.Tags != nil {
		tagsJSON, err := json.Marshal(req.Tags)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal tags: %w", err)
		}
		updates = append(updates, "tags = ?")
		args = append(args, string(tagsJSON))
	}
	if req.Status != nil {
		updates = append(updates, "status = ?")
		args = append(args, *req.Status)
	}

	if len(updates) == 0 {
		return existing, nil
	}

	updates = append(updates, "updated_at = ?")
	args = append(args, time.Now())

	// Build the query with proper comma separation
	query := "UPDATE services SET " + strings.Join(updates, ", ") + " WHERE id = ?"
	args = append(args, serviceID)

	_, err = s.db.Exec(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to update service: %w", err)
	}

	return s.GetService(serviceID)
}

// DeleteService deletes a service
func (s *table_service) DeleteService(serviceID string, ownerID string) error {
	// Verify ownership first
	existing, err := s.GetService(serviceID)
	if err != nil {
		return err
	}
	if existing.OwnerID != ownerID {
		return fmt.Errorf("access denied")
	}

	_, err = s.db.Exec("DELETE FROM services WHERE id = ?", serviceID)
	if err != nil {
		return fmt.Errorf("failed to delete service: %w", err)
	}

	return nil
}

// scanService scans a row into a Service struct
func (s *table_service) scanService(rows *sql.Rows) (*Service, error) {
	var service Service
	var tags sql.NullString

	err := rows.Scan(
		&service.ID, &service.Name, &service.Description, &service.NodeID,
		&service.ServiceURL, &tags, &service.Status, &service.OwnerID,
		&service.CreatedAt, &service.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	// Parse tags JSON
	service.Tags = tags.String

	return &service, nil
}
