package main

import (
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"jurobot/pkg/client"
	modchat "jurobot/pkg/client/modules/chat"
	"jurobot/pkg/client/modules/combat"
	"jurobot/pkg/client/modules/entities"
	"jurobot/pkg/client/modules/physics"
	"jurobot/pkg/client/modules/playerlist"
	"jurobot/pkg/client/modules/self"
)

const (
	ModeAuto          = "auto"
	ModeRemoteControl = "remote_control"
)

var (
	botMode   string
	botModeMu sync.RWMutex
)

func init() {
	botMode = ModeAuto
}

func GetMode() string {
	botModeMu.RLock()
	defer botModeMu.RUnlock()
	return botMode
}

func SetMode(mode string) {
	botModeMu.Lock()
	defer botModeMu.Unlock()
	botMode = mode
}

type remoteControlInput struct {
	mu       sync.RWMutex
	forward  float64
	strafe   float64
	jumping  bool
	sneaking bool
	sprinting bool
	conns    int
}

var rcInput remoteControlInput

var wsUpgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

func initRemoteControl(c *client.Client) {
	phys := physics.From(c)
	s := self.From(c)
	if phys == nil || s == nil {
		return
	}

	phys.OnTick(func() {
		if GetMode() != ModeRemoteControl {
			return
		}
		rcInput.mu.RLock()
		fwd := rcInput.forward
		str := rcInput.strafe
		jmp := rcInput.jumping
		snk := rcInput.sneaking
		spr := rcInput.sprinting
		rcInput.mu.RUnlock()

		phys.SetInput(fwd, str, jmp)
		s.SetSneaking(snk)
		s.SetSprinting(spr)
	})
}

func handleWS(c *client.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		conn, err := wsUpgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()

		rcInput.mu.Lock()
		rcInput.conns++
		if rcInput.conns == 1 {
			SetMode(ModeRemoteControl)
		}
		connCount := rcInput.conns
		rcInput.mu.Unlock()

		defer func() {
			rcInput.mu.Lock()
			rcInput.conns--
			if rcInput.conns <= 0 {
				SetMode(ModeAuto)
				rcInput.forward = 0
				rcInput.strafe = 0
				rcInput.jumping = false
				rcInput.sneaking = false
				rcInput.sprinting = false
			}
			rcInput.mu.Unlock()
		}()

		conn.WriteJSON(map[string]interface{}{
			"type": "mode",
			"mode": GetMode(),
		})

		type wsCmd struct {
			Type    string  `json:"type"`
			Forward float64 `json:"forward,omitempty"`
			Strafe  float64 `json:"strafe,omitempty"`
			Jump    bool    `json:"jump,omitempty"`
			Yaw     float64 `json:"yaw,omitempty"`
			Pitch   float64 `json:"pitch,omitempty"`
			Action  string  `json:"action,omitempty"`
			Message string  `json:"message,omitempty"`
		}

		done := make(chan struct{})
		defer close(done)

		writeMu := sync.Mutex{}

		// Status sender
		go func() {
			ticker := time.NewTicker(500 * time.Millisecond)
			defer ticker.Stop()
			for {
				select {
				case <-done:
					return
				case <-ticker.C:
					s := self.From(c)
					if s == nil {
						continue
					}
					x, y, z := s.Position()
					health := s.Health()
					food := s.Food()
					ping := int32(0)
					if plist := playerlist.From(c); plist != nil {
						if p := plist.GetPlayerByName(c.Username); p != nil {
							ping = p.Ping
						}
					}

					writeMu.Lock()
					conn.SetWriteDeadline(time.Now().Add(2 * time.Second))
					err := conn.WriteJSON(map[string]interface{}{
						"type":   "status",
						"health": health,
						"food":   food,
						"x":      x,
						"y":      y,
						"z":      z,
						"ping":   ping,
						"mode":   GetMode(),
						"conns":  connCount,
					})
					writeMu.Unlock()
					if err != nil {
						return
					}
				}
			}
		}()

		// Read commands
		for {
			var cmd wsCmd
			if err := conn.ReadJSON(&cmd); err != nil {
				break
			}

			switch cmd.Type {
			case "move":
				rcInput.mu.Lock()
				rcInput.forward = cmd.Forward
				rcInput.strafe = cmd.Strafe
				rcInput.jumping = cmd.Jump
				rcInput.mu.Unlock()

			case "stop":
				rcInput.mu.Lock()
				rcInput.forward = 0
				rcInput.strafe = 0
				rcInput.jumping = false
				rcInput.mu.Unlock()

			case "action":
				switch cmd.Action {
				case "sneak":
					rcInput.mu.Lock()
					rcInput.sneaking = !rcInput.sneaking
					rcInput.mu.Unlock()
				case "sprint":
					rcInput.mu.Lock()
					rcInput.sprinting = !rcInput.sprinting
					rcInput.mu.Unlock()
				case "jump":
					rcInput.mu.Lock()
					rcInput.jumping = true
					rcInput.mu.Unlock()
					go func() {
						time.Sleep(100 * time.Millisecond)
						rcInput.mu.Lock()
						rcInput.jumping = false
						rcInput.mu.Unlock()
					}()
				case "attack":
					go func() {
						s := self.From(c)
						ent := entities.From(c)
						if s == nil || ent == nil {
							return
						}
						x, y, z := s.Position()
						nearest := ent.GetClosestEntity(x, y, z, nil)
						if nearest == nil {
							return
						}
						combatMod := combat.From(c)
						if combatMod == nil {
							c.SwingArm(0)
							return
						}
						combatMod.Attack(nearest.ID)
					}()
				case "use":
					go func() {
						s := self.From(c)
						if s != nil {
							s.Use(0)
						}
					}()
				case "drop":
					go func() {
						c.DropItem(false)
					}()
				}

		case "look":
			s := self.From(c)
			if s != nil {
				s.Rotate(cmd.Yaw, cmd.Pitch)
			}

			case "chat":
				if cmd.Message != "" {
					go modchat.From(c).SendMessage(cmd.Message)
				}
			}
		}
	}
}


