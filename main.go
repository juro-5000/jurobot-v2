package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"html"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-mclib/data/pkg/data/items"
	"github.com/go-mclib/data/pkg/data/packet_ids"
	"github.com/go-mclib/data/pkg/packets"
	jp "github.com/go-mclib/protocol/java_protocol"
	"jurobot/commands"
	"jurobot/pkg/chat"
	"jurobot/pkg/client"
	modchat "jurobot/pkg/client/modules/chat"
	"jurobot/pkg/client/modules/inventory"
	"jurobot/pkg/client/modules/playerlist"
	"jurobot/pkg/client/modules/self"
	"jurobot/pkg/client/modules/world"
	"jurobot/pkg/config"
	"jurobot/pkg/configui"
	"jurobot/pkg/console"
	"jurobot/pkg/helpers"
	"jurobot/pkg/loading"
	"jurobot/pkg/roles"
	"jurobot/pkg/setup"
	"jurobot/plugins"
)

var languageMap = make(map[string]string)

var appCfg = config.Default()
var configLoadedPath string

var (
	botLang      string
	botChatMode  string
	botLangMu    sync.RWMutex
	botChatModeMu sync.RWMutex
)

const (
	ColorReset  = "\033[0m"
	ColorYellow = "\033[93m"
	ColorBlue   = "\033[34m"
	ColorGray   = "\033[90m"
	ColorPurple = "\033[35m"
	ColorCyan   = "\033[36m"
	ColorRed    = "\033[31m"
)

func pauseOnWindows() {
	if runtime.GOOS == "windows" {
		fmt.Fprintln(os.Stderr, "Press Enter to exit.")
		fmt.Scanln()
	}
}

var (
	consoleLogs []LogEntry
	logsMu      sync.RWMutex
	startTime   = time.Now()
)

type LogEntry struct {
	Time    string `json:"time"`
	Message string `json:"message"`
}

