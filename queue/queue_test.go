package queue

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/minjieguo/infra/internal/testenv"
	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/twmb/franz-go/pkg/kmsg"
)

// 测试用 Kafka 集群地址,从仓库根目录的 .env 读取;
// 未配置 QUEUE_BROKER 时跳过测试。
//
// .env 示例：
//
//	QUEUE_BROKER=127.0.0.1:9092
func testBrokers(t *testing.T) []string {
	t.Helper()

	broker := testenv.Lookup("QUEUE_BROKER")
	if broker == "" {
		t.Skip("未配置 QUEUE_BROKER(参考 .env), 跳过 Kafka 集成测试")
	}
	return []string{broker}
}

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

// createTestTopic 通过 admin API 创建 topic(测试 broker 未开启自动建 topic),
// 若已存在则视为成功。
func createTestTopic(t *testing.T, broker, topic string) {
	t.Helper()

	client, err := kgo.NewClient(kgo.SeedBrokers(broker))
	if err != nil {
		t.Fatalf("创建 admin 客户端失败: %v", err)
	}
	defer client.Close()

	req := kmsg.NewPtrCreateTopicsRequest()
	reqTopic := kmsg.NewCreateTopicsRequestTopic()
	reqTopic.Topic = topic
	reqTopic.NumPartitions = 1
	reqTopic.ReplicationFactor = 1
	req.Topics = append(req.Topics, reqTopic)

	resp, err := req.RequestWith(context.Background(), client)
	if err != nil {
		t.Fatalf("创建 topic 失败: %v", err)
	}
	for _, rt := range resp.Topics {
		if rt.Topic != topic {
			continue
		}
		// 已存在(TOPIC_ALREADY_EXISTS=36)视为成功。
		if rt.ErrorCode != 0 && rt.ErrorCode != 36 {
			msg := ""
			if rt.ErrorMessage != nil {
				msg = *rt.ErrorMessage
			}
			t.Fatalf("创建 topic 失败: %s", msg)
		}
	}
}

// newTestHandler 构造把消息内容投递到 channel 的 Handler
func newTestHandler(received chan string) Handler {
	return func(_ context.Context, record *kgo.Record) error {
		received <- string(record.Value)
		return nil
	}
}

// consumeTestOpts 消费配置:从最早 offset 开始,保证读到本次新发送的消息。
func consumeTestOpts() []kgo.Opt {
	return []kgo.Opt{
		kgo.ConsumeResetOffset(kgo.NewOffset().AtStart()),
	}
}

// TestPublishSubscribe 最简单的订阅 + 发布测试：
// 1. 用同一个 Client 订阅 Topic 并启动消费
// 2. 通过同一个 Client 往 Topic 发送一条消息
// 3. 断言消费者能收到该消息
func TestPublishSubscribe(t *testing.T) {
	brokers := testBrokers(t)
	topic := newTestTopic()
	createTestTopic(t, brokers[0], topic)
	received := make(chan string, 10)

	client, err := New(Config{
		Brokers: brokers,
		GroupID: newTestGroup(),
		Opts:    consumeTestOpts(),
	})
	if err != nil {
		t.Fatalf("创建客户端失败: %v", err)
	}
	defer client.Close()

	if err := client.Consume(newTestHandler(received), topic); err != nil {
		t.Fatalf("启动消费失败: %v", err)
	}

	time.Sleep(3 * time.Second)

	payload := fmt.Sprintf("hello kafka test-%d", time.Now().UnixNano())
	if _, _, err := client.Send(context.Background(), &kgo.Record{
		Topic: topic,
		Value: []byte(payload),
	}); err != nil {
		t.Fatalf("发送消息失败: %v", err)
	}
	t.Logf("消息已发送: %s", payload)

	// 等待消费者确认收到消息（带超时）
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
	brokers := testBrokers(t)
	topic := newTestTopic()
	createTestTopic(t, brokers[0], topic)
	received := make(chan string, msgCount)

	client, err := New(Config{
		Brokers: brokers,
		GroupID: newTestGroup(),
		Opts:    consumeTestOpts(),
	})
	if err != nil {
		t.Fatalf("创建客户端失败: %v", err)
	}
	defer client.Close()

	if err := client.Consume(newTestHandler(received), topic); err != nil {
		t.Fatalf("启动消费失败: %v", err)
	}

	time.Sleep(3 * time.Second)

	var sendWg sync.WaitGroup
	for i := 0; i < msgCount; i++ {
		sendWg.Add(1)
		go func(i int) {
			defer sendWg.Done()
			payload := fmt.Sprintf("msg-%d-%d", i, time.Now().UnixNano())
			_, _, err := client.Send(context.Background(), &kgo.Record{
				Topic: topic,
				Value: []byte(payload),
			})
			if err != nil {
				t.Errorf("发送消息[%d]失败: %v", i, err)
			}
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
