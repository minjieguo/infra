package cache

import (
	"context"
	"time"
)

// DeleteMany 批量删除 key，覆盖字符串/hash/zset/计数/队列全部结构。
func (s *memoryStore) DeleteMany(_ context.Context, keys ...string) error {
	if len(keys) == 0 {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, key := range keys {
		s.client.Delete(key)
		delete(s.hash, key)
		delete(s.zset, key)
		delete(s.cnt, key)
		delete(s.exp, key)
		delete(s.queue, key)
	}
	return nil
}

// Exists 判断 key 是否存在于任意一块数据结构中（字符串/hash/zset/计数/队列）。
func (s *memoryStore) Exists(_ context.Context, key string) (bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	s.expireAllLocked()
	if _, ok := s.client.Get(key); ok {
		return true, nil
	}
	if _, ok := s.hash[key]; ok {
		return true, nil
	}
	if _, ok := s.zset[key]; ok {
		return true, nil
	}
	if _, ok := s.cnt[key]; ok {
		return true, nil
	}
	if values, ok := s.queue[key]; ok && len(values) > 0 {
		return true, nil
	}
	return false, nil
}

// Expire 为已存在的 key 设置相对过期时间，key 不存在时不做处理。
//
// 字符串 key 的过期由底层 go-cache 负责；hash/zset/计数 key 走 exp 台账。
func (s *memoryStore) Expire(_ context.Context, key string, ttl time.Duration) error {
	if ttl <= 0 {
		return s.DeleteMany(context.Background(), key)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.expireAllLocked()

	_, hasHash := s.hash[key]
	_, hasZSet := s.zset[key]
	_, hasCount := s.cnt[key]
	switch {
	case hasHash || hasZSet || hasCount:
		s.exp[key] = memoryExpireNow().Add(ttl)
	default:
		// 字符串键的 TTL 由 go-cache 维护：取原值后带新 TTL 重新写入。
		if value, _, ok := s.client.GetWithExpiration(key); ok {
			s.client.Set(key, value, ttl)
		}
	}
	return nil
}

// TTL 返回剩余过期时间：key 不存在返回 ErrCacheMiss，无过期时间返回 TTLNoExpire。
func (s *memoryStore) TTL(_ context.Context, key string) (time.Duration, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	s.expireAllLocked()

	if at, ok := s.exp[key]; ok {
		remaining := at.Sub(memoryExpireNow())
		if remaining <= 0 {
			return 0, ErrCacheMiss
		}
		return remaining, nil
	}

	// 字符串键的过期由 go-cache 维护
	if _, expirationTime, ok := s.client.GetWithExpiration(key); ok {
		if expirationTime.IsZero() {
			return TTLNoExpire, nil
		}
		remaining := expirationTime.Sub(memoryExpireNow())
		if remaining <= 0 {
			return 0, ErrCacheMiss
		}
		return remaining, nil
	}

	if _, ok := s.hash[key]; ok {
		return TTLNoExpire, nil
	}
	if _, ok := s.zset[key]; ok {
		return TTLNoExpire, nil
	}
	if _, ok := s.cnt[key]; ok {
		return TTLNoExpire, nil
	}
	if values, ok := s.queue[key]; ok && len(values) > 0 {
		return TTLNoExpire, nil
	}
	return 0, ErrCacheMiss
}

// IncrBy 累加计数值，计数与字符串空间隔离，不受 Set 写入内容影响。
func (s *memoryStore) IncrBy(_ context.Context, key string, delta int64) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.expireLocked(key)
	s.cnt[key] += delta
	return s.cnt[key], nil
}

// DecrBy 自减计数值，语义同 IncrBy。
func (s *memoryStore) DecrBy(_ context.Context, key string, delta int64) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.expireLocked(key)
	s.cnt[key] -= delta
	return s.cnt[key], nil
}