func main() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintf(os.Stderr, "\n\x1b[31mPANIC:\x1b[0m %v\n", r)
			fmt.Fprintln(os.Stderr, "jurobot crashed. Please report this error.")
			pauseOnWindows()
			os.Exit(1)
		}
	}()

	// ── Handle "config" subcommand ──
	if len(os.Args) > 1 && os.Args[1] == "config" {
		loaded, loadedPath, err := config.Load()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Config editor error: %v\n", err)
			pauseOnWindows()
			os.Exit(1)
		}

		savePath := loadedPath
		if savePath == "" {
			configDir, err := config.ConfigDir()
			if err == nil {
				os.MkdirAll(configDir, 0755)
				savePath, _ = config.ConfigPath()
			}
			if savePath == "" {
				savePath = "config.json"
			}
		}

		saved, err := configui.Run(&loaded)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Config editor error: %v\n", err)
			pauseOnWindows()
			os.Exit(1)
		}
		if saved {
			if err := config.Save(loaded, savePath); err != nil {
				fmt.Fprintf(os.Stderr, "Error saving config: %v\n", err)
				pauseOnWindows()
				os.Exit(1)
			}
			fmt.Println("Config saved to", savePath)
		} else {
			fmt.Println("Config not saved")
		}
		return
	}

	// ── Load config for auth / first-run detection ──
	var path string
	appCfg, path, _ = config.Load()
	if path != "" {
		configLoadedPath = path
	}

	// ── First run: no config found → simple setup wizard ──
	if path == "" {
		cfg, saved := setup.Run()
		if saved {
			savePath, _ := config.ConfigPath()
			if savePath == "" {
				savePath = "config.json"
			}
			os.MkdirAll(filepath.Dir(savePath), 0755)
			appCfg = cfg
			config.Save(appCfg, savePath)
			fmt.Println("Config saved to", savePath)
			configLoadedPath = savePath
		} else {
			fmt.Println("No config saved, using defaults")
		}
	}

	// ── Auth before loading screen — Prism Launcher only ──
	prismPath := helpers.GetPrismAccountsPath()
	if prismPath != "" && appCfg.Account.Username != "" {
		token, _, _, _, _, err := helpers.GetPrismToken(prismPath, appCfg.Account.Username)
		if err == nil && token != "" && token != appCfg.Account.RefreshToken {
			appCfg.Account.RefreshToken = token
			savePath := configLoadedPath
			if savePath == "" {
				savePath, _ = config.ConfigPath()
			}
			if savePath == "" {
				savePath = "config.json"
			}
			config.Save(appCfg, savePath)
		}
	}

	if appCfg.Account.RefreshToken == "" {
		fmt.Fprintf(os.Stderr, "\n\x1b[31mNo Prism Launcher account found.\x1b[0m\n")
		fmt.Fprintf(os.Stderr, "Please add a Microsoft account in Prism Launcher first.\n")
		pauseOnWindows()
		os.Exit(1)
	}

	// ── Loading screen — starts immediately, each step fires on the real event ──
	screen := loading.New()

	// Step 1 — load translations (search multiple paths)
	langPaths := []string{"langua.json"}
	if p, err := config.LangPath(); err == nil {
		langPaths = append(langPaths, p)
	}
	if configLoadedPath != "" {
		langPaths = append(langPaths, filepath.Join(filepath.Dir(configLoadedPath), "langua.json"))
	}
	for _, lp := range langPaths {
		langData, err := os.ReadFile(lp)
		if err == nil {
			json.Unmarshal(langData, &languageMap)
			break
		}
	}
	screen.Advance(loading.StepTranslations)

	var f helpers.Flags
	helpers.RegisterFlags(&f)
	flag.Parse()

	// Override server address from config if -s flag was not given.
	// Omit the default port 25565 so go-mclib can attempt SRV record resolution
	// (_minecraft._tcp.<host>), which many proxy-based servers require.
	if f.Address == "localhost:25565" && appCfg.Server.Address != "" {
		if appCfg.Server.Port == 25565 {
			f.Address = appCfg.Server.Address
		} else {
			f.Address = fmt.Sprintf("%s:%d", appCfg.Server.Address, appCfg.Server.Port)
		}
	}

	// Step 2 — account selection from config (Prism Launcher only)
	if f.Username == "" && appCfg.Account.Username != "" {
		f.Username = appCfg.Account.Username
	}
	if f.RefreshToken == "" && appCfg.Account.RefreshToken != "" {
		f.RefreshToken = appCfg.Account.RefreshToken
	}
	if f.Username == "" && f.RefreshToken == "" {
		prismPath := helpers.GetPrismAccountsPath()
		if prismPath != "" {
			accounts, err := helpers.ListPrismAccounts(prismPath)
			if err == nil && len(accounts) > 0 {
				f.Username = accounts[0].Profile.Name
			}
		}
	}
	screen.Advance(loading.StepAccount)
	screen.Advance(loading.StepTunnel)

	if f.MaxReconnectAttempts < -1 {
		f.MaxReconnectAttempts = -1
	}

	// Step 3 — authentication (MS token refresh happens inside ConnectAndStart)
	screen.Advance(loading.StepAuthenticating)

	c := helpers.NewClient(f)
	// Silence logger during loading to prevent random text leakage
	c.Logger.SetOutput(io.Discard)

	// Step 4 — encryption request incoming (fires right after TCP connect + auth)
	c.OnConnect(func() {
		screen.Advance(loading.StepEncrypting)
	})

	// Steps 5 & 6 — compression + login success via packet handler
	compressionDone := false
	c.RegisterHandler(func(cl *client.Client, pkt *jp.WirePacket) {
		if cl.State() != jp.StateLogin {
			return
		}
		if !compressionDone && pkt.PacketID == packet_ids.S2CLoginCompressionID {
			compressionDone = true
			screen.Advance(loading.StepSecuring)
		}
		if pkt.PacketID == packet_ids.S2CLoginFinishedID {
			screen.Advance(loading.StepConnecting)
		}
	})

	// Step 7 — configuration → play
	c.OnPlay(func() {
		screen.Advance(loading.StepEnteringWorld)
	})

	// Step 8 — spawned · ready; block until bar finishes then hand off
	self.From(c).OnSpawn(func() {
		screen.Advance(loading.StepReady)
		go func() {
			screen.Wait()
			screen.Close()

			// Initialize logic after loading screen is gone
			once.Do(func() {
				// Restore logger now that the loading screen is gone
				c.Logger.SetOutput(os.Stderr)

				// Persist refresh token obtained via device code flow
				if c.RefreshToken != "" && c.RefreshToken != appCfg.Account.RefreshToken && configLoadedPath != "" {
					appCfg.Account.RefreshToken = c.RefreshToken
					if err := config.Save(appCfg, configLoadedPath); err != nil {
						c.Logger.Printf("Warning: failed to save refresh token to config: %v", err)
					}
				}

				// Initialize Roles
				roleStore := roles.NewStore("juro-5000/jurobot-actions")
				roleStore.Load()

				// Initialize Command Handler
				cmdHandler := NewCommandHandler(c, roleStore)
				cmdHandler.Register(&commands.PullCommand{})
				cmdHandler.Register(&commands.InvCommand{})
				cmdHandler.Register(&commands.ListCommand{})
				cmdHandler.Register(&commands.RoleCommand{Roles: roleStore})
				cmdHandler.Register(&commands.BanCommand{Roles: roleStore})
				cmdHandler.Register(&commands.UnbanCommand{Roles: roleStore})
				cmdHandler.Register(&commands.EndCommand{Roles: roleStore})
				helpCmd := &commands.HelpCommand{
					CommandsFunc: cmdHandler.AllCommands,
					Roles:        roleStore,
				}
				cmdHandler.Register(helpCmd)

				// Initialize Plugins
				sorter := &plugins.SorterPlugin{
					AutoEchestEnabled: appCfg.Sorter.AutoEchest,
					CheckDisabled: func() bool {
						return GetMode() == ModeRemoteControl
					},
				}
				sorter.Init(c)

				combatPlug := &plugins.CombatPlugin{
					AutoEat:       appCfg.Combat.AutoEat,
					AutoTotem:     appCfg.Combat.AutoTotem,
					AutoArmor:     appCfg.Combat.AutoArmor,
					DisableEatArmor: func() bool {
						return GetMode() == ModeRemoteControl
					},
				}
				combatPlug.Init(c)

				// Initialize Remote Control
				initRemoteControl(c)

				// Initialize Console
				con, err := console.New("", createConsoleHandler(c, sorter))
				if err != nil {
					fmt.Printf("failed to init console: %v\n", err)
				} else {
					go con.Run()
				}

				startHeadlessAPI(c, &f, cmdHandler, sorter, con)
				// Ensure it goes to the real stdout after TUI/Loading screen restore
				fmt.Println("spawned; ready")
				c.Logger.Println("spawned; ready")

				// Auto-stop cloud bot if cloud_tunnel is enabled
				if appCfg.CloudTunnel.Enabled && appCfg.CloudTunnel.APIToken != "" && appCfg.CloudTunnel.HealthURL != "" {
					go func() {
						time.Sleep(3 * time.Second)
						url := appCfg.CloudTunnel.HealthURL + "/api/stop-cloud"
						req, err := http.NewRequest("POST", url, nil)
						if err != nil {
							c.Logger.Printf("[CLOUD] Auto-stop: failed to create request: %v", err)
							return
						}
						req.Header.Set("Authorization", "Bearer "+appCfg.CloudTunnel.APIToken)
						client := &http.Client{Timeout: 10 * time.Second}
						resp, err := client.Do(req)
						if err != nil {
							c.Logger.Printf("[CLOUD] Auto-stop: failed to ping cloud: %v", err)
							return
						}
						defer resp.Body.Close()
						if resp.StatusCode == 200 {
							c.Logger.Printf("[CLOUD] Auto-stopped cloud bot")
						} else {
							c.Logger.Printf("[CLOUD] Auto-stop: cloud returned status %d", resp.StatusCode)
						}
					}()
				}

				if f.Timeout > 0 {
					time.AfterFunc(time.Duration(f.Timeout)*time.Second, func() {
						c.Logger.Printf("Timeout of %d seconds reached, exiting...", f.Timeout)
						c.Disconnect(true)
					})
				}
			})
		}()
	})

	if err := c.ConnectAndStart(context.Background()); err != nil {
		screen.Close()
		screen.Wait()
		fmt.Fprintf(os.Stderr, "\n\x1b[31mConnection error:\x1b[0m %v\n", err)
		pauseOnWindows()
		os.Exit(1)
	}
}

