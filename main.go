package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"golang.org/x/term"

	"nwafu-srun/pkg/config"
	"nwafu-srun/pkg/srun"
)

// stdin is shared so prompts and password reads stay in sync.
var stdin = bufio.NewReader(os.Stdin)

const (
	exitOK      = 0
	exitRuntime = 1
	exitUsage   = 2
)

var (
	username   string
	password   string
	force      bool
	bypass     bool
	all        bool
	acid       string
	verbose    bool
	help       bool
	configPath string
	noConfig   bool
	saveConfig bool
	menu       bool
	logoutMode string
)

var (
	cfgRuntime config.Runtime
	cliFlags   config.CLIFlags
)

var (
	cachedConfigStr string
	cachedStatusStr string
	cacheTime       time.Time
	cacheTTL        = 10 * time.Second
)

func init() {
	flag.StringVar(&username, "u", "", "Username")
	flag.StringVar(&username, "username", "", "Username")
	flag.StringVar(&password, "p", "", "Password")
	flag.StringVar(&password, "password", "", "Password")
	flag.BoolVar(&force, "f", false, "Logout before login (clears stale sessions)")
	flag.BoolVar(&force, "force", false, "Logout before login")
	flag.BoolVar(&bypass, "b", false, "Bypass billing after login")
	flag.BoolVar(&bypass, "bypass", false, "Bypass billing")
	flag.BoolVar(&all, "a", false, "Kick ALL sessions on the account during bypass")
	flag.BoolVar(&all, "all", false, "Kick ALL sessions on the account during bypass")
	flag.StringVar(&acid, "acid", "1", "Access controller ID (ac_id)")
	flag.BoolVar(&verbose, "v", false, "Verbose output (stderr)")
	flag.BoolVar(&verbose, "verbose", false, "Verbose output")
	flag.BoolVar(&help, "h", false, "Help")
	flag.BoolVar(&help, "help", false, "Help")
	flag.StringVar(&configPath, "config", "", "Path to config file")
	flag.BoolVar(&noConfig, "no-config", false, "Ignore all config files")
	flag.BoolVar(&saveConfig, "save-config", false, "Save current flags to user config dir and exit")
	flag.BoolVar(&menu, "m", false, "Force interactive menu mode")
	flag.BoolVar(&menu, "menu", false, "Force interactive menu mode")
	flag.StringVar(&logoutMode, "logout-mode", "", "Logout mode: portal or selfservice")
}

func guide(argv string) {
	fmt.Printf("Usage:\n")
	fmt.Printf("  Interactive (no credentials):\n")
	fmt.Printf("    %s\n", argv)
	fmt.Printf("  Non-interactive (credentials via flags, env, or config file):\n")
	fmt.Printf("    %s -u <user> -p <pass> [-f] [-b [-a]] [--acid N] [-v]\n", argv)
	fmt.Printf("\nConfig:\n")
	fmt.Printf("    %s --save-config   (uses -u/-p, env, or loaded config)\n", argv)
	fmt.Printf("    %s --config <path>   %s --no-config\n", argv, argv)
	fmt.Printf("\nOptions:\n")
	fmt.Printf("  -u, -p           Credentials (or env %s / %s)\n", srun.EnvUsername, srun.EnvPassword)
	fmt.Printf("  -f               Logout before login\n")
	fmt.Printf("  -b               Bypass billing after login (kicks own MAC only by default)\n")
	fmt.Printf("  -a               With -b: kick ALL sessions on the account\n")
	fmt.Printf("  --acid, -v, -h   See README\n")
	fmt.Printf("\nWith a saved config (auto_auth=true), running with no args performs auto-login.\n")
}

const (
	colorReset    = "\033[0m"
	colorBoldRed  = "\033[1;31m"
	colorBoldGreen = "\033[1;32m"
	colorBoldCyan = "\033[1;36m"
)

var ansiRe = regexp.MustCompile(`\033\[[0-9;]*m`)

func displayWidth(s string) int {
	clean := ansiRe.ReplaceAllString(s, "")
	w := 0
	for _, r := range clean {
		if r > 127 {
			w += 2
		} else {
			w += 1
		}
	}
	return w
}

func printBoxLine(leftText string, rightText string, width int) {
	wLeft := displayWidth(leftText)
	wRight := displayWidth(rightText)
	padding := width - wLeft - wRight - 2
	if padding < 0 {
		padding = 0
	}
	fmt.Printf("%s│%s %s%s%s %s│%s\n",
		colorBoldCyan,
		colorReset,
		leftText,
		strings.Repeat(" ", padding),
		rightText,
		colorBoldCyan,
		colorReset,
	)
}

