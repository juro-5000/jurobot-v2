package setup

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"jurobot/pkg/config"
)

const logo = `
     ██╗██╗   ██╗██████╗  ██████╗ ██████╗  ██████╗ ████████╗
     ██║██║   ██║██╔══██╗██╔═══██╗██╔══██╗██╔═══██╗╚══██╔══╝
     ██║██║   ██║██████╔╝██║   ██║██████╔╝██║   ██║   ██║
██   ██║██║   ██║██╔══██╗██║   ██║██╔══██╗██║   ██║   ██║
╚█████╔╝╚██████╔╝██║  ██║╚██████╔╝██████╔╝╚██████╔╝   ██║
 ╚════╝  ╚═════╝ ╚═╝  ╚═╝ ╚═════╝ ╚═════╝  ╚═════╝   ╚═╝`

var cyan = "\033[36m"
var yellow = "\033[93m"
var green = "\033[92m"
var red = "\033[31m"
var gray = "\033[90m"
var reset = "\033[0m"
var bold = "\033[1m"

func clear() {
	fmt.Print("\033[2J\033[H")
}

func printLogo() {
	for _, line := range strings.Split(logo, "\n") {
		fmt.Printf("%s%s%s\n", cyan, line, reset)
	}
	fmt.Println()
}

func askYesNo(reader *bufio.Reader, question string, defaultYes bool) bool {
	if defaultYes {
		fmt.Printf("  %s? %s[y/N]%s ", question, gray, reset)
	} else {
		fmt.Printf("  %s? %s[Y/n]%s ", question, gray, reset)
	}
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(strings.ToLower(input))
	if input == "" {
		return defaultYes
	}
	return input == "y" || input == "yes"
}

func askString(reader *bufio.Reader, question, defaultVal string) string {
	if defaultVal != "" {
		fmt.Printf("  %s %s[%s]%s ", question, gray, defaultVal, reset)
	} else {
		fmt.Printf("  %s ", question)
	}
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)
	if input == "" {
		return defaultVal
	}
	return input
}

func askInt(reader *bufio.Reader, question string, defaultVal int) int {
	def := strconv.Itoa(defaultVal)
	result := askString(reader, question, def)
	n, err := strconv.Atoi(result)
	if err != nil {
		return defaultVal
	}
	return n
}

// Run starts the first-run setup wizard.
// Returns true if config was saved.
func Run() (config.Config, bool) {
	clear()
	printLogo()

	fmt.Printf("  %sWelcome to JuroBot!%s\n", bold, reset)
	fmt.Printf("  %sNo config found. Let's set things up.%s\n\n", gray, reset)

	reader := bufio.NewReader(os.Stdin)

	if !askYesNo(reader, "Set up JuroBot now", true) {
		fmt.Printf("\n  %sSkipping setup. Using defaults.%s\n", gray, reset)
		fmt.Println()
		return config.Default(), false
	}

	fmt.Println()

	cfg := config.Default()

	// Server
	fmt.Printf("  %s── SERVER ──%s\n\n", yellow, reset)
	cfg.Server.Address = askString(reader, "Server address:", "")
	cfg.Server.Port = askInt(reader, "Server port:", 25565)

	// Account
	fmt.Printf("\n  %s── ACCOUNT ──%s\n\n", yellow, reset)
	cfg.Account.Username = askString(reader, "Bot username (Minecraft):", "")
	cfg.Account.OwnerUsername = askString(reader, "Owner username (your MC name):", "")

	fmt.Println()

	// Features
	fmt.Printf("  %s── FEATURES ──%s\n\n", yellow, reset)
	cfg.Translation.Enabled = askYesNo(reader, "Enable chat translation", true)
	if cfg.Translation.Enabled {
		cfg.Translation.TargetLang = askString(reader, "Target language:", "en")
	}
	cfg.Combat.AutoEat = askYesNo(reader, "Enable auto eat", true)
	cfg.Combat.AutoTotem = askYesNo(reader, "Enable auto totem", true)
	cfg.Combat.AutoArmor = askYesNo(reader, "Enable auto armor", true)
	cfg.Sorter.AutoEchest = askYesNo(reader, "Enable auto ender chest sorting", true)

	fmt.Println()

	// Cloud tunnel (optional)
	fmt.Printf("  %s── CLOUD TUNNEL (optional) ──%s\n\n", yellow, reset)
	if askYesNo(reader, "Enable cloud tunnel for remote access", false) {
		cfg.CloudTunnel.Enabled = true
		cfg.CloudTunnel.HealthURL = askString(reader, "Health URL:", "")
		cfg.CloudTunnel.APIToken = askString(reader, "API token:", "")
		cfg.CloudTunnel.GitHubRepo = askString(reader, "GitHub repo (owner/repo):", "")
		cfg.CloudTunnel.TunnelConfig = askString(reader, "Tunnel config path:", "")
	} else {
		cfg.CloudTunnel.Enabled = false
	}

	fmt.Println()
	fmt.Printf("  %sSetup complete!%s\n", green, reset)
	fmt.Println()

	return cfg, true
}
