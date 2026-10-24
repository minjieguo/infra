package cache

import (
	"context"
	"sort"
	"sync"
	"time"

	gocache "github.com/patrickmn/go-cache"
)

type memoryStore struct {
	client *gocache.Cache

	mu   sync.RWMutex
	hash map[string]map[string]string
	zset map[string]map[string]float64 // key -> member -> score
	cnt  map[string]int64              // key -> 计数值，与字符串空间隔离
	exp  map[string]time.Time          // key -> 过期时间，覆盖 hash/zset/cnt

	queue  map[string][]string      // key -> 元素切片，尾部为 Pop 端
	wait   map[string]chan struct{} // key -> 唤醒通知(1 缓冲)
	closed chan struct{}            // 关闭信号，用于唤醒阻塞中的 Pop
}

func newMemoryStore() *memoryStore {
	return &memoryStore{
		client: gocache.New(5*time.Minute, 10*time.Minute),
		hash:   make(map[string]map[string]string),
		zset:   make(map[string]map[string]float64),
		cnt:    make(map[string]int64),
		exp:    make(map[string]time.Time),
		queue:  make(map[string][]string),
		wait:   make(map[string]chan struct{}),
		closed: make(chan struct{}),
	}
}

func (s *memoryStore) Set(_ context.Context, key string, value string, ttl time.Duration) error {
	s.client.Set(key, value, ttl)
	return nil
}

func (s *memoryStore) Get(_ context.Context, key string) (string, error) {
	value, ok := s.client.Get(key)
	if !ok {
		return "", nil
	}
	text, ok := value.(string)
	if !ok {
		return "", nil
	}
	return text, nil
}

func (s *memoryStore) Delete(_ context.Context, key string) error {
	s.client.Delete(key)
	return nil
}

func (s *memoryStore) Close() error {
	s.mu.Lock()
	// 用 close 通知所有阻塞中的 Pop 退出,避免 goroutine 泄漏
	select {
	case <-s.closed:
	default:
		close(s.closed)
	}
	s.hash = nil
	s.zset = nil
	s.cnt = nil
	s.exp = nil
	s.queue = nil
	s.wait = nil
	s.mu.Unlock()

	s.client.Flush()
	return nil
}

func (s *memoryStore) HSet(_ context.Context, key string, field string, value any) error {
	text, err := marshalHashValue(value)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	fields, ok := s.hash[key]
	if !ok {
		fields = make(map[string]string)
		s.hash[key] = fields
	}
	fields[field] = text
	return nil
}

func (s *memoryStore) HGet(_ context.Context, key string, field string) (any, error) {
	s.mu.RLock()
	text, ok := s.hash[key][field]
	s.mu.RUnlock()
	if !ok {
		return nil, ErrCacheMiss
	}
	return unmarshalHashValue(text)
}

func (s *memoryStore) HGetAll(_ context.Context, key string) (map[string]any, error) {
	s.mu.RLock()
	fields := s.hash[key]
	snapshot := make(map[string]string, len(fields))
	for field, text := range fields {
		snapshot[field] = text
	}
	s.mu.RUnlock()

	result := make(map[string]any, len(snapshot))
	for field, text := range snapshot {
		value, err := unmarshalHashValue(text)
		if err != nil {
			return nil, err
		}
		result[field] = value
	}
	return result, nil
}