func badge(v bool) string {
	if v {
		return "[" + colorBoldGreen + " ON " + colorReset + "]"
	}
	return "[" + colorBoldRed + " OFF " + colorReset + "]"
}

var interruptChan chan struct{}

func startInterruptMonitor() {
	interruptChan = make(chan struct{}, 1)
	go func() {
		_, _ = stdin.ReadString('\n')
		interruptChan <- struct{}{}
	}()
}

func checkInterrupt() bool {
	fmt.Print("Press Enter within 2 seconds to enter interactive menu...")
	select {
	case <-interruptChan:
		fmt.Println("Interrupting auto-login...")
		return true
	case <-time.After(2 * time.Second):
		fmt.Println()
		return false
	}
}

func getStatusHeader(client *srun.Client, rt *config.Runtime) (string, string) {
	configStr := fmt.Sprintf("User: %s | Auto: %s | Bypass: %s",
		rt.Username,
		onOff(rt.AutoAuth),
		onOff(rt.Bypass),
	)

	if time.Since(cacheTime) < cacheTTL {
		return configStr, cachedStatusStr
	}

	info, err := client.GetLoginInfo()
	var statusStr string
	if err == nil {
		statusStr = fmt.Sprintf("Status: Online | IP: %s | Balance: %s", info.IP, info.Balance)
	} else if errors.Is(err, srun.ErrNotOnline) {
		statusStr = "Status: Offline"
	} else {
		statusStr = "Status: Check failed (Off-campus?)"
	}

	cachedStatusStr = statusStr
	cacheTime = time.Now()
	return configStr, statusStr
}

func invalidateStatusCache() {
	cacheTime = time.Time{}
}

func fail(code int, msg string, args ...any) {
	if len(args) > 0 {
		fmt.Fprintf(os.Stderr, "Error: "+msg+"\n", args...)
	} else {
		fmt.Fprintln(os.Stderr, "Error:", msg)
	}
	os.Exit(code)
}

func printErr(err error) {
	fmt.Fprintln(os.Stderr, "Error:", err)
	if h := srun.Hint(err); h != "" {
		fmt.Fprintf(os.Stderr, "Hint: %s\n", h)
	}
}

// exitOnEOF terminates cleanly on Ctrl+D / pipe EOF (exit 0).
func exitOnEOF(err error) {
	if err == nil {
		return
	}
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
		fmt.Println()
		os.Exit(exitOK)
	}
}

func readLine(prompt string) (string, error) {
	fmt.Print(prompt)
	line, err := stdin.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(line), nil
}

func readPassword(prompt string) (string, error) {
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return "", errors.New("password must be provided via -p or " + srun.EnvPassword + " when stdin is not a TTY")
	}
	fmt.Print(prompt)
	b, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Println()
	if err != nil {
		return "", err
	}
	return string(b), nil
}

const menuInnerWidth = 60


func confirm(prompt string, defaultYes bool) bool {
	def := "n"
	if defaultYes {
		def = "y"
	}
	ans, err := readLine(fmt.Sprintf("%s [%s]: ", prompt, def))
	if err != nil {
		exitOnEOF(err)
		return defaultYes
	}
	if ans == "" {
		return defaultYes
	}
	ans = strings.ToLower(ans)
	return ans == "y" || ans == "yes"
}

func syncClientCredentials(client *srun.Client, rt *config.Runtime) {
	client.Username = rt.Username
	client.Password = rt.Password
	client.AcID = rt.AcID
	client.LogoutMode = rt.LogoutMode
}

func promptUsername(rt *config.Runtime) error {
	if rt.Username == "" {
		if !term.IsTerminal(int(os.Stdin.Fd())) {
			return fmt.Errorf("username required via -u or %s when stdin is not a TTY", srun.EnvUsername)
		}
		u, err := readLine("Username: ")
		exitOnEOF(err)
		if err != nil {
			return err
		}
		if u == "" {
			return errors.New("username required")
		}
		rt.Username = u
	}
	return nil
}

func promptCredentials(rt *config.Runtime) error {
	if err := promptUsername(rt); err != nil {
		return err
	}
	if rt.Password == "" {
		p, err := readPassword("Password: ")
		exitOnEOF(err)
		if err != nil {
			return err
		}
		if p == "" {
			return errors.New("password required")
		}
		rt.Password = p
	}
	return nil
}

