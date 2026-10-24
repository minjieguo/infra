package cache

import (
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

type redisStore struct {
	client *redis.Client
	prefix string
}

func newRedisStore(cfg Config) (*redisStore, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     cfg.Addr,
		Password: cfg.Password,
		DB:       cfg.DB,
	})
	if err := client.Ping(context.Background()).Err(); err != nil {
		_ = client.Close()
		return nil, err
	}
	return &redisStore{
		client: client,
		prefix: cfg.Prefix,
	}, nil
}

func (s *redisStore) Set(ctx context.Context, key string, value string, ttl time.Duration) error {
	return s.client.Set(ctx, s.key(key), value, ttl).Err()
}

func (s *redisStore) Get(ctx context.Context, key string) (string, error) {
	value, err := s.client.Get(ctx, s.key(key)).Result()
	if errors.Is(err, redis.Nil) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return value, nil
}

func (s *redisStore) Delete(ctx context.Context, key string) error {
	return s.client.Del(ctx, s.key(key)).Err()
}

func (s *redisStore) DeleteMany(ctx context.Context, keys ...string) error {
	if len(keys) == 0 {
		return nil
	}
	fullKeys := make([]string, 0, len(keys))
	for _, key := range keys {
		fullKeys = append(fullKeys, s.key(key))
	}
	return s.client.Del(ctx, fullKeys...).Err()
}

