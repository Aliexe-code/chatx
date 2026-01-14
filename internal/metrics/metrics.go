package metrics

import (
	"sync"
	"sync/atomic"
	"time"
)

// Metrics tracks application performance and health metrics
type Metrics struct {
	// Connection metrics
	ActiveConnections   int64
	TotalConnections    int64
	Disconnections      int64

	// Message metrics
	TotalMessages       int64
	MessagesPerSecond   float64
	MessageLatency      int64 // nanoseconds
	MessageErrors       int64

	// Room metrics
	TotalRooms          int64
	RoomOccupancy       map[string]int64

	// Performance metrics
	AverageLatency      int64
	P95Latency          int64
	P99Latency          int64

	// Timing
	StartTime           time.Time
	LastReset           time.Time

	// Thread safety
	Mutex               sync.RWMutex
}

// NewMetrics creates a new metrics instance
func NewMetrics() *Metrics {
	return &Metrics{
		RoomOccupancy: make(map[string]int64),
		StartTime:    time.Now(),
		LastReset:    time.Now(),
	}
}

// IncrementActiveConnections increments the active connection count
func (m *Metrics) IncrementActiveConnections() {
	atomic.AddInt64(&m.ActiveConnections, 1)
	atomic.AddInt64(&m.TotalConnections, 1)
}

// DecrementActiveConnections decrements the active connection count
func (m *Metrics) DecrementActiveConnections() {
	atomic.AddInt64(&m.ActiveConnections, -1)
	atomic.AddInt64(&m.Disconnections, 1)
}

// IncrementMessages increments the message count
func (m *Metrics) IncrementMessages() {
	atomic.AddInt64(&m.TotalMessages, 1)
}

// IncrementMessageErrors increments the message error count
func (m *Metrics) IncrementMessageErrors() {
	atomic.AddInt64(&m.MessageErrors, 1)
}

// RecordLatency records a message latency
func (m *Metrics) RecordLatency(latency time.Duration) {
	latencyNanos := latency.Nanoseconds()
	atomic.AddInt64(&m.MessageLatency, latencyNanos)
}

// GetActiveConnections returns the current active connection count
func (m *Metrics) GetActiveConnections() int64 {
	return atomic.LoadInt64(&m.ActiveConnections)
}

// GetTotalConnections returns the total connection count
func (m *Metrics) GetTotalConnections() int64 {
	return atomic.LoadInt64(&m.TotalConnections)
}

// GetTotalMessages returns the total message count
func (m *Metrics) GetTotalMessages() int64 {
	return atomic.LoadInt64(&m.TotalMessages)
}

// GetMessageErrors returns the total message error count
func (m *Metrics) GetMessageErrors() int64 {
	return atomic.LoadInt64(&m.MessageErrors)
}

// GetAverageLatency returns the average message latency
func (m *Metrics) GetAverageLatency() time.Duration {
	totalMessages := atomic.LoadInt64(&m.TotalMessages)
	if totalMessages == 0 {
		return 0
	}
	totalLatency := atomic.LoadInt64(&m.MessageLatency)
	return time.Duration(totalLatency / totalMessages)
}

// GetMessagesPerSecond calculates messages per second
func (m *Metrics) GetMessagesPerSecond() float64 {
	m.Mutex.RLock()
	defer m.Mutex.RUnlock()

	duration := time.Since(m.LastReset).Seconds()
	if duration == 0 {
		return 0
	}

	totalMessages := atomic.LoadInt64(&m.TotalMessages)
	return float64(totalMessages) / duration
}

// SetRoomOccupancy sets the occupancy for a room
func (m *Metrics) SetRoomOccupancy(roomName string, count int64) {
	m.Mutex.Lock()
	defer m.Mutex.Unlock()
	m.RoomOccupancy[roomName] = count
}

// GetRoomOccupancy returns the occupancy for a room
func (m *Metrics) GetRoomOccupancy(roomName string) int64 {
	m.Mutex.RLock()
	defer m.Mutex.RUnlock()
	return m.RoomOccupancy[roomName]
}

// GetAllRoomOccupancy returns all room occupancies
func (m *Metrics) GetAllRoomOccupancy() map[string]int64 {
	m.Mutex.RLock()
	defer m.Mutex.RUnlock()

	occupancy := make(map[string]int64, len(m.RoomOccupancy))
	for k, v := range m.RoomOccupancy {
		occupancy[k] = v
	}
	return occupancy
}

// RemoveRoom removes a room from metrics
func (m *Metrics) RemoveRoom(roomName string) {
	m.Mutex.Lock()
	defer m.Mutex.Unlock()
	delete(m.RoomOccupancy, roomName)
}

// Reset resets the metrics (except total counters)
func (m *Metrics) Reset() {
	m.Mutex.Lock()
	defer m.Mutex.Unlock()

	atomic.StoreInt64(&m.ActiveConnections, 0)
	atomic.StoreInt64(&m.Disconnections, 0)
	atomic.StoreInt64(&m.MessageErrors, 0)
	atomic.StoreInt64(&m.MessageLatency, 0)
	m.RoomOccupancy = make(map[string]int64)
	m.LastReset = time.Now()
}

// GetUptime returns the uptime of the application
func (m *Metrics) GetUptime() time.Duration {
	return time.Since(m.StartTime)
}

// GetSummary returns a summary of all metrics
func (m *Metrics) GetSummary() map[string]interface{} {
	return map[string]interface{}{
		"active_connections":    m.GetActiveConnections(),
		"total_connections":     m.GetTotalConnections(),
		"disconnections":        atomic.LoadInt64(&m.Disconnections),
		"total_messages":        m.GetTotalMessages(),
		"message_errors":        m.GetMessageErrors(),
		"messages_per_second":   m.GetMessagesPerSecond(),
		"average_latency_ms":    m.GetAverageLatency().Milliseconds(),
		"room_occupancy":        m.GetAllRoomOccupancy(),
		"uptime_seconds":        m.GetUptime().Seconds(),
	}
}