func changeCredentials(rt *config.Runtime, client *srun.Client) {
	label := rt.Username
	if label == "" {
		label = "(empty)"
	}
	u, err := readLine(fmt.Sprintf("Username [%s]: ", label))
	exitOnEOF(err)
	if err != nil {
		return
	}
	if u != "" {
		rt.Username = strings.TrimSpace(u)
	}
	if rt.Username == "" {
		fmt.Println("Username cannot be empty.")
		return
	}
	if confirm("Change password?", false) {
		p, err := readPassword("New password: ")
		exitOnEOF(err)
		if err != nil {
			return
		}
		if p == "" {
			fmt.Println("Password cannot be empty.")
			return
		}
		rt.Password = p
	}
	syncClientCredentials(client, rt)
	fmt.Println("Credentials updated for this session.")
	if confirm("Save new credentials to config file?", true) {
		path := config.DefaultPath(rt.Paths)
		if err := warnPlaintextSave(path); err == nil {
			f := config.FileForPersist(*rt, cliFlags, rt.File)
			if err := config.Save(path, &f); err != nil {
				printErr(err)
			} else {
				rt.File = f
				rt.SourcePath = path
				fmt.Printf("Config saved to %s\n", path)
			}
		}
	}
}

func captureCLIFlags() config.CLIFlags {
	f := config.CLIFlags{}
	flag.Visit(func(fl *flag.Flag) {
		switch fl.Name {
		case "u", "username":
			f.UsernameSet = true
			f.Username = username
		case "p", "password":
			f.PasswordSet = true
			f.Password = password
		case "acid":
			f.AcIDSet = true
			f.AcID = acid
		case "f", "force":
			f.ForceSet = true
			f.Force = force
		case "b", "bypass":
			f.BypassSet = true
			f.Bypass = bypass
		case "a", "all":
			f.AllSet = true
			f.All = all
		case "logout-mode":
			f.LogoutModeSet = true
			f.LogoutMode = logoutMode
		}
	})
	return f
}

func mergedRuntime() config.Runtime {
	return config.Merge(&cfgRuntime, cliFlags, os.Getenv(srun.EnvUsername), os.Getenv(srun.EnvPassword))
}

func newClient(rt config.Runtime) *srun.Client {
	c := srun.NewClient(rt.Username, rt.Password, rt.AcID)
	c.SetVerbose(verbose)
	c.LogoutMode = rt.LogoutMode
	return c
}

// runBypass kicks sessions with random fake MACs to bypass billing.
//
// kickAll == false (default): only sessions matching this device's MAC are
// kicked. Safer when the account is shared with other people but is less
// likely to trigger the accounting desync because partial kicks may leave
// some sessions intact.
//
// kickAll == true: kick every session under the account. Can be more reliable
// and also clears any other devices/users on the same account.
func runBypass(client *srun.Client, kickAll bool) error {
	fmt.Println("\n--- Bypass Mode ---")
	macFilter := client.MAC
	if kickAll {
		macFilter = ""
	}
	if macFilter == "" && !kickAll {
		return fmt.Errorf("%w", srun.ErrMACUndetected)
	}

	kicked, sessions, err := srun.RunBypass(client.Username, macFilter, verbose, true)
	if err != nil {
		return err
	}
	if kicked == 1 {
		fmt.Println("Kicked 1 session with random fake MACs.")
	} else {
		fmt.Printf("Kicked %d sessions with random fake MACs.\n", kicked)
	}

	if len(sessions) == 0 {
		fmt.Println("No sessions visible after kick. Device should reconnect shortly.")
	} else {
		fmt.Printf("%d session(s) online after kick:\n", len(sessions))
		for _, sess := range sessions {
			tag := ""
			if client.MAC != "" && sess.MAC == client.MAC {
				tag = "  (your device)"
			}
			fmt.Printf("  id=%s mac=%s%s\n", sess.ID, sess.MAC, tag)
		}
		if !kickAll {
			fmt.Println("Note: only the middle session of your device's MAC was kicked to bypass billing.")
			fmt.Println("      To kick every session under the account, use -a.")
		} else {
			fmt.Println("Tip: newly created session(s) are typically NOT billed.")
		}
	}
	fmt.Println("--- Bypass Complete ---")
	return nil
}

func nonInteractiveRun(rt config.Runtime) error {
	client := newClient(rt)

	if rt.Force {
		if err := client.QuietLogOut(); err != nil && verbose {
			fmt.Fprintf(os.Stderr, "Warning: pre-login logout: %v\n", err)
		}
		time.Sleep(srun.LogoutSettleDelay)
	}

	info, err := client.LogIn()
	if err != nil {
		return err
	}
	fmt.Print(srun.FormatLoginInfo(info))

	if rt.Bypass {
		if err := runBypass(client, rt.All); err != nil {
			return err
		}
	}
	return nil
}

