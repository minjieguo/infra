package mqtt

// Broker MQTT 接口。
type Broker interface {
	Subscribe(topic string, qos byte) error
	Publish(topic string, payload []byte, qos byte, retain bool) error
	Close() error
}
