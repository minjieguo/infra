package mqtt

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"sync"
	"time"

	"github.com/eclipse/paho.golang/autopaho"
	"github.com/eclipse/paho.golang/paho"
	"github.com/minjieguo/infra/logger"
	"go.uber.org/zap"
)

// 消息接收事件
type ReceiveHandler func(string, []byte) (bool, error)

// Config MQTT 配置
type Config struct {
	Host           string         // 主机或IP地址
	Port           int            // 端口 默认1883
	Username       string         // 用户名,可为空
	Password       string         // 密码,可为空
	ReceiveHandler ReceiveHandler // 消息接收事件
	Router         *Router        // 路由模式
	Logger         logger.Logger  // 日志
}

// Client MQTT 客户端。
type Client struct {
	client *autopaho.ConnectionManager
	logger logger.Logger

	mu   sync.Mutex
	subs map[string]byte // 已订阅主题, 重连后用于重新订阅
}

// New 初始化 MQTT 连接
func New(cfg Config) (*Client, error) {
	// logger := cfg.Logger
	// if logger == nil {
	// 	logger = defaultLogger{}
	// }

	mqttClient := &Client{
		logger: cfg.Logger,
		subs:   make(map[string]byte),
	}

	serverURL, err := url.Parse(fmt.Sprintf("mqtt://%s:%d", cfg.Host, cfg.Port))
	if err != nil {
		return nil, fmt.Errorf("MQTT URL 解析失败: %w", err)
	}

	ctx := context.Background()

	client, err := autopaho.NewConnection(ctx, autopaho.ClientConfig{
		ServerUrls:       []*url.URL{serverURL},
		ConnectUsername:  cfg.Username,
		ConnectPassword:  []byte(cfg.Password),
		KeepAlive:        30,
		ReconnectBackoff: autopaho.NewConstantBackoff(5 * time.Second),
		OnConnectionUp: func(cm *autopaho.ConnectionManager, _ *paho.Connack) {
			mqttClient.logger.Info("MQTT Connected")
			// 连接(含重连)建立后重新订阅, 防止会话丢失后收不到消息。
			// 注意: 该回调不能阻塞, 因此异步执行。
			go mqttClient.resubscribe(cm)
		},
		OnConnectError: func(err error) {
			mqttClient.logger.Error("MQTT Connection Error", zap.Error(err))
		},
		ClientID: fmt.Sprintf("go_mqtt_client_%d", time.Now().UnixMilli()),
		OnPublishReceived: []func(pr paho.PublishReceived) (bool, error){
			func(pr paho.PublishReceived) (bool, error) {
				if pr.Packet == nil {
					return true, nil
				}
				if cfg.Router != nil {
					cfg.Router.Route(pr.Packet.Packet())
				}

				if cfg.ReceiveHandler != nil {
					return cfg.ReceiveHandler(pr.Packet.Topic, pr.Packet.Payload)
				}
				return true, nil
			},
		},
	})
	if err != nil {
		return nil, err
	}
	mqttClient.client = client

	waitCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if err = mqttClient.client.AwaitConnection(waitCtx); err != nil {
		mqttClient.logger.Warn("MQTT Initial Connection Timeout, will keep reconnecting in background", zap.Error(err))
	}

	return mqttClient, nil
}

// Close 关闭 MQTT 连接
func (c *Client) Close() error {
	if c != nil && c.client != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := c.client.Disconnect(ctx); err != nil {
			return err
		}
	}
	return nil
}

// Subscribe 订阅主题
// 订阅意图会被记录, 连接断开重连后由 OnConnectionUp 自动重新订阅。
func (c *Client) Subscribe(topic string, qos byte) error {
	if c == nil || c.client == nil {
		return errors.New("client not initialized")
	}

	// 先记录订阅意图, 保证即使当前未连接, 重连后也能重新订阅。
	c.mu.Lock()
	c.subs[topic] = qos
	c.mu.Unlock()

	_, err := c.client.Subscribe(context.Background(), &paho.Subscribe{
		Subscriptions: []paho.SubscribeOptions{{Topic: topic, QoS: qos}},
	})
	if err != nil {
		// 连接可能尚未建立, 此时订阅会在重连后由 resubscribe 补上。
		c.logger.Warn("MQTT Subscribe Failed (will retry on reconnect)", zap.Error(err))
		return err
	}
	return nil
}

// Unsubscribe 取消订阅主题
func (c *Client) Unsubscribe(topic string) error {
	if c == nil || c.client == nil {
		return errors.New("client not initialized")
	}

	c.mu.Lock()
	delete(c.subs, topic)
	c.mu.Unlock()

	_, err := c.client.Unsubscribe(context.Background(), &paho.Unsubscribe{
		Topics: []string{topic},
	})
	if err != nil {
		return err
	}
	return nil
}

// resubscribe 重新订阅所有已记录的主题, 用于连接(重连)建立后恢复订阅
func (c *Client) resubscribe(cm *autopaho.ConnectionManager) {
	c.mu.Lock()
	if len(c.subs) == 0 {
		c.mu.Unlock()
		return
	}
	subs := make([]paho.SubscribeOptions, 0, len(c.subs))
	for topic, qos := range c.subs {
		subs = append(subs, paho.SubscribeOptions{Topic: topic, QoS: qos})
	}
	c.mu.Unlock()

	if _, err := cm.Subscribe(context.Background(), &paho.Subscribe{Subscriptions: subs}); err != nil {
		c.logger.Error("MQTT Resubscribe Failed", zap.Error(err))
		return
	}
	c.logger.Info("MQTT Subscribed", zap.Int("count", len(subs)))
}

// Publish 发布消息
func (c *Client) Publish(topic string, payload []byte, qos byte, retain bool, properties *paho.PublishProperties) error {
	if c == nil || c.client == nil {
		return errors.New("client not initialized")
	}
	_, err := c.client.Publish(context.Background(), &paho.Publish{
		Topic:      topic,
		QoS:        qos,
		Retain:     retain,
		Payload:    payload,
		Properties: properties,
	})
	if err != nil {
		return err
	}
	return nil
}

// PublishViaQueue 发布队列消息
func (c *Client) PublishViaQueue(topic string, payload []byte, qos byte, retain bool, properties *paho.PublishProperties) error {
	if c == nil || c.client == nil {
		return errors.New("client not initialized")
	}
	return c.client.PublishViaQueue(context.Background(), &autopaho.QueuePublish{
		Publish: &paho.Publish{Topic: topic,
			QoS:        qos,
			Retain:     retain,
			Payload:    payload,
			Properties: properties,
		},
	})
}
