package relay

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"connectrpc.com/connect"
	servicev1 "github.com/unblink/unblink/relay/gen/unblink/service/v1"
)

// ServiceService implements the ServiceServiceHandler interface
type ServiceService struct {
	relay *Relay
}

func NewServiceService(relay *Relay) *ServiceService {
	return &ServiceService{relay: relay}
}

// ListServices returns all services owned by the authenticated user
func (s *ServiceService) ListServices(
	ctx context.Context,
	req *connect.Request[servicev1.ListServicesRequest],
) (*connect.Response[servicev1.ListServicesResponse], error) {
	// Get user ID from context (set by auth interceptor)
	userID, ok := GetUserIDFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated,
			fmt.Errorf("not authenticated"))
	}

	services, err := s.relay.ServiceTable.GetServicesByOwner(userID)
	if err != nil {
		log.Printf("[ServiceService] Error getting services: %v", err)
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	protoServices := make([]*servicev1.Service, len(services))
	for i, svc := range services {
		protoServices[i] = toProtoService(svc)
	}

	return connect.NewResponse(&servicev1.ListServicesResponse{
		Services: protoServices,
	}), nil
}

// CreateService creates a new service
func (s *ServiceService) CreateService(
	ctx context.Context,
	req *connect.Request[servicev1.CreateServiceRequest],
) (*connect.Response[servicev1.CreateServiceResponse], error) {
	// Get user ID from context (set by auth interceptor)
	userID, ok := GetUserIDFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated,
			fmt.Errorf("not authenticated"))
	}

	// Validate request
	if req.Msg.Name == "" || req.Msg.ServiceUrl == "" || req.Msg.NodeId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument,
			fmt.Errorf("missing required fields (name, service_url, node_id)"))
	}

	// Verify node ownership
	if !s.relay.NodeTable.UserOwnsNode(userID, req.Msg.NodeId) {
		return nil, connect.NewError(connect.CodeNotFound,
			fmt.Errorf("node not found"))
	}

	// Create service
	createReq := &CreateServiceRequest{
		Name:        req.Msg.Name,
		Description: req.Msg.Description,
		NodeID:      req.Msg.NodeId,
		ServiceURL:  req.Msg.ServiceUrl,
		Tags:        req.Msg.Tags,
	}

	service, err := s.relay.ServiceTable.CreateService(createReq, userID)
	if err != nil {
		log.Printf("[ServiceService] Error creating service: %v", err)
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	log.Printf("[ServiceService] Created service %s for node %s", service.ID, service.NodeID)

	// Notify AutoStreamManager that a service has been created
	if s.relay.AutoStreamMgr != nil {
		go s.relay.AutoStreamMgr.OnServiceCreated(service.ID, service.NodeID)
	}

	return connect.NewResponse(&servicev1.CreateServiceResponse{
		Success: true,
		Id:      service.ID,
		Service: toProtoService(service),
	}), nil
}

// GetService returns a specific service
func (s *ServiceService) GetService(
	ctx context.Context,
	req *connect.Request[servicev1.GetServiceRequest],
) (*connect.Response[servicev1.GetServiceResponse], error) {
	serviceID := req.Msg.ServiceId

	// Validate request
	if serviceID == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument,
			fmt.Errorf("service_id is required"))
	}

	service, err := s.relay.ServiceTable.GetService(serviceID)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound,
			fmt.Errorf("service not found"))
	}

	// Verify ownership
	userID, ok := GetUserIDFromContext(ctx)
	if !ok || service.OwnerID != userID {
		return nil, connect.NewError(connect.CodeNotFound,
			fmt.Errorf("service not found"))
	}

	return connect.NewResponse(&servicev1.GetServiceResponse{
		Service: toProtoService(service),
	}), nil
}