func warnPlaintextSave(path string) error {
	fmt.Fprintf(os.Stderr, "\nWARNING: Password will be stored in PLAIN TEXT at:\n  %s\n", path)
	fmt.Fprintln(os.Stderr, "Do not share this file or your user account. Press Enter to confirm, Ctrl+D to cancel.")
	_, err := readLine("")
	if err != nil {
		exitOnEOF(err)
	}
	return err
}

func askSaveCredentials(rt *config.Runtime, client *srun.Client) {
	if noConfig {
		printSaveNoticeOnce("--no-config: credentials are not written to disk")
		return
	}
	if reason := rt.SavePromptSkipReason(); reason != "" {
		printSaveNoticeOnce(reason)
		return
	}
	if !rt.ShouldOfferSavePrompt() {
		return
	}

	fmt.Println()
	fmt.Println("┌─ Save credentials? ────────────────────────────────────────────┐")
	fmt.Println("│  [y]      Save to user config dir (password stored in PLAIN)   │")
	fmt.Println("│  [n]      Skip this time (default)                             │")
	fmt.Println("│  [never]  Never ask again on this machine                      │")
	fmt.Println("└────────────────────────────────────────────────────────────────┘")
	ans, err := readLine("Choice [y/n/never] (default n): ")
	if err != nil {
		exitOnEOF(err)
		return
	}
	switch strings.ToLower(strings.TrimSpace(ans)) {
	case "n", "no", "":
		return
	case "never":
		if err := config.SaveNeverAskMarker(rt.Paths); err != nil {
			fmt.Fprintf(os.Stderr, "Could not save preference: %v\n", err)
		} else {
			rt.SavePromptDisabled = true
			fmt.Println("Save prompt disabled. Re-enable from Settings menu later if needed.")
		}
		return
	case "y", "yes":
		// continue
	default:
		fmt.Println("Unrecognized answer; treating as 'n'.")
		return
	}

	auto := confirm("Enable auto-auth on next launch (no args)?", true)
	path := config.DefaultPath(rt.Paths)
	if err := warnPlaintextSave(path); err != nil {
		return
	}

	rt.AutoAuth = auto
	f := config.FileForPersist(*rt, cliFlags, rt.File)
	if err := config.Save(path, &f); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to save config: %v\n", err)
		return
	}
	rt.File = f
	rt.SourcePath = path
	fmt.Printf("Config saved to %s\n", path)
}

// saveNoticeShown prints skip/save hints at most once per interactive session.
var saveNoticeShown bool

func printSaveNoticeOnce(msg string) {
	if msg == "" || saveNoticeShown {
		return
	}
	fmt.Println("(" + msg + ")")
	saveNoticeShown = true
}

func loginWithOnlineCheck(client *srun.Client, forceRelogin bool) (*srun.LoginInfo, error) {
	needLogout := forceRelogin
	if !forceRelogin {
		if info, err := client.GetLoginInfo(); err == nil {
			msg := fmt.Sprintf("Already online as %s, IP %s", info.Username, info.IP)
			if !confirm(msg+". Continue with new login?", false) {
				return info, srun.ErrStayOnline
			}
			// User wants to re-login while already online: srun will return
			// E2620 "already online" if we don't logout first, so do it here.
			needLogout = true
		}
	}
	if needLogout {
		if err := client.QuietLogOut(); err != nil && verbose {
			fmt.Fprintf(os.Stderr, "Warning: logout: %v\n", err)
		}
		time.Sleep(srun.LogoutSettleDelay)
	}
	return client.LogIn()
}

func printLoginOutcome(info *srun.LoginInfo, err error) {
	if errors.Is(err, srun.ErrStayOnline) {
		if info != nil {
			fmt.Printf("Stayed online as %s, IP %s\n", info.Username, info.IP)
		} else {
			fmt.Println("Stayed online (no new login).")
		}
		return
	}
	if err != nil {
		printErr(err)
		return
	}
	fmt.Print(srun.FormatLoginInfo(info))
}

func onOff(v bool) string {
	if v {
		return colorBoldGreen + "ON" + colorReset
	}
	return colorBoldRed + "OFF" + colorReset
}

// saveRuntimeConfig writes rt to the user config file (pipeline flags from rt.File).
func saveRuntimeConfig(rt *config.Runtime) error {
	if noConfig {
		return errors.New("not written: --no-config")
	}
	if rt.Paths.User == "" {
		return errors.New("user config path unavailable")
	}
	if err := config.PersistRuntime(rt.Paths, *rt, cliFlags, rt.File); err != nil {
		return err
	}
	rt.File = config.FileForPersist(*rt, cliFlags, rt.File)
	rt.SourcePath = rt.Paths.User
	return nil
}

func logoutModeDisplay(mode string) string {
	if mode == "selfservice" {
		return "selfservice"
	}
	return "portal"
}

