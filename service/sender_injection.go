// Package service — sender_injection.go
// SetSender methods allow deferred injection of the WS Sender.
// This breaks the circular dependency: services are created before the WS
// connection is built, and the connection needs the dispatcher (which needs services).
package service

// SetSender injects the WS sender into ChatService after construction.
func (s *ChatService) SetSender(sender Sender) { s.sender = sender }

// SetSender injects the WS sender into CapsuleService after construction.
func (s *CapsuleService) SetSender(sender Sender) { s.sender = sender }

// SetSender injects the WS sender into PresenceService after construction.
func (s *PresenceService) SetSender(sender Sender) { s.sender = sender }

// SetSender injects the WS sender into ScreenShareService after construction.
func (s *ScreenShareService) SetSender(sender Sender) { s.sender = sender }

// SetSender injects the WS sender into VoiceService after construction.
func (v *VoiceService) SetSender(sender Sender) { v.sender = sender }