func (s *redisStore) Exists(ctx context.Context, key string) (bool, error) {
	count, err := s.client.Exists(ctx, s.key(key)).Result()
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (s *redisStore) Expire(ctx context.Context, key string, ttl time.Duration) error {
	return s.client.Expire(ctx, s.key(key), ttl).Err()
}

// TTL 返回剩余过期时间：键不存在返回 ErrCacheMiss，无过期时间返回 TTLNoExpire。
func (s *redisStore) TTL(ctx context.Context, key string) (time.Duration, error) {
	ttl, err := s.client.TTL(ctx, s.key(key)).Result()
	if err != nil {
		return 0, err
	}
	if ttl == -2 {
		return 0, ErrCacheMiss
	}
	if ttl == -1 {
		return TTLNoExpire, nil
	}
	return ttl, nil
}

func (s *redisStore) IncrBy(ctx context.Context, key string, delta int64) (int64, error) {
	return s.client.IncrBy(ctx, s.key(key), delta).Result()
}

func (s *redisStore) DecrBy(ctx context.Context, key string, delta int64) (int64, error) {
	return s.client.DecrBy(ctx, s.key(key), delta).Result()
}

func (s *redisStore) Close() error {
	return s.client.Close()
}

func (s *redisStore) key(key string) string {
	return s.prefix + key
}

func (s *redisStore) HSet(ctx context.Context, key string, field string, value any) error {
	text, err := marshalHashValue(value)
	if err != nil {
		return err
	}
	return s.client.HSet(ctx, s.key(key), field, text).Err()
}

func (s *redisStore) HGet(ctx context.Context, key string, field string) (any, error) {
	text, err := s.client.HGet(ctx, s.key(key), field).Result()
	if errors.Is(err, redis.Nil) {
		return nil, ErrCacheMiss
	}
	if err != nil {
		return nil, err
	}
	return unmarshalHashValue(text)
}

func (s *redisStore) HGetAll(ctx context.Context, key string) (map[string]any, error) {
	items, err := s.client.HGetAll(ctx, s.key(key)).Result()
	if err != nil {
		return nil, err
	}
	result := make(map[string]any, len(items))
	for field, text := range items {
		value, err := unmarshalHashValue(text)
		if err != nil {
			return nil, err
		}
		result[field] = value
	}
	return result, nil
}

func (s *redisStore) HDel(ctx context.Context, key string, fields ...string) error {
	if len(fields) == 0 {
		return nil
	}
	return s.client.HDel(ctx, s.key(key), fields...).Err()
}

func (s *redisStore) HExists(ctx context.Context, key string, field string) (bool, error) {
	return s.client.HExists(ctx, s.key(key), field).Result()
}

func (s *redisStore) HLen(ctx context.Context, key string) (int64, error) {
	return s.client.HLen(ctx, s.key(key)).Result()
}

func (s *redisStore) HKeys(ctx context.Context, key string) ([]string, error) {
	keys, err := s.client.HKeys(ctx, s.key(key)).Result()
	if errors.Is(err, redis.Nil) {
		return []string{}, nil
	}
	if err != nil {
		return nil, err
	}
	return keys, nil
}

func (s *redisStore) HVals(ctx context.Context, key string) ([]any, error) {
	values, err := s.client.HVals(ctx, s.key(key)).Result()
	if errors.Is(err, redis.Nil) {
		return []any{}, nil
	}
	if err != nil {
		return nil, err
	}
	result := make([]any, 0, len(values))
	for _, text := range values {
		value, err := unmarshalHashValue(text)
		if err != nil {
			return nil, err
		}
		result = append(result, value)
	}
	return result, nil
}

func (s *redisStore) HMGet(ctx context.Context, key string, fields ...string) (map[string]any, error) {
	result := make(map[string]any, len(fields))
	if len(fields) == 0 {
		return result, nil
	}
	values, err := s.client.HMGet(ctx, s.key(key), fields...).Result()
	if err != nil {
		return nil, err
	}
	for i, field := range fields {
		if i >= len(values) || values[i] == nil {
			continue
		}
		text, ok := values[i].(string)
		if !ok {
			continue
		}
		value, err := unmarshalHashValue(text)
		if err != nil {
			return nil, err
		}
		result[field] = value
	}
	return result, nil
}

func (s *redisStore) HMSet(ctx context.Context, key string, values map[string]any) error {
	if len(values) == 0 {
		return nil
	}
	items := make(map[string]any, len(values))
	for field, value := range values {
		text, err := marshalHashValue(value)
		if err != nil {
			return err
		}
		items[field] = text
	}
	return s.client.HSet(ctx, s.key(key), items).Err()
}

func (s *redisStore) HExpire(ctx context.Context, key string, ttl time.Duration, fields ...string) error {
	if len(fields) == 0 {
		return nil
	}
	return s.client.HExpire(ctx, s.key(key), ttl, fields...).Err()
}

func (s *redisStore) HExpireAt(ctx context.Context, key string, at time.Time, fields ...string) error {
	if len(fields) == 0 {
		return nil
	}
	return s.client.HExpireAt(ctx, s.key(key), at, fields...).Err()
}

// queueBlockPollInterval 是 Redis 队列等待元素时的轮询间隔。
//
// 不使用 BRPOP 原生阻塞的原因：go-redis 的连接读超时由 Cmd 的 readTimeout 推导，
// BRPOP(0) 传入 0 会让 cmdTimeout 返回 0，最终读超时变成 noDeadline，
// 由 ctx 取消触发的 socket 读不会被打断，调用方拿不回控制权。
// 改成短超时 BRPOP 轮询：既保留了阻塞语义，又保证 ctx 取消/超时能及时返回。
const queueBlockPollInterval = time.Second

func (s *redisStore) LPush(ctx context.Context, key string, value string) error {
	return s.client.LPush(ctx, s.key(key), value).Err()
}

// BRPop 阻塞等待队列右侧元素: 按短超时轮询 BRPOP，ctx 结束或客户端关闭时返回。
func (s *redisStore) BRPop(ctx context.Context, key string) (string, error) {
	for {
		values, err := s.client.BRPop(ctx, queueBlockPollInterval, s.key(key)).Result()
		switch {
		case err == nil:
			if len(values) < 2 {
				return "", ErrCacheMiss
			}
			// values[0] 为 key 名,values[1] 为元素值
			return values[1], nil
		case errors.Is(err, redis.Nil):
			// 轮询窗口内无元素,继续等待
		case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
			return "", err
		default:
			return "", err
		}

		if err := ctx.Err(); err != nil {
			return "", err
		}
	}
}

func (s *redisStore) ZAdd(ctx context.Context, key string, members map[string]float64) error {
	if len(members) == 0 {
		return nil
	}
	items := make([]redis.Z, 0, len(members))
	for member, score := range members {
		items = append(items, redis.Z{Score: score, Member: member})
	}
	return s.client.ZAdd(ctx, s.key(key), items...).Err()
}

func (s *redisStore) ZRange(ctx context.Context, key string, start int64, stop int64) ([]ZSetMember, error) {
	return s.zRange(ctx, key, start, stop, false)
}

func (s *redisStore) ZRevRange(ctx context.Context, key string, start int64, stop int64) ([]ZSetMember, error) {
	return s.zRange(ctx, key, start, stop, true)
}

// zRange 读取有序集合区间，rev 为 true 时按分值降序返回。
func (s *redisStore) zRange(ctx context.Context, key string, start int64, stop int64, rev bool) ([]ZSetMember, error) {
	items, err := s.client.ZRangeArgsWithScores(ctx, redis.ZRangeArgs{
		Key:   s.key(key),
		Start: start,
		Stop:  stop,
		Rev:   rev,
	}).Result()
	if err != nil {
		return nil, err
	}
	result := make([]ZSetMember, 0, len(items))
	for _, item := range items {
		member, ok := item.Member.(string)
		if !ok {
			continue
		}
		result = append(result, ZSetMember{Member: member, Score: item.Score})
	}
	return result, nil
}

func (s *redisStore) ZCard(ctx context.Context, key string) (int64, error) {
	return s.client.ZCard(ctx, s.key(key)).Result()
}

func (s *redisStore) ZScore(ctx context.Context, key string, member string) (float64, bool, error) {
	score, err := s.client.ZScore(ctx, s.key(key), member).Result()
	if errors.Is(err, redis.Nil) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	return score, true, nil
}

func (s *redisStore) ZIncrBy(ctx context.Context, key string, member string, delta float64) (float64, error) {
	return s.client.ZIncrBy(ctx, s.key(key), delta, member).Result()
}

func (s *redisStore) ZRem(ctx context.Context, key string, members ...string) error {
	if len(members) == 0 {
		return nil
	}
	items := make([]any, 0, len(members))
	for _, member := range members {
		items = append(items, member)
	}
	return s.client.ZRem(ctx, s.key(key), items...).Err()
}

func (s *redisStore) ZRank(ctx context.Context, key string, member string) (int64, bool, error) {
	return s.zRank(ctx, key, member, false)
}

func (s *redisStore) ZRevRank(ctx context.Context, key string, member string) (int64, bool, error) {
	return s.zRank(ctx, key, member, true)
}

// zRank 查询成员名次，rev 为 true 时按分值降序计算。
func (s *redisStore) zRank(ctx context.Context, key string, member string, rev bool) (int64, bool, error) {
	var (
		rank int64
		err  error
	)
	if rev {
		rank, err = s.client.ZRevRank(ctx, s.key(key), member).Result()
	} else {
		rank, err = s.client.ZRank(ctx, s.key(key), member).Result()
	}
	if errors.Is(err, redis.Nil) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	return rank, true, nil
}

func (s *redisStore) ZRangeByScore(ctx context.Context, key string, min float64, max float64, offset int64, count int64) ([]ZSetMember, error) {
	if min > max {
		return []ZSetMember{}, nil
	}
	items, err := s.client.ZRangeArgsWithScores(ctx, redis.ZRangeArgs{
		Key:     s.key(key),
		Start:   min,
		Stop:    max,
		ByScore: true,
		Offset:  offset,
		Count:   count,
	}).Result()
	if err != nil {
		return nil, err
	}
	return redisZSlices(items), nil
}

func (s *redisStore) ZRevRangeByScore(ctx context.Context, key string, min float64, max float64, offset int64, count int64) ([]ZSetMember, error) {
	if min > max {
		return []ZSetMember{}, nil
	}
	// BYSCORE + REV 时 Redis 要求先写高分（max）再写低分（min）
	items, err := s.client.ZRangeArgsWithScores(ctx, redis.ZRangeArgs{
		Key:     s.key(key),
		Start:   max,
		Stop:    min,
		ByScore: true,
		Rev:     true,
		Offset:  offset,
		Count:   count,
	}).Result()
	if err != nil {
		return nil, err
	}
	return redisZSlices(items), nil
}

func (s *redisStore) ZCount(ctx context.Context, key string, min float64, max float64) (int64, error) {
	if min > max {
		return 0, nil
	}
	return s.client.ZCount(ctx, s.key(key), formatScore(min), formatScore(max)).Result()
}

func (s *redisStore) ZRemRangeByScore(ctx context.Context, key string, min float64, max float64) (int64, error) {
	if min > max {
		return 0, nil
	}
	return s.client.ZRemRangeByScore(ctx, s.key(key), formatScore(min), formatScore(max)).Result()
}

func (s *redisStore) ZRemRangeByRank(ctx context.Context, key string, start int64, stop int64) (int64, error) {
	return s.client.ZRemRangeByRank(ctx, s.key(key), start, stop).Result()
}

// redisZSlices 将 go-redis 的有序集合结果转换为 ZSetMember 切片。
func redisZSlices(items []redis.Z) []ZSetMember {
	result := make([]ZSetMember, 0, len(items))
	for _, item := range items {
		member, ok := item.Member.(string)
		if !ok {
			continue
		}
		result = append(result, ZSetMember{Member: member, Score: item.Score})
	}
	return result
}

// formatScore 将分值格式化为 Redis 分值区间参数，保证浮点数按最短形式传输。
func formatScore(score float64) string {
	return strconv.FormatFloat(score, 'g', -1, 64)
}
