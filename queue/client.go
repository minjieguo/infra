package queue

import (
	"context"
	"fmt"
	"time"

	"github.com/IBM/sarama"
	"github.com/minjieguo/infra/logger"
	"go.uber.org/zap"
)

// defaultLogger 默认空日志实现,当未传入 Logger 时使用
type defaultLogger struct{}

func (defaultLogger) Debug(string, ...zap.Field) {}
func (defaultLogger) Info(string, ...zap.Field)  {}
func (defaultLogger) Warn(string, ...zap.Field)  {}
func (defaultLogger) Error(string, ...zap.Field) {}

// ProducerConfig 发布者配置
type ProducerConfig struct {
	Brokers []string             // Kafka 集群地址
	Logger  logger.Logger        // 日志
	Setting func(*sarama.Config) // Sarama 配置回调
}

// ConsumeConfig 消费者配置
type ConsumeConfig struct {
	Brokers []string                    // Kafka 集群地址
	GroupID string                      // 消费组 ID
	Topics  []string                    // 订阅的 Topic 列表
	Handler sarama.ConsumerGroupHandler // 消息处理 Handler
	Logger  logger.Logger               // 日志
	Setting func(*sarama.Config)        // Sarama 配置回调
}

// consumeRetryBackoff 消费者重连的退避等待时长（固定等待）
const consumeRetryBackoff = 2 * time.Second

// Client Kafka 客户端。
type Client struct {
	ctx           context.Context
	cancel        context.CancelFunc
	logger        logger.Logger
	producer      sarama.SyncProducer
	consumerGroup sarama.ConsumerGroup
}

// NewConsume 初始化 Kafka 消费者
func NewConsume(cfg ConsumeConfig) (*Client, error) {
	if len(cfg.Brokers) == 0 {
		return nil, fmt.Errorf("brokers 不能为空")
	}
	if len(cfg.Topics) == 0 {
		return nil, fmt.Errorf("topics 不能为空")
	}
	if cfg.Handler == nil {
		return nil, fmt.Errorf("handler 不能为空")
	}

	client := &Client{logger: cfg.Logger}
	if client.logger == nil {
		client.logger = defaultLogger{}
	}
	client.ctx, client.cancel = context.WithCancel(context.Background())

	// 1. 配置消费者
	config := sarama.NewConfig()
	if cfg.Setting != nil {
		cfg.Setting(config)
	}
	// config.Consumer.Return.Errors = true

	// 2. 创建消费者组
	consumerGroup, err := sarama.NewConsumerGroup(cfg.Brokers, cfg.GroupID, config)
	if err != nil {
		return nil, fmt.Errorf("创建消费者组失败: %w", err)
	}
	client.consumerGroup = consumerGroup

	// 3. 启动一个独立的 goroutine 来处理错误
	// go func() {
	// 	for err := range consumerGroup.Errors() {
	// 		// 处理错误：记录日志、上报监控、触发报警
	// 		client.logger.Error("队列消费者错误", zap.Error(err))
	// 		// 这里可以增加业务逻辑，比如发生错误时重新创建消费者
	// 	}
	// }()

	// 4. 启动消费者（订阅该商户的Topic）
	//    注意: sarama.ConsumerGroup.Consume 是阻塞方法,
	//    它会一直阻塞直到 ctx 被取消(此时返回 nil),或在消费过程中发生错误(返回 error)。
	//    因此需要循环调用以在出错后重新消费。
	//    为避免 broker 持续不可用时造成无退避的忙循环,出错后固定等待 2s 再重试;
	//    收到 ctx 取消通知即刻退出。
	go func() {
		for {
			if err := consumerGroup.Consume(client.ctx, cfg.Topics, cfg.Handler); err != nil {
				client.logger.Error("队列消费错误", zap.Error(err))
				client.delayRetry(consumeRetryBackoff)
				continue
			}
			// Consume 正常返回（ctx 被取消），关闭并退出
			if client.ctx.Err() != nil {
				consumerGroup.Close()
				return
			}
		}
	}()

	return client, nil
}

// NewProducer 初始化 Kafka 生产者
func NewProducer(cfg ProducerConfig) (*Client, error) {
	if len(cfg.Brokers) == 0 {
		return nil, fmt.Errorf("brokers 不能为空")
	}

	client := &Client{logger: cfg.Logger}
	if client.logger == nil {
		client.logger = defaultLogger{}
	}
	client.ctx, client.cancel = context.WithCancel(context.Background())

	// 1. 配置生产者（使用默认分区器，按Key Hash分区）
	config := sarama.NewConfig()
	config.Producer.RequiredAcks = sarama.WaitForAll        // 等待所有副本确认
	config.Producer.Retry.Max = 5                           // 重试次数
	config.Producer.Return.Successes = true                 // 必须开启才能获取发送结果
	config.Producer.Partitioner = sarama.NewHashPartitioner // 关键：按Key Hash分区
	if cfg.Setting != nil {
		cfg.Setting(config)
	}

	var err error
	// 2. 连接Kafka集群
	client.producer, err = sarama.NewSyncProducer(cfg.Brokers, config)
	if err != nil {
		return nil, fmt.Errorf("创建生产者失败: %w", err)
	}

	return client, nil
}

// SendMessage 发送消息
func (c *Client) SendMessage(msg *sarama.ProducerMessage) (partition int32, offset int64, err error) {
	if msg == nil {
		return 0, 0, fmt.Errorf("消息不能为空")
	}
	partition, offset, err = c.producer.SendMessage(msg)
	if err != nil {
		c.logger.Error("发送失败", zap.Error(err))
	} else {
		c.logger.Debug("发送成功",
			zap.String("topic", msg.Topic),
			zap.Int32("partition", partition),
			zap.Int64("offset", offset),
			zap.Any("key", msg.Key))
	}
	return
}

// delayRetry 退避等待,期间可被 ctx 取消立即返回
func (c *Client) delayRetry(d time.Duration) {
	select {
	case <-time.After(d):
	case <-c.ctx.Done():
	}
}

// Close 关闭 Kafka 连接
func (c *Client) Close() error {
	if c != nil {
		if c.cancel != nil {
			c.cancel() // 触发消费 goroutine 退出并关闭消费者组
		}
		if c.consumerGroup != nil {
			c.consumerGroup.Close()
		}
		if c.producer != nil {
			c.producer.Close()
		}
	}
	return nil
}
