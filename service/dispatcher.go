// Package service — dispatcher.go
// Dispatcher routes inbound WS envelopes to registered handlers.
// It implements ws.MessageHandler so it plugs directly into ws.Connection.
package service

import (
	"log"

	"umbra/client/ws"
)

// InboundHandler processes a specific message type.
type InboundHandler interface {
	Handle(env ws.Envelope)
}

// InboundHandlerFunc adapts a function to InboundHandler.
type InboundHandlerFunc func(ws.Envelope)

func (f InboundHandlerFunc) Handle(env ws.Envelope) { f(env) }

// Dispatcher maps message types to handlers and implements ws.MessageHandler.
type Dispatcher struct {
	handlers map[string]InboundHandler
	fallback InboundHandler // called for unregistered types (optional)
}

// NewDispatcher creates a Dispatcher with an optional fallback.
func NewDispatcher(fallback InboundHandler) *Dispatcher {
	return &Dispatcher{
		handlers: make(map[string]InboundHandler),
		fallback: fallback,
	}
}

// Register binds a type string to a handler. Panics on duplicate.
func (d *Dispatcher) Register(msgType string, h InboundHandler) {
	if _, exists := d.handlers[msgType]; exists {
		panic("dispatcher: duplicate handler for " + msgType)
	}
	d.handlers[msgType] = h
}

// OnMessage satisfies ws.MessageHandler.
func (d *Dispatcher) OnMessage(env ws.Envelope) {
	h, ok := d.handlers[env.Type]
	if !ok {
		if d.fallback != nil {
			d.fallback.Handle(env)
		} else {
			log.Printf("[dispatcher] no handler for type %q", env.Type)
		}
		return
	}
	h.Handle(env)
}
