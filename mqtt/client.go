package mqtt

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"time"

	"github.com/eclipse/paho.golang/autopaho"
	"github.com/eclipse/paho.golang/paho"
)

// Config MQTT 配置
type Config struct {
	Host           string                             // 主机或IP地址
	Port           int                                // 端口 默认1883
	Username       string                             // 用户名,可为空
	Password       string                             // 密码,可为空
	ReceiveHandler func(string, []byte) (bool, error) // 消息接收事件
	Logger         Logger                             // 日志
}

var (
	client *autopaho.ConnectionManager
	logger Logger
)

// New 初始化 MQTT 连接
func New(cfg Config) error {

	if cfg.Logger == nil {
		logger = defaultLogger{}
	} else {
		logger = cfg.Logger
	}

	serverURL, err := url.Parse(fmt.Sprintf("mqtt://%s:%d", cfg.Host, cfg.Port))
	if err != nil {
		return fmt.Errorf("MQTT URL 解析失败: %w", err)
	}

	ctx := context.Background()

	client, err = autopaho.NewConnection(ctx, autopaho.ClientConfig{
		ServerUrls:        []*url.URL{serverURL},
		ConnectUsername:   cfg.Username,
		ConnectPassword:   []byte(cfg.Password),
		KeepAlive:         30,
		ConnectRetryDelay: 5 * time.Second,
		OnConnectionUp: func(cm *autopaho.ConnectionManager, _ *paho.Connack) {
			logger.Info("MQTT Connected")
		},
		OnConnectError: func(err error) {
			logger.Error("MQTT Connection Error:%s", err.Error())
		},
		ClientConfig: paho.ClientConfig{
			ClientID: fmt.Sprintf("go_mqtt_client_%d", time.Now().UnixMilli()),
			OnPublishReceived: []func(pr paho.PublishReceived) (bool, error){
				func(pr paho.PublishReceived) (bool, error) {
					if pr.Packet == nil {
						return true, nil
					}
					if cfg.ReceiveHandler != nil {
						return cfg.ReceiveHandler(pr.Packet.Topic, pr.Packet.Payload)
					}
					return true, nil
				},
			},
		},
	})
	if err != nil {
		return err
	}

	waitCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if err = client.AwaitConnection(waitCtx); err != nil {
		logger.Warn("MQTT Initial Connection Timeout, will keep reconnecting in background: %s", err.Error())
	}

	return nil
}

// Close 关闭 MQTT 连接
func Close() error {
	if client != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := client.Disconnect(ctx); err != nil {
			return err
		}
	}
	return nil
}

// Subscribe 订阅主题
func Subscribe(topic string, qos byte) error {
	if client == nil {
		return errors.New("client not initialized")
	}
	_, err := client.Subscribe(context.Background(), &paho.Subscribe{
		Subscriptions: []paho.SubscribeOptions{{Topic: topic, QoS: qos}},
	})
	if err != nil {
		return err
	}
	return nil
}

// Publish 发布消息
func Publish(topic string, payload []byte, qos byte, retain bool) error {
	_, err := client.Publish(context.Background(), &paho.Publish{
		Topic:   topic,
		QoS:     qos,
		Retain:  retain,
		Payload: payload,
	})
	if err != nil {
		return err
	}
	return nil
}