var once sync.Once

type logRedirector struct {
	original io.Writer
	con      *console.Console
}

func (l *logRedirector) Write(p []byte) (n int, err error) {
	msg := strings.TrimSpace(string(p))
	content := msg
	timestamp := ""
	if len(msg) > 20 && msg[10] == ' ' && msg[19] == ' ' {
		timestamp = msg[:20]
		content = msg[20:]
	}

	colored := msg
	if strings.HasPrefix(content, "[") {
		endIdx := strings.Index(content, "]")
		if endIdx > 1 {
			prefix := content[1:endIdx]
			rest := strings.TrimSpace(content[endIdx+1:])
			switch {
			case prefix == "CHAT-JSON":
				var comp interface{}
				if err := json.Unmarshal([]byte(rest), &comp); err == nil {
					colored = timestamp + "[" + ColorYellow + "CHAT" + ColorReset + "] " + Colorize(nil, comp)
				}
			case prefix == "SYSTEM-JSON":
				var comp interface{}
				if err := json.Unmarshal([]byte(rest), &comp); err == nil {
					colored = timestamp + "[" + ColorGray + "SYSTEM" + ColorReset + "] " + Colorize(nil, comp)
				}
			case prefix == "CHAT":
				// Standard chat
			case prefix == "WHISPER" || prefix == "CHAT-WHISPER" || prefix == "DISGUISED-WHISPER":
				colored = timestamp + "[" + ColorPurple + "WHISPER" + ColorReset + "]" + rest
			case strings.Contains(prefix, "Discord |"):
				role := "Discord"
				if idx := strings.Index(prefix, "| "); idx != -1 {
					role = prefix[idx+2:]
				}
				sender := ""
				messageBody := rest
				if idx := strings.Index(rest, " » "); idx != -1 {
					sender = rest[:idx]
					messageBody = rest[idx+3:]
				}
				if sender != "" {
					colored = timestamp + "[" + ColorBlue + "Discord" + ColorReset + "] [" + role + "] " + sender + ": " + messageBody
				} else {
					colored = timestamp + "[" + ColorBlue + "Discord" + ColorReset + "] " + rest
				}
			}
		}
	} else if strings.Contains(content, "Discord |") {
		displayMsg := content
		if idx := strings.Index(content, " » "); idx != -1 {
			prefixPart := content[:idx]
			msgPart := content[idx+3:]
			role := "Discord"
			if pipeIdx := strings.Index(prefixPart, "| "); pipeIdx != -1 {
				if endBracketIdx := strings.Index(prefixPart[pipeIdx:], "]"); endBracketIdx != -1 {
					role = prefixPart[pipeIdx+2 : pipeIdx+endBracketIdx]
					senderPart := strings.TrimSpace(prefixPart[pipeIdx+endBracketIdx+1:])
					displayMsg = "[" + ColorBlue + "Discord" + ColorReset + "] [" + role + "] " + senderPart + ": " + msgPart
				}
			}
		}
		colored = timestamp + displayMsg
	}

	logToConsole(colored)
	finalColored := chat.ReplaceWaypoints(colored)

	if l.con != nil {
		l.con.Println(finalColored)
	} else {
		fmt.Fprintln(l.original, finalColored)
	}
	return len(p), nil
}

