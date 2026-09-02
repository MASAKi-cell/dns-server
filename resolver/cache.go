// TTLベースのDNSキャッシュ。
package resolver

import (
	"strings"
	"sync"
	"time"

	"github.com/MASAKi-cell/dns/message"
)

// キャッシュエントリ。
type cacheEntry struct {
	records   []message.ResourceRecord
	expiresAt time.Time
}

// TTLベースのDNSキャッシュ。
type Cache struct {
	mu      sync.RWMutex
	entries map[string]cacheEntry
}

// 新しいキャッシュを作成する。
func NewCache() *Cache {
	return &Cache{
		entries: make(map[string]cacheEntry),
	}
}

// キャッシュキーを生成する。
func cacheKey(name string, typ message.Type) string {
	return strings.ToLower(name) + "/" + typ.String()
}

// キャッシュからレコードを取得する。
// 有効期限切れの場合はnilを返す。
func (c *Cache) Get(name string, typ message.Type) []message.ResourceRecord {
	c.mu.RLock()
	defer c.mu.RUnlock()

	key := cacheKey(name, typ)
	entry, ok := c.entries[key]
	if !ok {
		return nil
	}

	if time.Now().After(entry.expiresAt) {
		return nil
	}

	// レコードのコピーを返す（TTLを調整）
	remaining := time.Until(entry.expiresAt)
	remainingTTL := uint32(remaining.Seconds())
	if remainingTTL == 0 {
		remainingTTL = 1
	}

	result := make([]message.ResourceRecord, len(entry.records))
	for i, rr := range entry.records {
		result[i] = rr
		result[i].TTL = remainingTTL
	}

	return result
}

// キャッシュにレコードを追加する。
func (c *Cache) Set(records []message.ResourceRecord) {
	if len(records) == 0 {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// レコードをタイプごとにグループ化
	groups := make(map[string][]message.ResourceRecord)
	for _, rr := range records {
		key := cacheKey(string(rr.Name), rr.Type)
		groups[key] = append(groups[key], rr)
	}

	now := time.Now()
	for key, rrs := range groups {
		// 最小TTLを使用
		minTTL := rrs[0].TTL
		for _, rr := range rrs[1:] {
			if rr.TTL < minTTL {
				minTTL = rr.TTL
			}
		}

		c.entries[key] = cacheEntry{
			records:   rrs,
			expiresAt: now.Add(time.Duration(minTTL) * time.Second),
		}
	}
}

// 期限切れエントリを削除する。
func (c *Cache) Cleanup() {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	for key, entry := range c.entries {
		if now.After(entry.expiresAt) {
			delete(c.entries, key)
		}
	}
}

// キャッシュのエントリ数を返す。
func (c *Cache) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.entries)
}