func logoutModeBadge(mode string) string {
	if mode == "selfservice" {
		return "[" + colorBoldCyan + " selfservice " + colorReset + "]"
	}
	return "[" + colorBoldGreen + " portal " + colorReset + "]"
}

func settingsMenu(rt *config.Runtime, client *srun.Client) {
	for {
		fmt.Println("\n" + colorBoldCyan + "┌" + strings.Repeat("─", menuInnerWidth) + "┐" + colorReset)
		printBoxLine("   Settings & Configuration", "", menuInnerWidth)
		fmt.Println(colorBoldCyan + "├" + strings.Repeat("─", menuInnerWidth) + "┤" + colorReset)
		printBoxLine("  Auto-Auth  : ", badge(rt.AutoAuth), menuInnerWidth)
		printBoxLine("  Force      : ", badge(rt.Force), menuInnerWidth)
		printBoxLine("  Bypass     : ", badge(rt.Bypass), menuInnerWidth)
		printBoxLine("  Kick-All   : ", badge(rt.All), menuInnerWidth)
		printBoxLine("  Logout-Mode: ", logoutModeBadge(rt.LogoutMode), menuInnerWidth)
		fmt.Println(colorBoldCyan + "├" + strings.Repeat("─", menuInnerWidth) + "┤" + colorReset)
		printBoxLine("  1) Save current credentials as config", "", menuInnerWidth)
		printBoxLine("  2) Toggle auto-auth (saved immediately)", "", menuInnerWidth)
		printBoxLine("  3) Toggle force (logout before auto-login)", "", menuInnerWidth)
		printBoxLine("  4) Toggle bypass (bypass after auto-login)", "", menuInnerWidth)
		printBoxLine("  5) Toggle kick-all (-a) (bypass all devices)", "", menuInnerWidth)
		printBoxLine("  6) Toggle logout mode (portal / selfservice)", "", menuInnerWidth)
		printBoxLine("  7) Show config paths and redacted contents", "", menuInnerWidth)
		printBoxLine("  8) Delete config files", "", menuInnerWidth)
		printBoxLine("  9) Re-enable save prompt", "", menuInnerWidth)
		printBoxLine("  0) Back", "", menuInnerWidth)
		fmt.Println(colorBoldCyan + "└" + strings.Repeat("─", menuInnerWidth) + "┘" + colorReset)
		choice, err := readLine("Select an option [0-9]: ")
		if err != nil {
			exitOnEOF(err)
			return
		}
		switch choice {
		case "1":
			if !rt.HasCredentials() {
				fmt.Println("Set credentials first:")
				changeCredentials(rt, client)
				if !rt.HasCredentials() {
					continue
				}
			}
			path := config.DefaultPath(rt.Paths)
			if err := warnPlaintextSave(path); err != nil {
				continue
			}
			syncClientCredentials(client, rt)
			f := config.FileForPersist(*rt, cliFlags, rt.File)
			if err := config.Save(path, &f); err != nil {
				printErr(err)
			} else {
				rt.File = f
				rt.SourcePath = path
				fmt.Printf("Saved to %s\n", path)
			}
		case "2":
			if !rt.HasCredentials() {
				fmt.Println("Set credentials first:")
				changeCredentials(rt, client)
				if !rt.HasCredentials() {
					continue
				}
			}
			prev := rt.AutoAuth
			rt.AutoAuth = !rt.AutoAuth
			if noConfig {
				fmt.Printf("auto_auth is now %s (not written: --no-config)\n", onOff(rt.AutoAuth))
				continue
			}
			syncClientCredentials(client, rt)
			if err := saveRuntimeConfig(rt); err != nil {
				rt.AutoAuth = prev
				printErr(err)
				continue
			}
			fmt.Printf("auto_auth is now %s (saved)\n", onOff(rt.AutoAuth))
		case "3":
			prev := rt.Force
			rt.Force = !rt.Force
			if noConfig {
				fmt.Printf("force is now %s (not written: --no-config)\n", onOff(rt.Force))
				continue
			}
			if err := saveRuntimeConfig(rt); err != nil {
				rt.Force = prev
				printErr(err)
				continue
			}
			fmt.Printf("force is now %s (saved)\n", onOff(rt.Force))
		case "4":
			prevB, prevA := rt.Bypass, rt.All
			rt.Bypass = !rt.Bypass
			if !rt.Bypass {
				rt.All = false
			}
			if noConfig {
				fmt.Printf("bypass is now %s, all is %s (not written: --no-config)\n", onOff(rt.Bypass), onOff(rt.All))
				continue
			}
			if err := saveRuntimeConfig(rt); err != nil {
				rt.Bypass, rt.All = prevB, prevA
				printErr(err)
				continue
			}
			fmt.Printf("bypass is now %s, all is %s (saved)\n", onOff(rt.Bypass), onOff(rt.All))
		case "5":
			if !rt.Bypass {
				fmt.Println("Enable bypass (option 4) first; kick-all only applies with bypass.")
				continue
			}
			if !rt.All {
				if !confirm("Kick ALL sessions on this account during auto-bypass? Other devices will be disconnected.", false) {
					continue
				}
			}
			prev := rt.All
			rt.All = !rt.All
			if noConfig {
				fmt.Printf("all is now %s (not written: --no-config)\n", onOff(rt.All))
				continue
			}
			if err := saveRuntimeConfig(rt); err != nil {
				rt.All = prev
				printErr(err)
				continue
			}
			fmt.Printf("all is now %s (saved)\n", onOff(rt.All))
		case "6":
			prev := rt.LogoutMode
			if rt.LogoutMode == "selfservice" {
				rt.LogoutMode = "portal"
			} else {
				rt.LogoutMode = "selfservice"
			}
			if noConfig {
				fmt.Printf("logout_mode is now %s (not written: --no-config)\n", logoutModeDisplay(rt.LogoutMode))
				continue
			}
			syncClientCredentials(client, rt)
			if err := saveRuntimeConfig(rt); err != nil {
				rt.LogoutMode = prev
				printErr(err)
				continue
			}
			fmt.Printf("logout_mode is now %s (saved)\n", logoutModeDisplay(rt.LogoutMode))
		case "7":
			showConfigInfo(*rt)
		case "8":
			if rt.Paths.User == "" {
				fmt.Println("User config path unavailable.")
				break
			}
			if _, err := os.Stat(rt.Paths.User); os.IsNotExist(err) {
				fmt.Println("No config file to delete.")
				break
			}
			if !confirm(fmt.Sprintf("Delete %s? This cannot be undone.", rt.Paths.User), false) {
				fmt.Println("Cancelled.")
				break
			}
			if err := os.Remove(rt.Paths.User); err != nil && !os.IsNotExist(err) {
				fmt.Fprintf(os.Stderr, "remove %s: %v\n", rt.Paths.User, err)
			} else {
				fmt.Printf("Deleted %s\n", rt.Paths.User)
				rt.File = config.File{}
				rt.SourcePath = ""
				rt.AutoAuth = false
				rt.Force = false
				rt.Bypass = false
				rt.All = false
				rt.LogoutMode = ""
				rt.SavePromptDisabled = false
			}
		case "9":
			if err := config.ReenableSavePrompt(rt.Paths); err != nil {
				printErr(err)
			} else {
				rt.SavePromptDisabled = false
				fmt.Println("Save prompt re-enabled.")
			}
		case "0", "q", "quit", "exit", "back":
			return
		}
	}
}