func createConsoleHandler(c *client.Client, sorter *plugins.SorterPlugin) console.Handler {
	return func(cmd, args string) string {
		switch cmd {
		case "setlang":
			lang := strings.TrimSpace(args)
			if lang == "" {
				return ""
			}
			botLangMu.Lock()
			botChatModeMu.Lock()
			if lang == "off" || lang == "default" {
				botLang = ""
				botChatMode = "normal"
				botChatModeMu.Unlock()
				botLangMu.Unlock()
				return "Auto-translate disabled"
			}
			botLang = lang
			botChatMode = "normal"
			botChatModeMu.Unlock()
			botLangMu.Unlock()
			return fmt.Sprintf("Auto-translate set to %s", lang)
		case "chat":
			method := strings.TrimSpace(args)
			if method == "" {
				return ""
			}
			botChatModeMu.Lock()
			botLangMu.Lock()
			if method == "normal" || method == "default" || method == "off" {
				botChatMode = "normal"
				botLang = ""
				botLangMu.Unlock()
				botChatModeMu.Unlock()
				return "Chat mode set to normal"
			}
			if method == "anti_translate" {
				botChatMode = "anti_translate"
				botLang = ""
				botLangMu.Unlock()
				botChatModeMu.Unlock()
				return "Chat mode set to anti_translate"
			}
			botLangMu.Unlock()
			botChatModeMu.Unlock()
			return fmt.Sprintf("Unknown chat mode: %s (use normal or anti_translate)", method)
		case "say":
			msg := args
			botLangMu.RLock()
			lang := botLang
			botLangMu.RUnlock()
			if lang != "" {
				if translated := translate(msg, lang); translated != "" && !strings.EqualFold(translated, msg) {
					msg = translated
				}
			} else {
				botChatModeMu.RLock()
				mode := botChatMode
				botChatModeMu.RUnlock()
				if mode == "anti_translate" {
					msg = antiTranslate(msg)
				}
			}
			modchat.From(c).SendMessage(msg)
			return ""
		case "inv":
			inv := inventory.From(c)
			if inv == nil {
				return "Inventory module not ready"
			}
			var sb strings.Builder
			sb.WriteString(ColorYellow + "[Inventory]" + ColorReset + "\n")

			formatItem := func(item *items.ItemStack) string {
				if item == nil || item.IsEmpty() {
					return "(empty)"
				}
				res := fmt.Sprintf("%s x%d", items.ItemName(item.ID), item.Count)
				if item.Components != nil {
					// Durability
					if item.Components.MaxDamage > 0 {
						dur := item.Components.MaxDamage - item.Components.Damage
						res += fmt.Sprintf(" (durability: %d/%d)", dur, item.Components.MaxDamage)
					}
					// Enchantments
					if len(item.Components.Enchantments) > 0 {
						res += " ["
						first := true
						for ench, lv := range item.Components.Enchantments {
							if !first {
								res += ", "
							}
							res += fmt.Sprintf("%s %d", ench, lv)
							first = false
						}
						res += "]"
					}
				}
				return res
			}

			// Hotbar (Slots 36-44)
			sb.WriteString(ColorCyan + "Hotbar (0-8):" + ColorReset + "\n")
			for i := 0; i < 9; i++ {
				item := inv.GetSlot(inventory.SlotHotbarStart + i)
				sb.WriteString(fmt.Sprintf("  Slot %d: %s\n", i, formatItem(item)))
			}

			// Main Inventory (Slots 9-35)
			sb.WriteString(ColorCyan + "Main Inventory (9-35):" + ColorReset + "\n")
			for i := inventory.SlotMainStart; i < inventory.SlotMainEnd; i++ {
				item := inv.GetSlot(i)
				if item != nil && !item.IsEmpty() {
					sb.WriteString(fmt.Sprintf("  Slot %d: %s\n", i, formatItem(item)))
				}
			}

			// Armor (Slots 5-8)
			sb.WriteString(ColorCyan + "Armor:" + ColorReset + "\n")
			armorLabels := []string{"Head", "Chest", "Legs", "Feet"}
			for i := 0; i < 4; i++ {
				item := inv.GetSlot(inventory.SlotArmorHead + i)
				sb.WriteString(fmt.Sprintf("  %s: %s\n", armorLabels[i], formatItem(item)))
			}
			return sb.String()
		case "forcekeeplist":
			go sorter.RunSortingRoutine(c)
			return ""
		case "pos":
			s := self.From(c)
			x, y, z := s.Position()
			return fmt.Sprintf("Position: %.2f, %.2f, %.2f", x, y, z)
		case "health":
			s := self.From(c)
			return fmt.Sprintf("Health: %.1f, Food: %d", s.Health(), s.Food())
		case "list":
			plist := playerlist.From(c)
			if plist == nil {
				return "Player list module not ready"
			}
			players := plist.GetAllPlayers()
			var sb strings.Builder
			sb.WriteString(fmt.Sprintf(ColorYellow+"Online Players (%d):"+ColorReset+"\n", len(players)))
			for _, p := range players {
				sb.WriteString(fmt.Sprintf("  - %s (Ping: %dms)\n", p.Name, p.Ping))
			}
			return sb.String()
		case "echest":
			cmd := &commands.EchestCommand{}
			cmd.Execute(&commands.Context{
				Client:  c,
				Sender:  "[console]",
				Message: ".echest",
			})
			return ""
		case "dropspawners":
			go func() {
				s := self.From(c)
				inv := inventory.From(c)
				w := world.From(c)
				if s == nil || inv == nil || w == nil {
					return
				}
				sorter.DropSpawnersFromEchest(c, s, inv, w)
			}()
			return ""
		case "forcepearl":
			cmd := &commands.ForcePearlCommand{}
			cmd.Execute(&commands.Context{
				Client:  c,
				Sender:  "[console]",
				Message: ".forcepearl " + args,
			})
			return ""
		case "mode":
			arg := strings.TrimSpace(args)
			if arg == "" {
				return fmt.Sprintf("Current mode: %s", GetMode())
			}
			switch arg {
			case "auto":
				SetMode(ModeAuto)
				return "Mode set to auto"
			case "remote", "remote_control":
				SetMode(ModeRemoteControl)
				return "Mode set to remote control"
			default:
				return fmt.Sprintf("Unknown mode: %s (use auto or remote)", arg)
			}
		case "help":
			return "Available commands: .say <msg>, .inv, .echest, .forcekeeplist, .dropspawners, .pos, .health, .list, .forcepearl <user>, .setlang <lang>, .chat normal|anti_translate, .mode [auto|remote], .help"
		default:
			return "Unknown command: ." + cmd
		}
	}
}

