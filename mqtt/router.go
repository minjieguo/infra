package mqtt

import (
	"github.com/eclipse/paho.golang/packets"
	"github.com/eclipse/paho.golang/paho"
)

// Router 包装 paho.StandardRouter，对外只暴露业务级回调
type Router struct {
	router *paho.StandardRouter
}

// New 创建一个封装后的 Router
func NewStandardRouter() *Router {
	return &Router{
		router: paho.NewStandardRouter(),
	}
}

// Register 注册一条路由，把 Paho 的 Publish 转成 ReceiveHandler
func (r *Router) RegisterHandler(topic string, handler ReceiveHandler) {
	r.router.RegisterHandler(topic, func(p *paho.Publish) {
		handler(p.Topic, p.Payload)
	})
}

// Route 供 autopaho 的 OnPublishReceived 调用
func (r *Router) Route(p *packets.Publish) {
	r.router.Route(p)
}