func (s *memoryStore) HDel(_ context.Context, key string, fields ...string) error {
	if len(fields) == 0 {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	current, ok := s.hash[key]
	if !ok {
		return nil
	}
	for _, field := range fields {
		delete(current, field)
	}
	if len(current) == 0 {
		delete(s.hash, key)
	}
	return nil
}

func (s *memoryStore) HExists(_ context.Context, key string, field string) (bool, error) {
	s.mu.RLock()
	_, ok := s.hash[key][field]
	s.mu.RUnlock()
	return ok, nil
}

func (s *memoryStore) HLen(_ context.Context, key string) (int64, error) {
	s.mu.RLock()
	size := len(s.hash[key])
	s.mu.RUnlock()
	return int64(size), nil
}

func (s *memoryStore) HKeys(_ context.Context, key string) ([]string, error) {
	s.mu.RLock()
	fields := s.hash[key]
	keys := make([]string, 0, len(fields))
	for field := range fields {
		keys = append(keys, field)
	}
	s.mu.RUnlock()
	sort.Strings(keys)
	return keys, nil
}

func (s *memoryStore) HVals(_ context.Context, key string) ([]any, error) {
	s.mu.RLock()
	fields := s.hash[key]
	keys := make([]string, 0, len(fields))
	for field := range fields {
		keys = append(keys, field)
	}
	sort.Strings(keys)
	texts := make([]string, 0, len(keys))
	for _, field := range keys {
		texts = append(texts, fields[field])
	}
	s.mu.RUnlock()

	result := make([]any, 0, len(texts))
	for _, text := range texts {
		value, err := unmarshalHashValue(text)
		if err != nil {
			return nil, err
		}
		result = append(result, value)
	}
	return result, nil
}

func (s *memoryStore) HMGet(_ context.Context, key string, fields ...string) (map[string]any, error) {
	result := make(map[string]any, len(fields))
	if len(fields) == 0 {
		return result, nil
	}
	s.mu.RLock()
	current := s.hash[key]
	texts := make(map[string]string, len(fields))
	for _, field := range fields {
		if text, ok := current[field]; ok {
			texts[field] = text
		}
	}
	s.mu.RUnlock()

	for field, text := range texts {
		value, err := unmarshalHashValue(text)
		if err != nil {
			return nil, err
		}
		result[field] = value
	}
	return result, nil
}

func (s *memoryStore) HMSet(_ context.Context, key string, values map[string]any) error {
	if len(values) == 0 {
		return nil
	}
	texts := make(map[string]string, len(values))
	for field, value := range values {
		text, err := marshalHashValue(value)
		if err != nil {
			return err
		}
		texts[field] = text
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	fields, ok := s.hash[key]
	if !ok {
		fields = make(map[string]string)
		s.hash[key] = fields
	}
	for field, text := range texts {
		fields[field] = text
	}
	return nil
}

// HExpire memory 实现不支持字段级过期，直接返回成功。
func (s *memoryStore) HExpire(_ context.Context, _ string, _ time.Duration, _ ...string) error {
	return nil
}

// HExpireAt memory 实现不支持字段级过期，直接返回成功。
func (s *memoryStore) HExpireAt(_ context.Context, _ string, _ time.Time, _ ...string) error {
	return nil
}

// LPush 从队列左侧插入元素(等价 Redis LPUSH)，由 BRPop 从右侧取出。
func (s *memoryStore) LPush(_ context.Context, key string, value string) error {
	s.mu.Lock()
	if s.queue == nil {
		s.mu.Unlock()
		return ErrQueueClosed
	}
	// 头插，保证与 Redis LPUSH 一致：多次 Push 后 Pop 按先进先出顺序取出
	s.queue[key] = append([]string{value}, s.queue[key]...)
	notify := s.wait[key]
	if notify == nil {
		notify = make(chan struct{}, 1)
		s.wait[key] = notify
	}
	s.mu.Unlock()

	// 非阻塞唤醒:无等待者时丢弃通知,不会阻塞 Push
	select {
	case notify <- struct{}{}:
	default:
	}
	return nil
}

// BRPop 阻塞等待队列元素;队列为空时挂起,直到有元素入队、ctx 结束或 Close。
func (s *memoryStore) BRPop(ctx context.Context, key string) (string, error) {
	for {
		s.mu.Lock()
		if s.queue == nil {
			s.mu.Unlock()
			return "", ErrQueueClosed
		}
		values := s.queue[key]
		if len(values) > 0 {
			value := values[len(values)-1]
			if len(values) == 1 {
				delete(s.queue, key)
			} else {
				s.queue[key] = values[:len(values)-1]
			}
			s.mu.Unlock()
			return value, nil
		}
		notify := s.wait[key]
		if notify == nil {
			notify = make(chan struct{}, 1)
			s.wait[key] = notify
		}
		s.mu.Unlock()

		select {
		case <-notify:
			// 被唤醒后重新判断队列长度,多余唤醒自然消化
		case <-ctx.Done():
			return "", ctx.Err()
		case <-s.closed:
			return "", ErrQueueClosed
		}
	}
}
