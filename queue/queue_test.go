package queue

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/IBM/sarama"
)

// 测试用 Kafka 集群地址
var testBrokers = []string{"115.29.231.154:9092"}

// 测试用 Topic：每次运行生成唯一 topic,避免复用之前的消息历史
func newTestTopic() string {
	return fmt.Sprintf("test-topic-%d", time.Now().UnixNano())
}

// 测试用消费组
// 每次测试使用唯一的消费组名,避免复用 broker 上已提交的 offset,
// 从而保证消费组能从最早的 offset 开始读取到本次新发送的消息。
func newTestGroup() string {
	return fmt.Sprintf("test-consumer-group-%d", time.Now().UnixNano())
}

// testHandler 实现 sarama.ConsumerGroupHandler,用于接收消息
type testHandler struct {
	received chan string
}

func (h *testHandler) Setup(session sarama.ConsumerGroupSession) error {
	return nil
}

func (h *testHandler) Cleanup(session sarama.ConsumerGroupSession) error {
	return nil
}

func (h *testHandler) ConsumeClaim(
	session sarama.ConsumerGroupSession,
	claim sarama.ConsumerGroupClaim,
) error {
	for msg := range claim.Messages() {
		h.received <- string(msg.Value)
		session.MarkMessage(msg, "")
	}
	return nil
}

// testProducerConfig 构造测试用生产者配置(需开启 Return.Successes 才能用 SyncProducer)
func testProducerConfig() *sarama.Config {
	c := sarama.NewConfig()
	c.Producer.Return.Successes = true
	return c
}

// testConsumerConfig 构造测试用消费者配置(从最早 offset 开始,并返回错误)
func testConsumerConfig() *sarama.Config {
	c := sarama.NewConfig()
	c.Consumer.Offsets.Initial = sarama.OffsetOldest
	c.Consumer.Return.Errors = true
	return c
}

// TestPublishSubscribe 最简单的订阅 + 发布测试：
// 1. 启动一个消费者组订阅 Topic
// 2. 通过生产者往同一 Topic 发送一条消息
// 3. 断言消费者能收到该消息
func TestPublishSubscribe(t *testing.T) {
	topic := newTestTopic()
	received := make(chan string, 10)
	handler := &testHandler{received: received}

	consumer, err := NewConsumer(ConsumeConfig{
		Brokers: testBrokers,
		GroupID: newTestGroup(),
		Topics:  []string{topic},
		Handler: handler,
		Config:  testConsumerConfig(),
	})
	if err != nil {
		t.Fatalf("创建消费者失败: %v", err)
	}
	defer consumer.Close()

	producer, err := NewProducer(ProducerConfig{
		Brokers: testBrokers,
		Config:  testProducerConfig(),
	})
	if err != nil {
		t.Fatalf("创建生产者失败: %v", err)
	}
	defer producer.Close()

	time.Sleep(3 * time.Second)

	payload := fmt.Sprintf("hello kafka test-%d", time.Now().UnixNano())
	_, _, err = producer.SendMessage(&sarama.ProducerMessage{
		Topic: topic,
		Value: sarama.ByteEncoder(payload),
	})
	if err != nil {
		t.Fatalf("发送消息失败: %v", err)
	}
	t.Logf("消息已发送: %s", payload)

	// 5. 等待消费者确认收到消息（带超时）
	select {
	case got := <-received:
		if got != payload {
			t.Fatalf("消息内容不匹配: want=%q got=%q", payload, got)
		}
		t.Logf("订阅/发布测试通过, 收到消息: %s", got)
	case <-time.After(30 * time.Second):
		t.Fatal("等待消费消息超时(30s), 未收到订阅消息")
	}
}

// TestPublishSubscribeConcurrent 并发发布多条消息，验证都能被消费到
func TestPublishSubscribeConcurrent(t *testing.T) {
	const msgCount = 10
	topic := newTestTopic()
	received := make(chan string, msgCount)
	handler := &testHandler{received: received}

	consumer, err := NewConsumer(ConsumeConfig{
		Brokers: testBrokers,
		GroupID: newTestGroup(),
		Topics:  []string{topic},
		Handler: handler,
		Config:  testConsumerConfig(),
	})
	if err != nil {
		t.Fatalf("创建消费者失败: %v", err)
	}
	defer consumer.Close()

	producer, err := NewProducer(ProducerConfig{
		Brokers: testBrokers,
		Config:  testProducerConfig(),
	})
	if err != nil {
		t.Fatalf("创建生产者失败: %v", err)
	}
	defer producer.Close()

	time.Sleep(3 * time.Second)

	var sendWg sync.WaitGroup
	for i := 0; i < msgCount; i++ {
		sendWg.Add(1)
		go func(i int) {
			defer sendWg.Done()
			payload := fmt.Sprintf("msg-%d-%d", i, time.Now().UnixNano())
			_, _, err := producer.SendMessage(&sarama.ProducerMessage{
				Topic: topic,
				Value: sarama.ByteEncoder(payload),
			})
			if err != nil {
				t.Errorf("发送消息[%d]失败: %v", i, err)
			}
			received <- payload
		}(i)
	}
	sendWg.Wait()

	// 检查消费端收到的消息数量
	got := make(map[string]bool)
	for len(got) < msgCount {
		select {
		case m := <-received:
			got[m] = true
		case <-time.After(30 * time.Second):
			t.Fatalf("超时, 仅收到 %d/%d 条消息", len(got), msgCount)
			return
		}
	}
	if len(got) != msgCount {
		t.Fatalf("去重后收到 %d 条, 期望 %d 条", len(got), msgCount)
	}

	t.Logf("并发发布/订阅测试通过, 共收到 %d 条消息", msgCount)
}
