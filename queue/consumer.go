package queue

import (
	"context"
	"fmt"
	"time"

	"github.com/IBM/sarama"
	"github.com/minjieguo/infra/logger"
	"go.uber.org/zap"
)

// ConsumeConfig 消费者配置
type ConsumeConfig struct {
	Brokers []string                    // Kafka 集群地址
	GroupID string                      // 消费组 ID
	Topics  []string                    // 订阅的 Topic 列表
	Handler sarama.ConsumerGroupHandler // 消息处理 Handler
	Logger  logger.Logger               // 日志
	Config  *sarama.Config              // Sarama 配置,完全由外部传入(nil 则使用 sarama.NewConfig() 默认值)
}

// consumeRetryBackoff 消费者重连的退避等待时长（固定等待）
const consumeRetryBackoff = 2 * time.Second

// Consumer Kafka 消费者客户端。
type Consumer struct {
	ctx           context.Context
	cancel        context.CancelFunc
	logger        logger.Logger
	consumerGroup sarama.ConsumerGroup
}

// NewConsumer 初始化 Kafka 消费者
func NewConsumer(cfg ConsumeConfig) (*Consumer, error) {
	if len(cfg.Brokers) == 0 {
		return nil, fmt.Errorf("brokers 不能为空")
	}
	if len(cfg.Topics) == 0 {
		return nil, fmt.Errorf("topics 不能为空")
	}
	if cfg.Handler == nil {
		return nil, fmt.Errorf("handler 不能为空")
	}

	c := &Consumer{logger: cfg.Logger}
	if c.logger == nil {
		c.logger = defaultLogger{}
	}
	c.ctx, c.cancel = context.WithCancel(context.Background())

	// 1. 配置消费者(完全由外部传入;未传入时使用 sarama 默认配置)
	config := cfg.Config
	if config == nil {
		config = sarama.NewConfig()
	}

	// 2. 创建消费者组
	consumerGroup, err := sarama.NewConsumerGroup(cfg.Brokers, cfg.GroupID, config)
	if err != nil {
		return nil, fmt.Errorf("创建消费者组失败: %w", err)
	}
	c.consumerGroup = consumerGroup

	// 3. 启动一个独立的 goroutine 来处理错误
	//    这里必须消费 Errors() 通道,否则开启 Return.Errors 后错误会被静默丢弃;
	//    错误通过 logger 输出,可供监控/报警使用。
	go func() {
		for err := range consumerGroup.Errors() {
			c.logger.Error("队列消费者错误", zap.Error(err))
		}
	}()

	// 4. 启动消费者（订阅该商户的Topic）
	//    注意: sarama.ConsumerGroup.Consume 是阻塞方法,
	//    它会一直阻塞直到 ctx 被取消(此时返回 nil),或在消费过程中发生错误(返回 error)。
	//    因此需要循环调用以在出错后重新消费。
	//    为避免 broker 持续不可用时造成无退避的忙循环,出错后固定等待 2s 再重试;
	//    收到 ctx 取消通知即刻退出。
	go func() {
		for {
			if err := consumerGroup.Consume(c.ctx, cfg.Topics, cfg.Handler); err != nil {
				c.logger.Error("队列消费错误", zap.Error(err))
				c.delayRetry(consumeRetryBackoff)
				continue
			}
			// Consume 正常返回（ctx 被取消），关闭并退出
			if c.ctx.Err() != nil {
				consumerGroup.Close()
				return
			}
		}
	}()

	return c, nil
}

// delayRetry 退避等待,期间可被 ctx 取消立即返回
func (c *Consumer) delayRetry(d time.Duration) {
	select {
	case <-time.After(d):
	case <-c.ctx.Done():
	}
}

// Close 关闭 Kafka 消费者连接
func (c *Consumer) Close() error {
	if c != nil {
		if c.cancel != nil {
			c.cancel() // 触发消费 goroutine 退出并关闭消费者组
		}
		if c.consumerGroup != nil {
			return c.consumerGroup.Close()
		}
	}
	return nil
}