func logToConsole(message string) {
	logsMu.Lock()
	defer logsMu.Unlock()
	entry := LogEntry{
		Time:    time.Now().Format(time.RFC3339),
		Message: message,
	}
	consoleLogs = append(consoleLogs, entry)
	if len(consoleLogs) > 45 {
		consoleLogs = consoleLogs[len(consoleLogs)-45:]
	}
}

func startHeadlessAPI(c *client.Client, f *helpers.Flags, h *CommandHandler, sorter *plugins.SorterPlugin, con *console.Console) {
	c.Logger.SetOutput(&logRedirector{original: os.Stdout, con: con})

	if appCfg.Debug.Enabled {
		c.RegisterHandler(func(c *client.Client, pkt *jp.WirePacket) {
			if appCfg.Debug.ShowChatPackets {
				switch pkt.PacketID {
				case packet_ids.S2CPlayerChatID:
					var d packets.S2CPlayerChat
					if err := pkt.ReadInto(&d); err == nil {
						if appCfg.Debug.ShowJSON {
							raw, _ := json.Marshal(d)
							c.Logger.Printf("[DEBUG-CHAT] PlayerChat JSON: %s", string(raw))
						} else {
							c.Logger.Printf("[DEBUG-CHAT] PlayerChat: Sender=%s, Content=%s", d.ChatType.Name.Text, string(d.Body.Content))
						}
					}
				case packet_ids.S2CSystemChatID:
					var d packets.S2CSystemChat
					if err := pkt.ReadInto(&d); err == nil {
						if appCfg.Debug.ShowJSON {
							raw, _ := json.Marshal(d)
							c.Logger.Printf("[DEBUG-CHAT] SystemChat JSON: %s", string(raw))
						} else {
							c.Logger.Printf("[DEBUG-CHAT] SystemChat: Content=%s", d.Content.String())
						}
					}
				case packet_ids.S2CDisguisedChatID:
					var d packets.S2CDisguisedChat
					if err := pkt.ReadInto(&d); err == nil {
						if appCfg.Debug.ShowJSON {
							raw, _ := json.Marshal(d)
							c.Logger.Printf("[DEBUG-CHAT] DisguisedChat JSON: %s", string(raw))
						} else {
							c.Logger.Printf("[DEBUG-CHAT] DisguisedChat: Sender=%s, Content=%s", d.SenderName.String(), d.Message.String())
						}
					}
				}
			}
			if appCfg.Debug.ShowRawHex {
				if pkt.PacketID == packet_ids.S2CPlayerChatID || pkt.PacketID == packet_ids.S2CSystemChatID || pkt.PacketID == packet_ids.S2CDisguisedChatID {
					c.Logger.Printf("[DEBUG-RAW] PacketID: 0x%X, Data: %x", pkt.PacketID, pkt.Data)
				}
			}
		})
	}

	modchat.From(c).OnSystemChat(func(message string, isOverlay bool) {
		source := "mc"
		sender := "System"
		cmdMsg := mojangTranslate(c, message)
		if strings.Contains(message, "Discord |") {
			source = "discord"
			sender, cmdMsg = parseDiscordMsg(message)
			cmdMsg = strings.TrimSpace(cmdMsg)
		}
		h.Handle(sender, cmdMsg, source, false)
	})
	modchat.From(c).OnPlayerChat(func(sender, message string, isWhisper bool) {
		source := "mc"
		if strings.Contains(sender, "Discord") || strings.Contains(message, "Discord |") {
			source = "discord"
		}
		h.Handle(sender, message, source, isWhisper)

		if appCfg.Translation.Enabled {
			go func() {
				translated := translate(message, appCfg.Translation.TargetLang)
				if translated != "" && !strings.EqualFold(translated, message) {
					c.Logger.Printf("[TRANSLATION] %s: %s", sender, translated)
				} else if appCfg.Translation.Verbose {
					c.Logger.Printf("[DEBUG-CHAT] Translation skipped for: \"%s\" (result: \"%s\")", message, translated)
				}
			}()
		}
	})
	modchat.From(c).OnDisguisedChat(func(sender, message string, isWhisper bool) {
		source := "mc"
		if strings.Contains(sender, "Discord") || strings.Contains(message, "Discord |") {
			source = "discord"
		}
		h.Handle(sender, message, source, isWhisper)
	})

	var autoLogoff sync.Once
	keepCfg := &appCfg.Keepalive
	self.From(c).OnHealthSet(func(health, food float32) {
		if keepCfg.Enabled && health < float32(keepCfg.HealthThreshold) {
			logToConsole(fmt.Sprintf("%sCRITICAL HEALTH: %.1f - logging off%s", ColorRed, health, ColorReset))
			autoLogoff.Do(func() {
				c.Disconnect(true)
			})
		}
	})
	self.From(c).OnPosition(func(x, y, z float64) {
		if keepCfg.Enabled && y <= keepCfg.VoidYThreshold {
			logToConsole(fmt.Sprintf("%sBOT IN VOID (Y=%.1f) - logging off%s", ColorRed, y, ColorReset))
			autoLogoff.Do(func() {
				c.Disconnect(true)
			})
		}
	})



	http.HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) {
		if appCfg.CloudTunnel.APIToken != "" {
			authHeader := r.Header.Get("Authorization")
			if authHeader != "Bearer "+appCfg.CloudTunnel.APIToken {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
		}
		w.Header().Set("Content-Type", "application/json")
		if c.State() == jp.StatePlay {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"active":  true,
				"uptime":  time.Since(startTime).Round(time.Second).String(),
				"username": c.Username,
			})
		} else {
			w.WriteHeader(http.StatusServiceUnavailable)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"active": false,
			})
		}
	})

	http.HandleFunc("/api/stop-cloud", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !appCfg.CloudTunnel.Enabled {
			http.Error(w, "Cloud tunnel not enabled", http.StatusForbidden)
			return
		}
		authHeader := r.Header.Get("Authorization")
		if authHeader != "Bearer "+appCfg.CloudTunnel.APIToken {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		repo := appCfg.CloudTunnel.GitHubRepo
		if repo == "" {
			repo = "juro-5000/jurobot-v2"
		}

		go func() {
			// Find the latest running workflow run and cancel it
			cmd := fmt.Sprintf("gh run list --repo %s --workflow=run-bot.yml --status=in_progress --limit=1 --json databaseId --jq '.[0].databaseId'", repo)
			output, err := exec.Command("bash", "-c", cmd).Output()
			if err != nil {
				c.Logger.Printf("[CLOUD] Failed to list runs: %v", err)
				return
			}
			runID := strings.TrimSpace(string(output))
			if runID == "" || runID == "null" {
				c.Logger.Printf("[CLOUD] No active cloud run found to stop")
				return
			}
			cancelCmd := fmt.Sprintf("gh run cancel %s --repo %s", runID, repo)
			if output, err := exec.Command("bash", "-c", cancelCmd).CombinedOutput(); err != nil {
				c.Logger.Printf("[CLOUD] Failed to cancel run %s: %v — %s", runID, err, string(output))
			} else {
				c.Logger.Printf("[CLOUD] Stopped cloud run %s", runID)
			}
		}()

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
		})
	})

	go func() {
		port := 5050
		for ; port <= 5060; port++ {
			addr := fmt.Sprintf(":%d", port)
			l, err := net.Listen("tcp", addr)
			if err == nil {
				if port != 5050 {
					c.Logger.Printf("port 5050 in use, using %s instead", addr)
				}
				if err := http.Serve(l, nil); err != nil {
					c.Logger.Printf("API Server error on %s: %v", addr, err)
				}
				return
			}
		}
		c.Logger.Printf("failed to start API server: no free port found in range 3000-3010")
	}()
}