func showConfigInfo(rt config.Runtime) {
	if rt.Paths.User == "" {
		fmt.Println("User config path unavailable.")
		return
	}
	fmt.Printf("\nConfig file: %s\n", rt.Paths.User)
	f, err := config.Load(config.LoadOptions{ExplicitPath: rt.Paths.User})
	if err != nil || f.SourcePath == "" {
		fmt.Println("  (not found or unreadable)")
	} else {
		b, _ := json.MarshalIndent(f.File.Redacted(), "  ", "  ")
		fmt.Printf("  %s\n", string(b))
	}
	if rt.SourcePath != "" {
		fmt.Printf("\nLoaded at startup: %s\n", rt.SourcePath)
	}
}

func runManageSessions(client *srun.Client) error {
	fmt.Println("\n--- Manage Active Sessions ---")
	ss := srun.NewSelfServiceClient()
	ss.SetVerbose(verbose)

	fmt.Println("Connecting to self-service portal...")
	if err := ss.SSOLogin(client.Username); err != nil {
		return fmt.Errorf("SSO login failed: %w", err)
	}

	for {
		fmt.Println("Fetching active sessions...")
		csrf, sessions, err := ss.GetSessions()
		if err != nil {
			return fmt.Errorf("failed to fetch sessions: %w", err)
		}

		if len(sessions) == 0 {
			fmt.Println("No active sessions found.")
			break
		}

		fmt.Printf("\nActive sessions under account %s:\n", client.Username)
		for i, sess := range sessions {
			tag := ""
			if client.MAC != "" && srun.MACEqual(sess.MAC, client.MAC) {
				tag = " (your current device)"
			}
			fmt.Printf("  %d) ID: %s | MAC: %s%s\n", i+1, sess.ID, sess.MAC, tag)
		}

		fmt.Println("\nOptions:")
		fmt.Println("  [1-N]  Enter session number to kick that session")
		fmt.Println("  all    Kick ALL sessions listed above")
		fmt.Println("  0      Back to main menu")

		input, err := readLine("\nChoose an option: ")
		if err != nil {
			return err
		}

		input = strings.TrimSpace(strings.ToLower(input))
		if input == "0" || input == "back" || input == "q" || input == "quit" || input == "exit" {
			break
		}

		if input == "all" {
			if !confirm("Are you sure you want to kick ALL active sessions?", false) {
				continue
			}
			fmt.Println("Kicking all sessions...")
			kickedCount := 0
			for _, sess := range sessions {
				fmt.Printf("Kicking session %s (MAC: %s)... ", sess.ID, sess.MAC)
				if err := ss.KickSession(sess.ID, sess.MAC, csrf); err != nil {
					fmt.Printf("Failed: %v\n", err)
				} else {
					fmt.Println("Success")
					kickedCount++
				}
			}
			if len(sessions) == 1 {
				fmt.Printf("Kicked %d of 1 session.\n", kickedCount)
			} else {
				fmt.Printf("Kicked %d of %d sessions.\n", kickedCount, len(sessions))
			}
			break
		}

		idx, err := strconv.Atoi(input)
		if err != nil || idx < 1 || idx > len(sessions) {
			fmt.Printf("Invalid option: %s. Enter a number between 1 and %d, 'all', or '0'.\n", input, len(sessions))
			continue
		}

		targetSess := sessions[idx-1]
		fmt.Printf("Kicking session %s (MAC: %s)... ", targetSess.ID, targetSess.MAC)
		if err := ss.KickSession(targetSess.ID, targetSess.MAC, csrf); err != nil {
			fmt.Printf("Failed: %v\n", err)
		} else {
			fmt.Println("Success")
		}
	}

	fmt.Println("--- Session Management Complete ---")
	return nil
}

