// internal/storage/leveldb/client.go
package leveldb

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/fawad-mazhar/naxos/internal/config"
	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/opt"
	"github.com/syndtr/goleveldb/leveldb/util"
)

type CacheEntry struct {
	Value     []byte    `json:"value"`
	ExpiresAt time.Time `json:"expiresAt"`
}

type Client struct {
	db              *leveldb.DB
	ttl             time.Duration
	cleanupInterval time.Duration
	mutex           sync.RWMutex
	stopCleanup     chan struct{}
}

func NewClient(cfg config.LevelDBConfig, ttl time.Duration) (*Client, error) {
	opts := &opt.Options{
		CompactionTableSize: 2 * 1024 * 1024, // 2MB
		WriteBuffer:         1 * 1024 * 1024, // 1MB
	}

	db, err := leveldb.OpenFile(cfg.Path, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to open leveldb: %w", err)
	}

	client := &Client{
		db:              db,
		ttl:             ttl,
		cleanupInterval: 6 * time.Hour, // Run cleanup every 6 hours
		stopCleanup:     make(chan struct{}),
	}

	go client.startCleanupRoutine()

	return client, nil
}

func (c *Client) Close() error {
	close(c.stopCleanup)
	return c.db.Close()
}

func (c *Client) Put(key string, value []byte) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	entry := CacheEntry{
		Value:     value,
		ExpiresAt: time.Now().Add(c.ttl),
	}

	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("failed to marshal cache entry: %w", err)
	}

	return c.db.Put([]byte(key), data, nil)
}

func (c *Client) Get(key string) ([]byte, error) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	data, err := c.db.Get([]byte(key), nil)
	if err != nil {
		if err == leveldb.ErrNotFound {
			return nil, nil
		}
		return nil, err
	}

	var entry CacheEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		return nil, fmt.Errorf("failed to unmarshal cache entry: %w", err)
	}

	if time.Now().After(entry.ExpiresAt) {
		// Entry has expired, delete it
		c.mutex.RUnlock()
		c.mutex.Lock()
		c.db.Delete([]byte(key), nil)
		c.mutex.Unlock()
		c.mutex.RLock()
		return nil, nil
	}

	return entry.Value, nil
}

func (c *Client) Delete(key string) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	return c.db.Delete([]byte(key), nil)
}

func (c *Client) startCleanupRoutine() {
	ticker := time.NewTicker(c.cleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.cleanup()
		case <-c.stopCleanup:
			return
		}
	}
}

func (c *Client) cleanup() {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	iter := c.db.NewIterator(util.BytesPrefix([]byte{}), nil)
	defer iter.Release()

	var keysToDelete [][]byte

	for iter.Next() {
		var entry CacheEntry
		if err := json.Unmarshal(iter.Value(), &entry); err != nil {
			continue
		}

		if time.Now().After(entry.ExpiresAt) {
			keysToDelete = append(keysToDelete, iter.Key())
		}
	}

	for _, key := range keysToDelete {
		c.db.Delete(key, nil)
	}
}
