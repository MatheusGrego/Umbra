// Package main — app.go
// App is the Wails-bound struct. Its only job: translate JS calls → service calls
// and emit typed events back to the frontend via EventsEmit.
// Zero crypto, zero WS, zero business logic lives here.
package main

import (
	"context"
	"log"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	gc "umbra/client/crypto"
	"umbra/client/service"
)

// App is bound to JS. Every exported method becomes callable as window.go.main.App.*
type App struct {
	ctx         context.Context
	emitter     *deferredEmitter
	chat        *service.ChatService
	capsule     *service.CapsuleService
	presence    *service.PresenceService
	screenshare *service.ScreenShareService
	voice       *service.VoiceService
}

// NewApp constructs the App with all dependencies injected.
func NewApp(
	chat *service.ChatService,
	capsule *service.CapsuleService,
	presence *service.PresenceService,
	screenshare *service.ScreenShareService,
	voice *service.VoiceService,
	emitter *deferredEmitter,
) *App {
	return &App{
		chat:        chat,
		capsule:     capsule,
		presence:    presence,
		screenshare: screenshare,
		voice:       voice,
		emitter:     emitter,
	}
}

// startup is called by Wails once the window is ready and the context is live.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	// Wire the real Wails context into the emitter — all buffered Emit calls now flow.
	a.emitter.setCtx(&a.ctx)

	if err := applyWindowProtection(); err != nil {
		log.Printf("[app] window protection unavailable: %v", err)
	}
}

// ---- Chat ---------------------------------------------------------------

// SendMessage encrypts and sends a plaintext chat message to toUserID.
func (a *App) SendMessage(toUserID, text string) error {
	return a.chat.Send(toUserID, text)
}

// ---- Capsule ------------------------------------------------------------

// SendCapsule encrypts plaintext and schedules delivery after releaseInSecs seconds.
func (a *App) SendCapsule(toUserID, text string, releaseInSecs int) error {
	return a.capsule.Send(toUserID, text, releaseInSecs)
}

// ---- Presence -----------------------------------------------------------

// GetOnlinePeers returns IDs of currently online known contacts.
func (a *App) GetOnlinePeers() []string {
	return a.presence.OnlinePeers()
}

// GetMyID returns this client's user ID for display in the sidebar.
func (a *App) GetMyID() string {
	return a.presence.MyUserID()
}

// ---- Invites ------------------------------------------------------------

// CreateInvite asks the server to issue a one-time invite token.
func (a *App) CreateInvite() error {
	return a.presence.RequestInviteToken()
}

// ResolveInvite submits a token received out-of-band, completing the key exchange.
func (a *App) ResolveInvite(token string) error {
	return a.presence.ResolveInvite(token)
}

// ---- Screen Share -------------------------------------------------------

// StartScreenShare initiates a screen share offer to peerID.
func (a *App) StartScreenShare(peerID string) error {
	return a.screenshare.StartShare(peerID)
}

// AcceptScreenShare accepts an incoming screen share from peerID.
func (a *App) AcceptScreenShare(peerID string) {
	a.screenshare.AcceptShare(peerID)
}

// RejectScreenShare declines an incoming screen share from peerID.
func (a *App) RejectScreenShare(peerID string) error {
	return a.screenshare.RejectShare(peerID)
}

// StopScreenShare tears down the active screen share.
func (a *App) StopScreenShare() {
	a.screenshare.StopShare()
}

// ---- Voice Chat ---------------------------------------------------------

// StartVoiceCall initiates a voice call offer to peerID.
func (a *App) StartVoiceCall(peerID string) error {
	return a.voice.StartCall(peerID)
}

// AcceptVoiceCall accepts an incoming voice call from peerID.
func (a *App) AcceptVoiceCall(peerID string) {
	a.voice.AcceptCall(peerID)
}

// RejectVoiceCall declines an incoming voice call from peerID.
func (a *App) RejectVoiceCall(peerID string) error {
	return a.voice.RejectCall(peerID)
}

// HangupVoice tears down the active voice call.
func (a *App) HangupVoice() {
	a.voice.Hangup()
}

// ToggleMute toggles voice call microphone mute state.
func (a *App) ToggleMute() bool {
	return a.voice.ToggleMute()
}

// ---- Peers --------------------------------------------------------------

// GetAllPeers returns all known contacts with their nickname and session status.
// Used by the frontend to bootstrap the peer list.
func (a *App) GetAllPeers() []gc.PeerInfo {
	return a.presence.AllPeers()
}

// SetNickname assigns a local display name to a contact.
func (a *App) SetNickname(userID, nickname string) error {
	return a.presence.SetPeerNickname(userID, nickname)
}

// ---- Helpers ------------------------------------------------------------

// showError surfaces an error to the user via a native dialog.
func showError(ctx context.Context, msg string) {
	runtime.MessageDialog(ctx, runtime.MessageDialogOptions{
		Type:    runtime.ErrorDialog,
		Title:   "Umbra",
		Message: msg,
	})
}