// UpdateService updates a service
func (s *ServiceService) UpdateService(
	ctx context.Context,
	req *connect.Request[servicev1.UpdateServiceRequest],
) (*connect.Response[servicev1.UpdateServiceResponse], error) {
	serviceID := req.Msg.ServiceId

	// Validate request
	if serviceID == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument,
			fmt.Errorf("service_id is required"))
	}

	userID, ok := GetUserIDFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated,
			fmt.Errorf("not authenticated"))
	}

	// Build update request
	updateReq := &UpdateServiceRequest{}

	// In proto3, fields without optional are still optional in practice
	// We check if non-empty values are provided
	if req.Msg.Name != "" {
		updateReq.Name = &req.Msg.Name
	}
	if req.Msg.Description != "" {
		updateReq.Description = &req.Msg.Description
	}
	if req.Msg.ServiceUrl != "" {
		updateReq.ServiceURL = &req.Msg.ServiceUrl
	}
	if req.Msg.Status != "" {
		updateReq.Status = &req.Msg.Status
	}
	// Tags handling: since proto3 doesn't have optional for repeated fields,
	// we need a different approach. We'll skip tags update for now as it requires
	// additional logic to distinguish between "not provided" and "empty array"

	// Verify ownership and update
	service, err := s.relay.ServiceTable.UpdateService(serviceID, updateReq, userID)
	if err != nil {
		if err.Error() == "access denied" {
			return nil, connect.NewError(connect.CodeNotFound,
				fmt.Errorf("service not found"))
		}
		log.Printf("[ServiceService] Error updating service: %v", err)
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	log.Printf("[ServiceService] Updated service %s", serviceID)

	// Notify AutoStreamManager that a service has been updated
	if s.relay.AutoStreamMgr != nil {
		go s.relay.AutoStreamMgr.OnServiceUpdated(serviceID, service.NodeID)
	}

	return connect.NewResponse(&servicev1.UpdateServiceResponse{
		Service: toProtoService(service),
	}), nil
}

// DeleteService deletes a service
func (s *ServiceService) DeleteService(
	ctx context.Context,
	req *connect.Request[servicev1.DeleteServiceRequest],
) (*connect.Response[servicev1.DeleteServiceResponse], error) {
	serviceID := req.Msg.ServiceId

	// Validate request
	if serviceID == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument,
			fmt.Errorf("service_id is required"))
	}

	userID, ok := GetUserIDFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated,
			fmt.Errorf("not authenticated"))
	}

	err := s.relay.ServiceTable.DeleteService(serviceID, userID)
	if err != nil {
		if err.Error() == "access denied" {
			return nil, connect.NewError(connect.CodeNotFound,
				fmt.Errorf("service not found"))
		}
		log.Printf("[ServiceService] Error deleting service: %v", err)
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	log.Printf("[ServiceService] Deleted service %s", serviceID)

	// Remove the service ID from all agents that reference it
	if err := s.relay.AgentTable.RemoveServiceIDFromAllAgents(serviceID); err != nil {
		log.Printf("[ServiceService] Error removing service ID from agents: %v", err)
		// Non-fatal: continue with the deletion response
	}

	// Notify AutoStreamManager that a service has been deleted
	if s.relay.AutoStreamMgr != nil {
		go s.relay.AutoStreamMgr.OnServiceDeleted(serviceID)
	}

	return connect.NewResponse(&servicev1.DeleteServiceResponse{}), nil
}

// ListServicesByNode returns services for a specific node
func (s *ServiceService) ListServicesByNode(
	ctx context.Context,
	req *connect.Request[servicev1.ListServicesByNodeRequest],
) (*connect.Response[servicev1.ListServicesByNodeResponse], error) {
	nodeID := req.Msg.NodeId

	// Validate request
	if nodeID == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument,
			fmt.Errorf("node_id is required"))
	}

	services, err := s.relay.ServiceTable.GetServicesByNode(nodeID)
	if err != nil {
		log.Printf("[ServiceService] Error getting services by node: %v", err)
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	protoServices := make([]*servicev1.Service, len(services))
	for i, svc := range services {
		protoServices[i] = toProtoService(svc)
	}

	return connect.NewResponse(&servicev1.ListServicesByNodeResponse{
		Services: protoServices,
	}), nil
}

func toProtoService(svc *Service) *servicev1.Service {
	var tags []string
	if svc.Tags != "" {
		json.Unmarshal([]byte(svc.Tags), &tags)
	}

	// Parse owner_id as int64 for proto
	var ownerID int64
	fmt.Sscanf(svc.OwnerID, "%d", &ownerID)

	return &servicev1.Service{
		Id:          svc.ID,
		Name:        svc.Name,
		Description: svc.Description,
		NodeId:      svc.NodeID,
		ServiceUrl:  svc.ServiceURL,
		Tags:        tags,
		Status:      svc.Status,
		OwnerId:     ownerID,
		CreatedAt:   svc.CreatedAt.Unix(),
		UpdatedAt:   svc.UpdatedAt.Unix(),
	}
}
