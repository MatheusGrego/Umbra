// Package main — main.go (Umbra client)
// Entry point: wires all dependencies and starts the Wails window.
// Zero business logic here — only construction, configuration, wiring.
package main

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/runtime"

	gc "umbra/client/crypto"
	"umbra/client/service"
	"umbra/client/ws"
)

// deferredEmitter satisfies service.EventEmitter before the Wails context exists.
// App.startup calls setCtx once the context is available; after that all Emit
// calls are forwarded to runtime.EventsEmit.
type deferredEmitter struct {
	mu  sync.RWMutex
	ctx *context.Context
}

func (e *deferredEmitter) setCtx(ctx *context.Context) {
	e.mu.Lock()
	e.ctx = ctx
	e.mu.Unlock()
}

func (e *deferredEmitter) Emit(event string, data any) {
	e.mu.RLock()
	ctx := e.ctx
	e.mu.RUnlock()
	if ctx != nil && *ctx != nil {
		runtime.EventsEmit(*ctx, event, data)
	}
}

func main() {
	serverURL := envOr("UMBRA_SERVER", "ws://localhost:8080/ws")

	// ---- Data directory (supports multi-instance via UMBRA_HOME) ----
	home, _ := os.UserHomeDir()
	dataDir := envOr("UMBRA_HOME", filepath.Join(home, ".umbra"))
	log.Printf("[main] data dir: %s", dataDir)

	// ---- Identity ----
	store := gc.NewIdentityStoreWithDir(dataDir)
	identity, err := store.Load()
	fatalIfErr(err, "load identity")

	// ---- Peer store ----
	peers, err := gc.NewPeerStore(dataDir)
	fatalIfErr(err, "load peer store")
	fatalIfErr(peers.RehydrateSessions(identity.X25519Private), "rehydrate sessions")

	// ---- Event emitter (deferred until Wails ctx is available) ----
	emitter := &deferredEmitter{}

	// ---- Services ----
	chatSvc := service.NewChatService(identity, peers, nil, emitter)
	capsuleSvc := service.NewCapsuleService(identity, peers, nil, emitter)
	presenceSvc := service.NewPresenceService(identity, peers, nil, emitter)
	screenshareSvc := service.NewScreenShareService(identity.UserID, nil, emitter)
	voiceSvc := service.NewVoiceService(identity.UserID, nil, emitter)

	// ---- Dispatcher (Strategy pattern) ----
	dispatcher := service.NewDispatcher(nil)
	dispatcher.Register("auth_ok", service.InboundHandlerFunc(presenceSvc.HandleAuthOK))
	dispatcher.Register("msg", service.InboundHandlerFunc(chatSvc.HandleIncoming))
	dispatcher.Register("capsule_ready", service.InboundHandlerFunc(capsuleSvc.HandleReady))
	dispatcher.Register("capsule_read", service.InboundHandlerFunc(capsuleSvc.HandleData))
	dispatcher.Register("peer_online", service.InboundHandlerFunc(presenceSvc.HandlePeerOnline))
	dispatcher.Register("peer_offline", service.InboundHandlerFunc(presenceSvc.HandlePeerOffline))
	dispatcher.Register("invite_create", service.InboundHandlerFunc(presenceSvc.HandleInviteToken))
	dispatcher.Register("invite_result", service.InboundHandlerFunc(presenceSvc.HandleInviteResult))
	dispatcher.Register("webrtc_offer", service.InboundHandlerFunc(screenshareSvc.HandleOffer))
	dispatcher.Register("webrtc_answer", service.InboundHandlerFunc(screenshareSvc.HandleAnswer))
	dispatcher.Register("webrtc_ice", service.InboundHandlerFunc(screenshareSvc.HandleICE))
	dispatcher.Register("webrtc_reject", service.InboundHandlerFunc(screenshareSvc.HandleReject))
	dispatcher.Register("voice_offer", service.InboundHandlerFunc(voiceSvc.HandleOffer))
	dispatcher.Register("voice_answer", service.InboundHandlerFunc(voiceSvc.HandleAnswer))
	dispatcher.Register("voice_ice", service.InboundHandlerFunc(voiceSvc.HandleICE))
	dispatcher.Register("voice_reject", service.InboundHandlerFunc(voiceSvc.HandleReject))
	dispatcher.Register("voice_hangup", service.InboundHandlerFunc(voiceSvc.HandleHangup))

	// ---- WS connection ----
	conn := ws.NewConnection(serverURL, buildAuthFn(identity), dispatcher)

	// Inject sender now that conn exists.
	chatSvc.SetSender(conn)
	capsuleSvc.SetSender(conn)
	presenceSvc.SetSender(conn)
	screenshareSvc.SetSender(conn)
	voiceSvc.SetSender(conn)

	// Connect in background — window opens immediately regardless.
	go func() {
		if err := conn.Connect(); err != nil {
			log.Printf("[main] WS connect: %v", err)
		}
	}()

	// ---- App (Wails bridge) — emitter wired inside startup ----
	app := NewApp(chatSvc, capsuleSvc, presenceSvc, screenshareSvc, voiceSvc, emitter)

	// ---- Wails window ----
	err = wails.Run(&options.App{
		Title:     "Umbra",
		Width:     900,
		Height:    680,
		MinWidth:  800,
		MinHeight: 600,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 3, G: 3, B: 10, A: 255},
		OnStartup:        app.startup,
		Bind:             []any{app},
	})
	fatalIfErr(err, "wails.Run")
}

// buildAuthFn creates the WS AuthFunc that signs the current Unix timestamp.
func buildAuthFn(id *gc.Identity) ws.AuthFunc {
	return func() (ws.Envelope, error) {
		ts := time.Now().Unix()
		tsStr := strconv.FormatInt(ts, 10)
		sig := id.Sign([]byte(tsStr))

		payload := ws.MustMarshal(map[string]any{
			"user_id":    id.UserID,
			"public_key": id.PublicKeyB64(),
			"x25519_key": id.X25519PublicB64(),
			"timestamp":  ts,
			"signature":  sig,
		})
		return ws.Envelope{Type: "auth", Payload: payload}, nil
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// fatalIfErr panics with a formatted message on startup wiring failures.
func fatalIfErr(err error, ctx string) {
	if err != nil {
		panic("FATAL [" + ctx + "]: " + err.Error())
	}
}