func Colorize(c *client.Client, comp interface{}) string {
	if comp == nil {
		return ""
	}
	colorMap := map[string]string{
		"black":        "\033[30m",
		"dark_blue":    "\033[34m",
		"dark_green":   "\033[32m",
		"dark_aqua":    "\033[36m",
		"dark_red":     "\033[31m",
		"dark_purple":  "\033[35m",
		"gold":         "\033[33m",
		"gray":         "\033[37m",
		"dark_gray":    "\033[90m",
		"blue":         "\033[94m",
		"green":        "\033[92m",
		"aqua":         "\033[96m",
		"red":          "\033[91m",
		"light_purple": "\033[95m",
		"yellow":       "\033[93m",
		"white":        "\033[97m",
	}
	data, err := json.Marshal(comp)
	if err != nil {
		return ""
	}
	var m struct {
		Text      string        `json:"text"`
		Translate string        `json:"translate"`
		With      []interface{} `json:"with"`
		Color     string        `json:"color"`
		Bold      bool          `json:"bold"`
		Extra     []interface{} `json:"extra"`
	}
	if err := json.Unmarshal(data, &m); err != nil {
		return ""
	}
	result := ""
	if m.Color != "" {
		if strings.HasPrefix(m.Color, "#") {
			result += "\033[38;2;" + hexToRGB(m.Color) + "m"
		} else if col, ok := colorMap[m.Color]; ok {
			result += col
		}
	}
	if m.Bold {
		result += "\033[1m"
	}
	if m.Translate != "" {
		format := m.Translate
		if val, ok := languageMap[m.Translate]; ok {
			format = val
		}
		translated := format
		for i, arg := range m.With {
			argText := Colorize(c, arg)
			pName := fmt.Sprintf("%%%d$s", i+1)
			if strings.Contains(translated, pName) {
				translated = strings.ReplaceAll(translated, pName, argText)
			} else if strings.Contains(translated, "%s") {
				translated = strings.Replace(translated, "%s", argText, 1)
			}
		}
		result += translated
	} else {
		result += m.Text
	}
	for _, e := range m.Extra {
		result += Colorize(c, e)
	}

	final := result + ColorReset
	return chat.ReplaceWaypoints(final)
}

