package cache

import (
	"context"
	"time"
)

// memoryExpireNow 返回 memory 后端统一使用的当前时间，抽成函数便于用例替换。
var memoryExpireNow = time.Now

// expireLocked 清除已过期的 key（hash/zset/cnt 三块受 exp 控制的数据）。
//
// 调用方必须已持有 s.mu 写锁；惰性淘汰，读取与写入前各调用一次，
// 保证过期 key 不会再被访问到。
func (s *memoryStore) expireLocked(key string) {
	at, ok := s.exp[key]
	if !ok {
		return
	}
	if memoryExpireNow().Before(at) {
		return
	}
	delete(s.exp, key)
	delete(s.hash, key)
	delete(s.zset, key)
	delete(s.cnt, key)
}

// expireAllLocked 整体清理一次过期 key，供 Exists/TTL 等跨 key 查询使用。
func (s *memoryStore) expireAllLocked() {
	now := memoryExpireNow()
	for key, at := range s.exp {
		if now.Before(at) {
			continue
		}
		delete(s.exp, key)
		delete(s.hash, key)
		delete(s.zset, key)
		delete(s.cnt, key)
	}
}

func (s *memoryStore) ZAdd(_ context.Context, key string, members map[string]float64) error {
	if len(members) == 0 {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.expireLocked(key)
	scores, ok := s.zset[key]
	if !ok {
		scores = make(map[string]float64, len(members))
		s.zset[key] = scores
	}
	for member, score := range members {
		scores[member] = score
	}
	return nil
}

func (s *memoryStore) ZRange(_ context.Context, key string, start int64, stop int64) ([]ZSetMember, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	s.expireLocked(key)
	return sliceZSetMembers(sortZSetMembers(s.zset[key]), start, stop), nil
}

func (s *memoryStore) ZRevRange(_ context.Context, key string, start int64, stop int64) ([]ZSetMember, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	s.expireLocked(key)
	members := reverseZSetMembers(sortZSetMembers(s.zset[key]))
	return sliceZSetMembers(members, start, stop), nil
}

func (s *memoryStore) ZCard(_ context.Context, key string) (int64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	s.expireLocked(key)
	return int64(len(s.zset[key])), nil
}

func (s *memoryStore) ZScore(_ context.Context, key string, member string) (float64, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	s.expireLocked(key)
	score, ok := s.zset[key][member]
	return score, ok, nil
}

func (s *memoryStore) ZIncrBy(_ context.Context, key string, member string, delta float64) (float64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.expireLocked(key)
	scores, ok := s.zset[key]
	if !ok {
		scores = make(map[string]float64)
		s.zset[key] = scores
	}
	scores[member] += delta
	return scores[member], nil
}

func (s *memoryStore) ZRem(_ context.Context, key string, members ...string) error {
	if len(members) == 0 {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.expireLocked(key)
	scores, ok := s.zset[key]
	if !ok {
		return nil
	}
	for _, member := range members {
		delete(scores, member)
	}
	if len(scores) == 0 {
		delete(s.zset, key)
		delete(s.exp, key)
	}
	return nil
}

func (s *memoryStore) ZRank(_ context.Context, key string, member string) (int64, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	s.expireLocked(key)
	rank, ok := zSetRank(sortZSetMembers(s.zset[key]), member)
	return rank, ok, nil
}

func (s *memoryStore) ZRevRank(_ context.Context, key string, member string) (int64, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	s.expireLocked(key)
	members := reverseZSetMembers(sortZSetMembers(s.zset[key]))
	rank, ok := zSetRank(members, member)
	return rank, ok, nil
}

func (s *memoryStore) ZRangeByScore(_ context.Context, key string, min float64, max float64, offset int64, count int64) ([]ZSetMember, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	s.expireLocked(key)
	return sliceZSetByScore(sortZSetMembers(s.zset[key]), min, max, offset, count), nil
}

func (s *memoryStore) ZRevRangeByScore(_ context.Context, key string, min float64, max float64, offset int64, count int64) ([]ZSetMember, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	s.expireLocked(key)

	// 先按分值过滤再反转，保证 offset/count 作用在降序序列上
	filtered := filterZSetByScore(sortZSetMembers(s.zset[key]), min, max)
	return sliceZSetRange(reverseZSetMembers(filtered), offset, count), nil
}

func (s *memoryStore) ZCount(_ context.Context, key string, min float64, max float64) (int64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	s.expireLocked(key)
	members := sliceZSetByScore(sortZSetMembers(s.zset[key]), min, max, 0, 0)
	return int64(len(members)), nil
}

func (s *memoryStore) ZRemRangeByScore(_ context.Context, key string, min float64, max float64) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.expireLocked(key)
	scores, ok := s.zset[key]
	if !ok || min > max {
		return 0, nil
	}
	var removed int64
	for member, score := range scores {
		if score >= min && score <= max {
			delete(scores, member)
			removed++
		}
	}
	s.cleanupZSetLocked(key, scores)
	return removed, nil
}

func (s *memoryStore) ZRemRangeByRank(_ context.Context, key string, start int64, stop int64) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.expireLocked(key)
	scores, ok := s.zset[key]
	if !ok {
		return 0, nil
	}
	targets := sliceZSetMembers(sortZSetMembers(scores), start, stop)
	for _, item := range targets {
		delete(scores, item.Member)
	}
	s.cleanupZSetLocked(key, scores)
	return int64(len(targets)), nil
}

// cleanupZSetLocked 在有序集合被清空时移除 key 及关联的过期台账。
//
// 调用方必须已持有 s.mu 写锁。
func (s *memoryStore) cleanupZSetLocked(key string, scores map[string]float64) {
	if len(scores) > 0 {
		return
	}
	delete(s.zset, key)
	delete(s.exp, key)
}
