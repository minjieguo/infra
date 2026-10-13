package oss

import (
	"fmt"
	"os"
	"sync"
	"time"
)

// 雪花 ID 位段划分, 共 63 位, 最高位恒为 0 保证正数:
//
//	1 位符号位 | 41 位时间戳(毫秒) | 10 位机器位 | 12 位序列号
//
// 41 位毫秒时间戳可用约 69 年, 10 位机器位支持 1024 个节点,
// 12 位序列号表示同一毫秒内最多生成 4096 个 ID。
const (
	snowflakeEpoch        int64 = 1700000000000 // 起始时间戳(毫秒), 2023-11-14T22:13:20Z
	snowflakeNodeBits     uint8 = 10            // 机器位长度
	snowflakeSequenceBits uint8 = 12            // 序列号位长度

	snowflakeMaxNode     int64 = -1 ^ (-1 << snowflakeNodeBits)     // 1023
	snowflakeMaxSequence int64 = -1 ^ (-1 << snowflakeSequenceBits) // 4095

	snowflakeNodeShift     = snowflakeSequenceBits
	snowflakeTimeShift     = snowflakeSequenceBits + snowflakeNodeBits
	snowflakeSequenceMask  = snowflakeMaxSequence
	snowflakeTimestampMask = -1 ^ (-1 << (63 - snowflakeSequenceBits - snowflakeNodeBits))
)

// snowflakeNode 雪花 ID 生成器, 通过全局实例串行发号, 保证同一毫秒内序列号递增。
type snowflakeNode struct {
	mu       sync.Mutex
	lastTime int64 // 上次发号的时间戳(毫秒)
	node     int64 // 机器位
	sequence int64 // 当前毫秒内的序列号
}

// snowflakeGlobal 全局雪花生成器, 进程启动时按机器位初始化一次。
var (
	snowflakeOnce   sync.Once
	snowflakeGlobal *snowflakeNode
)

// newSnowflakeNode 创建雪花 ID 生成器, 机器位超出范围时返回错误。
func newSnowflakeNode(node int64) (*snowflakeNode, error) {
	if node < 0 || node > snowflakeMaxNode {
		return nil, fmt.Errorf("雪花 ID 机器位超出范围: %d", node)
	}
	return &snowflakeNode{node: node}, nil
}

// nextID 生成下一个雪花 ID。
//
// 同一毫秒内序列号自增; 序列号用尽则自旋等待下一毫秒;
// 检测到系统时钟回拨时返回错误, 避免生成重复 ID。
func (n *snowflakeNode) nextID() (int64, error) {
	n.mu.Lock()
	defer n.mu.Unlock()

	now := time.Now().UnixMilli()
	if now < n.lastTime {
		return 0, fmt.Errorf("系统时钟回拨, 拒绝生成雪花 ID: 当前 %d, 上次 %d", now, n.lastTime)
	}

	if now == n.lastTime {
		n.sequence = (n.sequence + 1) & snowflakeSequenceMask
		if n.sequence == 0 {
			// 当前毫秒序列号用尽, 自旋等待下一毫秒
			for now <= n.lastTime {
				now = time.Now().UnixMilli()
			}
		}
	} else {
		n.sequence = 0
	}

	n.lastTime = now

	id := ((now - snowflakeEpoch) & snowflakeTimestampMask) << snowflakeTimeShift
	id |= (n.node & snowflakeMaxNode) << snowflakeNodeShift
	id |= n.sequence
	return id, nil
}

// snowflakeID 返回全局雪花生成器生成的 ID 字符串。
//
// 机器位取自环境变量 OSS_SNOWFLAKE_NODE, 未配置或非法时使用 0, 保证多实例部署时可配置隔离。
func snowflakeID() (string, error) {
	snowflakeOnce.Do(func() {
		var node int64
		if v := os.Getenv("OSS_SNOWFLAKE_NODE"); v != "" {
			if _, err := fmt.Sscanf(v, "%d", &node); err != nil {
				node = 0
			}
		}
		instance, err := newSnowflakeNode(node)
		if err != nil {
			instance = &snowflakeNode{}
		}
		snowflakeGlobal = instance
	})

	id, err := snowflakeGlobal.nextID()
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%d", id), nil
}