var trueColorRegex = regexp.MustCompile(`\x1b\[38;2;(\d+);(\d+);(\d+)m`)

func ansiToHTML(s string) string {
	// Escape special HTML characters but preserve ESC character for now
	result := html.EscapeString(s)

	// 1. Handle TrueColor: \033[38;2;R;G;Bm -> <span style="color:rgb(R,G,B)">
	result = trueColorRegex.ReplaceAllString(result, `<span style="color:rgb($1,$2,$3)">`)

	// 2. Handle basic Minecraft colors (using \x1b since it was escaped or literal)
	replacements := []struct{ ansi, html string }{
		{"\x1b[93m", "<span style=\"color:#FFFF55\">"}, // Yellow
		{"\x1b[34m", "<span style=\"color:#0000AA\">"}, // Dark Blue
		{"\x1b[94m", "<span style=\"color:#5555FF\">"}, // Blue
		{"\x1b[90m", "<span style=\"color:#555555\">"}, // Dark Gray
		{"\x1b[35m", "<span style=\"color:#AA00AA\">"}, // Dark Purple
		{"\x1b[95m", "<span style=\"color:#FF55FF\">"}, // Light Purple
		{"\x1b[36m", "<span style=\"color:#00AAAA\">"}, // Dark Aqua
		{"\x1b[96m", "<span style=\"color:#55FFFF\">"}, // Aqua
		{"\x1b[31m", "<span style=\"color:#AA0000\">"}, // Dark Red
		{"\x1b[91m", "<span style=\"color:#FF5555\">"}, // Red
		{"\x1b[37m", "<span style=\"color:#AAAAAA\">"}, // Gray
		{"\x1b[30m", "<span style=\"color:#000000\">"}, // Black
		{"\x1b[32m", "<span style=\"color:#00AA00\">"}, // Dark Green
		{"\x1b[92m", "<span style=\"color:#55FF55\">"}, // Green
		{"\x1b[33m", "<span style=\"color:#FFAA00\">"}, // Gold
		{"\x1b[97m", "<span style=\"color:#FFFFFF\">"}, // White
		{"\x1b[1m", "<b>"},
		{"\x1b[0m", "</span></b>"},
	}

	for _, r := range replacements {
		result = strings.ReplaceAll(result, r.ansi, r.html)
	}

	// 3. Ensure all tags are balanced
	openSpans := strings.Count(result, "<span") - strings.Count(result, "</span>")
	openBolds := strings.Count(result, "<b>") - strings.Count(result, "</b>")
	for i := 0; i < openSpans; i++ {
		result += "</span>"
	}
	for i := 0; i < openBolds; i++ {
		result += "</b>"
	}

	return result
}