func interactiveRun(rt *config.Runtime) {
	client := newClient(*rt)

	for {
		configStr, statusStr := getStatusHeader(client, rt)

		fmt.Println("\n" + colorBoldCyan + "┌" + strings.Repeat("─", menuInnerWidth) + "┐" + colorReset)
		printBoxLine("     NWAFU SRUN Portal Client", "", menuInnerWidth)
		fmt.Println(colorBoldCyan + "├" + strings.Repeat("─", menuInnerWidth) + "┤" + colorReset)
		printBoxLine("  "+configStr, "", menuInnerWidth)
		printBoxLine("  "+statusStr, "", menuInnerWidth)
		fmt.Println(colorBoldCyan + "├" + strings.Repeat("─", menuInnerWidth) + "┤" + colorReset)
		printBoxLine("  1) Login", "", menuInnerWidth)
		printBoxLine("  2) Force re-login", "", menuInnerWidth)
		printBoxLine("  3) Logout", "", menuInnerWidth)
		printBoxLine("  4) Status", "", menuInnerWidth)
		printBoxLine("  5) Bypass billing", "", menuInnerWidth)
		printBoxLine("  6) Manage active sessions", "", menuInnerWidth)
		printBoxLine("  7) Settings", "", menuInnerWidth)
		printBoxLine("  8) Change credentials", "", menuInnerWidth)
		printBoxLine("  0) Exit", "", menuInnerWidth)
		fmt.Println(colorBoldCyan + "└" + strings.Repeat("─", menuInnerWidth) + "┘" + colorReset)

		command, err := readLine("Select an option [0-8]: ")
		if err != nil {
			exitOnEOF(err)
			continue
		}

		switch command {
		case "1":
			if err := promptCredentials(rt); err != nil {
				printErr(err)
				continue
			}
			syncClientCredentials(client, rt)
			info, err := loginWithOnlineCheck(client, false)
			if err != nil && !errors.Is(err, srun.ErrStayOnline) {
				printErr(err)
				continue
			}
			printLoginOutcome(info, err)
			invalidateStatusCache()
			if err == nil {
				askSaveCredentials(rt, client)
			}
		case "2":
			if err := promptCredentials(rt); err != nil {
				printErr(err)
				continue
			}
			syncClientCredentials(client, rt)
			info, err := loginWithOnlineCheck(client, true)
			if err != nil {
				printErr(err)
				continue
			}
			printLoginOutcome(info, nil)
			invalidateStatusCache()
			askSaveCredentials(rt, client)
		case "3":
			if err := promptUsername(rt); err != nil {
				printErr(err)
				continue
			}
			syncClientCredentials(client, rt)
			if client.LogoutMode == "selfservice" {
				fmt.Println("Using self-service logout (kicking MAC sessions)...")
			} else {
				fmt.Println("Using portal logout...")
			}
			if err := client.LogOut(); err != nil {
				fmt.Println("-----------------------------------------")
				fmt.Println("             Fail to logout              ")
				printErr(err)
				fmt.Println("-----------------------------------------")
			} else {
				fmt.Println("-----------------------------------------")
				fmt.Println("           Logout successfully           ")
				fmt.Println("-----------------------------------------")
			}
			invalidateStatusCache()
		case "4":
			info, err := client.GetLoginInfo()
			if err != nil {
				printErr(err)
				continue
			}
			fmt.Print(srun.FormatStatusInfo(info))
			invalidateStatusCache()
		case "5":
			if err := promptCredentials(rt); err != nil {
				printErr(err)
				continue
			}
			syncClientCredentials(client, rt)
			if client.MAC == "" {
				if _, err := client.GetLoginInfo(); err != nil && verbose {
					fmt.Fprintf(os.Stderr, "Warning: could not refresh MAC: %v\n", err)
				}
			}
			fmt.Println("Bypass kicks your current device's session by default.")
			fmt.Println("To kick every session under the account instead, use the kick-all (-a) flag.")
			kickAll := confirm("Kick ALL sessions on this account?", false)
			if err := runBypass(client, kickAll); err != nil {
				printErr(err)
				continue
			}
			invalidateStatusCache()
		case "6":
			if err := promptCredentials(rt); err != nil {
				printErr(err)
				continue
			}
			syncClientCredentials(client, rt)
			if err := runManageSessions(client); err != nil {
				printErr(err)
			}
			invalidateStatusCache()
		case "7":
			settingsMenu(rt, client)
		case "8":
			changeCredentials(rt, client)
		case "0", "q", "quit", "exit":
			os.Exit(exitOK)
		default:
			fmt.Println("Invalid input. Please enter a number between 0 and 8.")
		}
	}
}

