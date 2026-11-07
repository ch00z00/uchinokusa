package ws

import "github.com/google/uuid"

// Connections は、Hub が抱えている接続の数を返す（テストで、接続を片付けたことを確かめるため）。
func (h *Hub) Connections() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.clients)
}

// Subscribers は、世帯を購読している接続の数を返す。
func (h *Hub) Subscribers(householdID uuid.UUID) int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.subs[householdID])
}
