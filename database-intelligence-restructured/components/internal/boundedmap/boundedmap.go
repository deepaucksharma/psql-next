package boundedmap

import (
	"container/list"
	"sync"
	"time"
)

// BoundedMap is a thread-safe map with size limits and LRU eviction
type BoundedMap struct {
	data     map[string]interface{}
	lru      *list.List
	maxSize  int
	mu       sync.RWMutex
	onEvict  func(key string, value interface{})
}

// Item represents an item in the map
type Item struct {
	key       string
	value     interface{}
	lastUsed  time.Time
}

// New creates a new bounded map
func New(maxSize int, onEvict func(key string, value interface{})) *BoundedMap {
	return &BoundedMap{
		data:    make(map[string]interface{}),
		lru:     list.New(),
		maxSize: maxSize,
		onEvict: onEvict,
	}
}

// Put adds or updates an item in the map
func (bm *BoundedMap) Put(key string, value interface{}) {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	// Check if key exists
	if elem, exists := bm.data[key]; exists {
		// Update existing item
		item := elem.(*list.Element).Value.(*Item)
		item.value = value
		item.lastUsed = time.Now()
		bm.lru.MoveToFront(elem.(*list.Element))
		return
	}

	// Add new item
	item := &Item{
		key:      key,
		value:    value,
		lastUsed: time.Now(),
	}
	elem := bm.lru.PushFront(item)
	bm.data[key] = elem

	// Check size limit
	if bm.lru.Len() > bm.maxSize {
		bm.evictOldest()
	}
}

// Get retrieves an item from the map
func (bm *BoundedMap) Get(key string) (interface{}, bool) {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	if elem, exists := bm.data[key]; exists {
		item := elem.(*list.Element).Value.(*Item)
		item.lastUsed = time.Now()
		bm.lru.MoveToFront(elem.(*list.Element))
		return item.value, true
	}
	return nil, false
}

// Delete removes an item from the map
func (bm *BoundedMap) Delete(key string) {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	if elem, exists := bm.data[key]; exists {
		bm.removeElement(elem.(*list.Element))
	}
}

// Len returns the current size of the map
func (bm *BoundedMap) Len() int {
	bm.mu.RLock()
	defer bm.mu.RUnlock()
	return bm.lru.Len()
}

// Clear removes all items from the map
func (bm *BoundedMap) Clear() {
	bm.mu.Lock()
	defer bm.mu.Unlock()
	
	bm.data = make(map[string]interface{})
	bm.lru.Init()
}

// evictOldest removes the least recently used item
func (bm *BoundedMap) evictOldest() {
	elem := bm.lru.Back()
	if elem != nil {
		bm.removeElement(elem)
	}
}

// removeElement removes an element from the map and list
func (bm *BoundedMap) removeElement(elem *list.Element) {
	item := elem.Value.(*Item)
	delete(bm.data, item.key)
	bm.lru.Remove(elem)
	
	if bm.onEvict != nil {
		bm.onEvict(item.key, item.value)
	}
}

// CleanupOlderThan removes items older than the specified duration
func (bm *BoundedMap) CleanupOlderThan(age time.Duration) int {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	cutoff := time.Now().Add(-age)
	removed := 0

	// Iterate from back (oldest) to front (newest)
	var next *list.Element
	for elem := bm.lru.Back(); elem != nil; elem = next {
		next = elem.Prev()
		item := elem.Value.(*Item)
		
		if item.lastUsed.Before(cutoff) {
			bm.removeElement(elem)
			removed++
		} else {
			// Items are ordered by last use, so we can stop here
			break
		}
	}

	return removed
}