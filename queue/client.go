package queue

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/minjieguo/infra/logger"
	"github.com/twmb/franz-go/pkg/kgo"
	"go.uber.org/zap"
)

// Handler 消费回调,对每条消息调用一次;返回 error 表示该消息处理失败,不会被提交。
type Handler func(ctx context.Context, record *kgo.Record) error

// Config Kafka 客户端配置。
//
// 同一个 Client 既可发送也可订阅(底层复用同一个 franz-go 客户端)。
type Config struct {
	Brokers  []string      // Kafka 集群地址
	GroupID  string        // 消费组 ID,仅消费时需要
	ClientID string        // 客户端标识,可选
	Logger   logger.Logger // 日志
	Opts     []kgo.Opt     // franz-go 配置,追加在包内默认配置之后,可覆盖默认值
	OnError  func(error)   // 可选,消费循环/提交错误回调
}

// consumeRetryBackoff 拉取出错后的退避等待时长(固定等待)
const consumeRetryBackoff = 2 * time.Second

// Client Kafka 客户端,支持生产与消费。
type Client struct {
	ctx    context.Context
	cancel context.CancelFunc
	logger logger.Logger
	client *kgo.Client
	onErr  func(error)
}

// New 初始化 Kafka 客户端。
//
// 未传入 Opts 时使用包内默认配置(对齐 sarama 默认语义):
//   - 生产者: AllISRAcks + snappy 压缩
//   - 消费者: 手动提交 offset + BlockRebalanceOnPoll
//
// GroupID 为空表示只用于生产;非空时可用于 Consume。
func New(cfg Config) (*Client, error) {
	if len(cfg.Brokers) == 0 {
		return nil, fmt.Errorf("brokers 不能为空")
	}

	c := &Client{logger: cfg.Logger, onErr: cfg.OnError}
	if c.logger == nil {
		c.logger = defaultLogger{}
	}

	ctx, cancel := context.WithCancel(context.Background())
	c.ctx, c.cancel = ctx, cancel

	opts := []kgo.Opt{
		kgo.SeedBrokers(cfg.Brokers...),
		kgo.RequiredAcks(kgo.AllISRAcks()),
		kgo.ProducerBatchCompression(kgo.SnappyCompression()),
	}
	if cfg.GroupID != "" {
		opts = append(opts,
			kgo.ConsumerGroup(cfg.GroupID),
			kgo.DisableAutoCommit(),    // 由包内手动提交,语义等价 sarama session.MarkMessage
			kgo.BlockRebalanceOnPoll(), // rebalance 前保证已拉取记录被处理完
		)
	}
	if cfg.ClientID != "" {
		opts = append(opts, kgo.ClientID(cfg.ClientID))
	}
	opts = append(opts, cfg.Opts...)

	client, err := kgo.NewClient(opts...)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("创建消费者组失败: %w", err)
	}
	c.client = client

	return c, nil
}

// Send 同步发送一条消息,返回所在分区与偏移量。
func (c *Client) Send(ctx context.Context, record *kgo.Record) (partition int32, offset int64, err error) {
	if c == nil || c.client == nil {
		return 0, 0, fmt.Errorf("客户端未初始化")
	}
	if record == nil {
		return 0, 0, fmt.Errorf("消息不能为空")
	}

	if err = c.client.ProduceSync(ctx, record).FirstErr(); err != nil {
		c.logger.Error("发送失败", zap.Error(err))
		return 0, 0, err
	}

	partition, offset = record.Partition, record.Offset
	c.logger.Debug("发送成功",
		zap.String("topic", record.Topic),
		zap.Int32("partition", partition),
		zap.Int64("offset", offset),
		zap.Any("key", record.Key))
	return partition, offset, nil
}

// Consume 订阅指定 Topic 并启动包内消费循环(非阻塞)。
//
// 每次调用会为传入的 topics 与 handler 启动一个消费循环;
// 需在 Config 中配置 GroupID,否则返回错误。
func (c *Client) Consume(handler Handler, topics ...string) error {
	if c == nil || c.client == nil {
		return fmt.Errorf("客户端未初始化")
	}
	if len(topics) == 0 {
		return fmt.Errorf("topics 不能为空")
	}
	if handler == nil {
		return fmt.Errorf("handler 不能为空")
	}

	c.client.AddConsumeTopics(topics...)
	go c.consumeLoop(handler)
	return nil
}

// consumeLoop 包内自驱动消费循环
func (c *Client) consumeLoop(handler Handler) {
	report := func(msg string, err error) {
		c.logger.Error(msg, zap.Error(err))
		if c.onErr != nil {
			c.onErr(err)
		}
	}

	for {
		fetches := c.client.PollFetches(c.ctx)

		// ctx 取消或客户端关闭,退出循环
		if c.ctx.Err() != nil || errors.Is(fetches.Err(), context.Canceled) || errors.Is(fetches.Err(), kgo.ErrClientClosed) {
			return
		}
		if err := fetches.Err(); err != nil {
			report("队列消费错误", err)
			c.delayRetry(consumeRetryBackoff)
			continue
		}

		var successful []*kgo.Record
		fetches.EachRecord(func(record *kgo.Record) {
			if err := handler(c.ctx, record); err != nil {
				// 处理失败:记录错误,不提交该消息,继续处理后续消息。
				report("队列消息处理失败", err)
				return
			}
			successful = append(successful, record)
		})

		if len(successful) > 0 {
			if err := c.client.CommitRecords(c.ctx, successful...); err != nil && c.ctx.Err() == nil {
				report("队列提交失败", err)
			}
		}
	}
}

// delayRetry 退避等待,期间可被 ctx 取消立即返回
func (c *Client) delayRetry(d time.Duration) {
	select {
	case <-time.After(d):
	case <-c.ctx.Done():
	}
}

// Raw 返回底层 franz-go 客户端,供高级场景使用。
func (c *Client) Raw() *kgo.Client {
	if c == nil {
		return nil
	}
	return c.client
}

// Close 关闭 Kafka 客户端。
//
// 由于启用了 kgo.BlockRebalanceOnPoll,必须使用 CloseAllowingRebalance,
// 否则在已 poll 的情况下关闭会因等待 rebalance 而阻塞。
func (c *Client) Close() error {
	if c == nil {
		return nil
	}
	if c.client != nil {
		c.client.CloseAllowingRebalance()
	}
	if c.cancel != nil {
		c.cancel()
	}
	return nil
}