func hexToRGB(hex string) string {
	hex = strings.TrimPrefix(hex, "#")
	if len(hex) != 6 {
		return "255;255;255"
	}
	r, _ := strconv.ParseInt(hex[0:2], 16, 64)
	g, _ := strconv.ParseInt(hex[2:4], 16, 64)
	b, _ := strconv.ParseInt(hex[4:6], 16, 64)
	return fmt.Sprintf("%d;%d;%d", r, g, b)
}

func translate(text string, target string) string {
	if len(text) == 0 || strings.Contains(text, "http") {
		return ""
	}
	if strings.EqualFold(strings.TrimSpace(text), "juro") {
		return ""
	}
	if target == "" {
		target = "en"
	}
	encodedText := url.QueryEscape(text)
	apiURL := fmt.Sprintf("https://translate.googleapis.com/translate_a/single?client=gtx&sl=auto&tl=%s&dt=t&q=%s", target, encodedText)
	resp, err := http.Get(apiURL)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return ""
	}
	var result []interface{}
	if err := json.Unmarshal(body, &result); err != nil || len(result) == 0 {
		return ""
	}
	inner, ok := result[0].([]interface{})
	if !ok || len(inner) == 0 {
		return ""
	}
	translated := ""
	for _, part := range inner {
		p, ok := part.([]interface{})
		if ok && len(p) > 0 {
			if t, ok := p[0].(string); ok {
				translated += t
			}
		}
	}
	return strings.TrimSpace(translated)
}

// parseDiscordMsg extracts the sender and message from a Discord system chat.
// Format: "Discord | <role>] <username> » <message>" or "Discord | <role>] <message>"
func parseDiscordMsg(msg string) (sender, body string) {
	idx := strings.Index(msg, "] ")
	if idx == -1 {
		return "Discord", msg
	}
	rest := msg[idx+2:]
	if sepIdx := strings.Index(rest, " » "); sepIdx != -1 {
		return rest[:sepIdx], rest[sepIdx+3:]
	}
	return "Discord", rest
}

func mojangTranslate(c *client.Client, message string) string {
	msg := strings.TrimSpace(message)
	if msg == "" {
		return ""
	}
	if val, ok := languageMap[msg]; ok {
		return val
	}
	bestKey := ""
	for key := range languageMap {
		if key != "" && strings.HasPrefix(msg, key) {
			if len(key) > len(bestKey) {
				bestKey = key
			}
		}
	}
	if bestKey != "" {
		format := languageMap[bestKey]
		remaining := strings.TrimPrefix(msg, bestKey)
		if !strings.Contains(format, "%") {
			if remaining != "" {
				return format + " " + mojangTranslate(c, remaining)
			}
			return format
		}
		placeholderCount := 0
		for i := 1; i <= 9; i++ {
			if strings.Contains(format, fmt.Sprintf("%%%d$s", i)) {
				placeholderCount++
			}
		}
		if placeholderCount == 0 && strings.Contains(format, "%s") {
			placeholderCount = strings.Count(format, "%s")
		}
		args := splitMojangArgs(c, remaining, placeholderCount)
		result := format
		for i, arg := range args {
			translatedArg := mojangTranslate(c, arg)
			pName := fmt.Sprintf("%%%d$s", i+1)
			if strings.Contains(result, pName) {
				result = strings.ReplaceAll(result, pName, translatedArg)
			} else if strings.Contains(result, "%s") {
				result = strings.Replace(result, "%s", translatedArg, 1)
			}
		}
		return result
	}
	return msg
}

func splitMojangArgs(c *client.Client, s string, expected int) []string {
	if expected <= 1 || s == "" {
		if s == "" {
			return nil
		}
		return []string{s}
	}
	bestPos := -1
	for key := range languageMap {
		if key == "" || len(key) < 4 {
			continue
		}
		pos := strings.Index(s, key)
		if pos > 0 && (bestPos == -1 || pos < bestPos) {
			bestPos = pos
		}
	}
	plist := playerlist.From(c)
	if plist != nil {
		for _, p := range plist.GetAllPlayers() {
			if len(p.Name) < 3 {
				continue
			}
			pos := strings.Index(s, p.Name)
			if pos > 0 && (bestPos == -1 || pos < bestPos) {
				bestPos = pos
			}
		}
	}
	if bestPos > 0 {
		return append([]string{s[:bestPos]}, splitMojangArgs(c, s[bestPos:], expected-1)...)
	}
	return []string{s}
}

func antiTranslate(text string) string {
	var sb strings.Builder
	for _, r := range text {
		switch r {
		case 'i', 'I':
			sb.WriteRune('1')
		case 'e', 'E':
			sb.WriteRune('3')
		case 'a', 'A':
			sb.WriteRune('@')
		case 'o', 'O':
			sb.WriteRune('0')
		default:
			sb.WriteRune(r)
		}
	}
	return sb.String()
}
