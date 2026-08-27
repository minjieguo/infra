package queue

import (
	"fmt"

	"github.com/IBM/sarama"
	"github.com/minjieguo/infra/logger"
	"go.uber.org/zap"
)

// ProducerConfig 发布者(Kafka 生产者)配置
type ProducerConfig struct {
	Brokers []string       // Kafka 集群地址
	Logger  logger.Logger  // 日志
	Config  *sarama.Config // Sarama 配置,完全由外部传入(nil 则使用 sarama.NewConfig() 默认值)
}

// Producer Kafka 生产者客户端。
type Producer struct {
	logger   logger.Logger
	producer sarama.SyncProducer
}

// NewProducer 初始化 Kafka 生产者
func NewProducer(cfg ProducerConfig) (*Producer, error) {
	if len(cfg.Brokers) == 0 {
		return nil, fmt.Errorf("brokers 不能为空")
	}

	p := &Producer{logger: cfg.Logger}
	if p.logger == nil {
		p.logger = defaultLogger{}
	}

	// 配置完全由外部传入;未传入时使用 sarama 默认配置。
	config := cfg.Config
	if config == nil {
		config = sarama.NewConfig()
	}

	// 连接 Kafka 集群
	producer, err := sarama.NewSyncProducer(cfg.Brokers, config)
	if err != nil {
		return nil, fmt.Errorf("创建生产者失败: %w", err)
	}
	p.producer = producer

	return p, nil
}

// SendMessage 发送消息
func (p *Producer) SendMessage(msg *sarama.ProducerMessage) (partition int32, offset int64, err error) {
	if p == nil || p.producer == nil {
		return 0, 0, fmt.Errorf("生产者未初始化")
	}
	if msg == nil {
		return 0, 0, fmt.Errorf("消息不能为空")
	}

	partition, offset, err = p.producer.SendMessage(msg)
	if err != nil {
		p.logger.Error("发送失败", zap.Error(err))
	} else {
		p.logger.Debug("发送成功",
			zap.String("topic", msg.Topic),
			zap.Int32("partition", partition),
			zap.Int64("offset", offset),
			zap.Any("key", msg.Key))
	}
	return
}

// Close 关闭 Kafka 生产者连接
func (p *Producer) Close() error {
	if p != nil && p.producer != nil {
		return p.producer.Close()
	}
	return nil
}
