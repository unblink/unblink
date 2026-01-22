package relay

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	storagev1 "github.com/unblink/unblink/relay/gen/unblink/storage/v1"
)

// StorageServiceServer implements the StorageService RPC
type StorageServiceServer struct {
	relay *Relay
}

// NewStorageServiceServer creates a new StorageServiceServer
func NewStorageServiceServer(relay *Relay) *StorageServiceServer {
	return &StorageServiceServer{relay: relay}
}

// ListStorage returns storage items (frames and videos)
func (s *StorageServiceServer) ListStorage(
	ctx context.Context,
	req *connect.Request[storagev1.ListStorageRequest],
) (*connect.Response[storagev1.ListStorageResponse], error) {
	// Default and max limit
	limit := int(req.Msg.Limit)
	if limit <= 0 {
		limit = 100
	}
	if limit > 500 {
		limit = 500
	}

	// Query database
	items, err := s.relay.StorageTable.ListStorageByService(req.Msg.ServiceId, req.Msg.Type, limit)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Convert to proto format
	protoItems := make([]*storagev1.Storage, len(items))
	for i, item := range items {
		protoItems[i] = storageToProto(item)
	}

	return connect.NewResponse(&storagev1.ListStorageResponse{Items: protoItems}), nil
}

// GetStorage returns a single storage item by ID
func (s *StorageServiceServer) GetStorage(
	ctx context.Context,
	req *connect.Request[storagev1.GetStorageRequest],
) (*connect.Response[storagev1.GetStorageResponse], error) {
	item, err := s.relay.StorageTable.GetStorage(req.Msg.Id)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}

	return connect.NewResponse(&storagev1.GetStorageResponse{Storage: storageToProto(item)}), nil
}

// storageToProto converts StorageDB to proto Storage
func storageToProto(s *StorageDB) *storagev1.Storage {
	metadata := make(map[string]string)
	if s.Metadata != nil {
		for k, v := range s.Metadata {
			switch val := v.(type) {
			case string:
				metadata[k] = val
			case float64:
				metadata[k] = fmt.Sprintf("%v", val)
			case bool:
				metadata[k] = fmt.Sprintf("%v", val)
			default:
				metadata[k] = ""
			}
		}
	}

	return &storagev1.Storage{
		Id:          s.ID,
		ServiceId:   s.ServiceID,
		Type:        s.Type,
		StoragePath: s.StoragePath,
		Timestamp:   s.Timestamp.Unix(),
		FileSize:    s.FileSize,
		ContentType: s.ContentType,
		Metadata:    metadata,
	}
}
