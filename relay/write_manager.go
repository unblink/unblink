package relay

import (
	"log"
	"sync"
	"time"
)

// WriteOp represents a database write operation
type WriteOp struct {
	Func func(*Database) error
}

// WriteManager serializes all database writes through a single goroutine
// This prevents SQLite "database is locked" errors from concurrent writes
type WriteManager struct {
	db    *Database
	queue chan WriteOp
	wg    sync.WaitGroup
	stop  chan struct{}
}

// NewWriteManager creates a new write manager
func NewWriteManager(db *Database) *WriteManager {
	wm := &WriteManager{
		db:    db,
		queue: make(chan WriteOp, 1000), // Buffered channel for backpressure
		stop:  make(chan struct{}),
	}
	wm.wg.Add(1)
	go wm.process()
	return wm
}

// process is the single goroutine that handles all database writes
func (wm *WriteManager) process() {
	defer wm.wg.Done()

	for {
		select {
		case op := <-wm.queue:
			if err := op.Func(wm.db); err != nil {
				log.Printf("[WriteManager] Write operation failed: %v", err)
			}
		case <-wm.stop:
			// Drain remaining writes
			for len(wm.queue) > 0 {
				op := <-wm.queue
				if err := op.Func(wm.db); err != nil {
					log.Printf("[WriteManager] Final write failed: %v", err)
				}
			}
			return
		}
	}
}

// CreateStorage queues a storage creation write
func (wm *WriteManager) CreateStorage(id, serviceID, storageType, storagePath string, timestamp time.Time, fileSize int64, contentType string, metadata map[string]interface{}) {
	wm.queue <- WriteOp{Func: func(db *Database) error {
		return (&table_storage{db: db}).CreateStorage(id, serviceID, storageType, storagePath, timestamp, fileSize, contentType, metadata)
	}}
}

// UpdateStorage queues a storage update write
func (wm *WriteManager) UpdateStorage(id string, fileSize int64, metadata map[string]interface{}) {
	wm.queue <- WriteOp{Func: func(db *Database) error {
		return (&table_storage{db: db}).UpdateStorage(id, fileSize, metadata)
	}}
}

// CreateAgentEvent queues an agent event creation write
func (wm *WriteManager) CreateAgentEvent(agentID string, serviceID string, data, metadata map[string]interface{}, createdAt time.Time) {
	wm.queue <- WriteOp{Func: func(db *Database) error {
		_, err := (&table_agent_event{db: db}).CreateEvent(agentID, serviceID, data, metadata, createdAt)
		return err
	}}
}

// Close stops the write manager gracefully
func (wm *WriteManager) Close() {
	close(wm.stop)
	wm.wg.Wait()
}