func doSaveConfig(rt config.Runtime) {
	path := config.DefaultPath(rt.Paths)
	if !rt.HasCredentials() {
		fail(exitUsage, "cannot save config: need username and password (-u/-p, env %s/%s, or existing config file)",
			srun.EnvUsername, srun.EnvPassword)
	}
	if err := warnPlaintextSave(path); err != nil {
		exitOnEOF(err)
		os.Exit(exitUsage)
	}
	f := config.FileForPersist(rt, cliFlags, cfgRuntime.File)
	if err := config.Save(path, &f); err != nil {
		fail(exitRuntime, "save config: %v", err)
	}
	fmt.Printf("Config saved to %s\n", path)
}

func main() {
	flag.Parse()

	if help {
		guide(os.Args[0])
		os.Exit(exitOK)
	}

	cliFlags = captureCLIFlags()

	loaded, err := config.Load(config.LoadOptions{
		ExplicitPath: configPath,
		NoConfig:     noConfig,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: %v\n", err)
		loaded = &config.Runtime{}
	}
	cfgRuntime = *loaded

	rt := mergedRuntime()
	if rt.AcID == "" {
		rt.AcID = acid
	}
	if rt.AcID == "" {
		rt.AcID = "1"
	}

	if saveConfig {
		doSaveConfig(rt)
		os.Exit(exitOK)
	}

	explicitCLI := cliFlags.UsernameSet && cliFlags.PasswordSet
	pipelineFlags := cliFlags.ForceSet || cliFlags.BypassSet
	wantPipeline := rt.HasCredentials() && (explicitCLI || (rt.AutoAuth && !menu) || pipelineFlags)

	if (explicitCLI || pipelineFlags) && !rt.HasCredentials() {
		fail(exitUsage, "missing credentials: provide -u and -p, set %s/%s, or use a config file with both fields",
			srun.EnvUsername, srun.EnvPassword)
	}

	if wantPipeline {
		isAutoAuthOnly := rt.AutoAuth && !explicitCLI && !pipelineFlags
		if isAutoAuthOnly {
			startInterruptMonitor()
			if checkInterrupt() {
				interactiveRun(&rt)
				return
			}
		}
		
		err := nonInteractiveRun(rt)
		if err != nil {
			printErr(err)
			if isAutoAuthOnly {
				fmt.Println("\nPress Enter to exit...")
				<-interruptChan
			}
			os.Exit(exitRuntime)
		}
		
		if isAutoAuthOnly {
			// Success: wait 2 seconds or until Enter is pressed
			select {
			case <-interruptChan:
			case <-time.After(2 * time.Second):
			}
		}
		return
	}

	interactiveRun(&rt)
}
