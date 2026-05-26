package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"
	"unicode/utf8"

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
)

var (
	cfgRuntime config.Runtime
	cliFlags   config.CLIFlags
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
	flag.BoolVar(&all, "a", false, "Kick ALL sessions on the account during bypass (required for the bypass to actually take effect)")
	flag.BoolVar(&all, "all", false, "Kick ALL sessions on the account during bypass")
	flag.StringVar(&acid, "acid", "1", "Access controller ID (ac_id)")
	flag.BoolVar(&verbose, "v", false, "Verbose output (stderr)")
	flag.BoolVar(&verbose, "verbose", false, "Verbose output")
	flag.BoolVar(&help, "h", false, "Help")
	flag.BoolVar(&help, "help", false, "Help")
	flag.StringVar(&configPath, "config", "", "Path to config file")
	flag.BoolVar(&noConfig, "no-config", false, "Ignore all config files")
	flag.BoolVar(&saveConfig, "save-config", false, "Save current flags to user config dir and exit")
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
	fmt.Printf("  -a               With -b: kick ALL sessions on the account (needed for bypass to take effect)\n")
	fmt.Printf("  --acid, -v, -h   See README\n")
	fmt.Printf("\nWith a saved config (auto_auth=true), running with no args performs auto-login.\n")
}

func formatCenter(s string, width int) string {
	n := utf8.RuneCountInString(s)
	if n >= width {
		return s
	}
	leftPad := (width - n) / 2
	rightPad := width - n - leftPad
	return strings.Repeat(" ", leftPad) + s + strings.Repeat(" ", rightPad)
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

const menuInnerWidth = 44

func boxLine(text string) string {
	n := utf8.RuneCountInString(text)
	if n >= menuInnerWidth {
		return "│" + text + "│"
	}
	return "│" + text + strings.Repeat(" ", menuInnerWidth-n) + "│"
}

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
}

func promptCredentials(rt *config.Runtime) error {
	if rt.Username == "" {
		if !term.IsTerminal(int(os.Stdin.Fd())) {
			return fmt.Errorf("username required via -u or %s when stdin is not a TTY", srun.EnvUsername)
		}
		u, err := readLine("Username: ")
		exitOnEOF(err)
		if err != nil {
			return err
		}
		u = strings.TrimSpace(u)
		if u == "" {
			return errors.New("username required")
		}
		rt.Username = u
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
	return c
}

// runBypass kicks sessions with random fake MACs to bypass billing.
//
// kickAll == false (default): only sessions matching this device's MAC are
// kicked. Safer when the account is shared with other people but is less
// likely to trigger the accounting desync because partial kicks may leave
// some sessions intact.
//
// kickAll == true: kick every session under the account. Most reliable for
// the bypass to actually take effect; also clears any other devices/users
// on the same account.
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
	fmt.Printf("Kicked %d sessions with random fake MACs.\n", kicked)

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
			fmt.Println("Note: only your own MAC was kicked. To trigger the accounting desync,")
			fmt.Println("      every session under the account must be kicked at once (use -a).")
		} else {
			fmt.Println("Tip: newly created session(s) are typically NOT billed.")
		}
	}
	fmt.Println("--- Bypass Complete ---")
	return nil
}

func nonInteractiveRun(rt config.Runtime) {
	client := newClient(rt)

	if rt.Force {
		if err := client.QuietLogOut(); err != nil && verbose {
			fmt.Fprintf(os.Stderr, "Warning: pre-login logout: %v\n", err)
		}
		time.Sleep(srun.LogoutSettleDelay)
	}

	info, err := client.LogIn()
	if err != nil {
		printErr(err)
		os.Exit(exitRuntime)
	}
	fmt.Print(srun.FormatLoginInfo(info))

	if rt.Bypass {
		if err := runBypass(client, rt.All); err != nil {
			printErr(err)
			os.Exit(exitRuntime)
		}
	}
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

func settingsMenu(rt *config.Runtime, client *srun.Client) {
	for {
		autoStr := "OFF"
		if rt.AutoAuth {
			autoStr = "ON"
		}
		fmt.Printf("\n--- Settings (auto-auth: %s) ---\n", autoStr)
		fmt.Println("  1) Save current credentials as config")
		fmt.Println("  2) Toggle auto-auth (saved immediately)")
		fmt.Println("  3) Show config paths and redacted contents")
		fmt.Println("  4) Delete config files")
		fmt.Println("  5) Re-enable save prompt")
		fmt.Println("  6) Back")
		choice, err := readLine("> ")
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
			if noConfig {
				rt.AutoAuth = !rt.AutoAuth
				fmt.Printf("auto_auth is now %v (not written: --no-config)\n", rt.AutoAuth)
				continue
			}
			syncClientCredentials(client, rt)
			rt.AutoAuth = !rt.AutoAuth
			if err := config.PersistRuntime(rt.Paths, *rt, cliFlags, rt.File); err != nil {
				printErr(err)
				rt.AutoAuth = !rt.AutoAuth
				continue
			}
			fmt.Printf("auto_auth is now %v (saved)\n", rt.AutoAuth)
		case "3":
			showConfigInfo(*rt)
		case "4":
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
				rt.SavePromptDisabled = false
			}
		case "5":
			if err := config.ReenableSavePrompt(rt.Paths); err != nil {
				printErr(err)
			} else {
				rt.SavePromptDisabled = false
				fmt.Println("Save prompt re-enabled.")
			}
		case "6":
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

func interactiveRun(rt *config.Runtime) {
	if err := promptCredentials(rt); err != nil {
		fail(exitUsage, "%v", err)
	}

	client := newClient(*rt)

	if confirm("\nLog in now? (recommended; verifies credentials and offers to save them)", true) {
		info, err := loginWithOnlineCheck(client, false)
		if err != nil && !errors.Is(err, srun.ErrStayOnline) {
			printErr(err)
			fmt.Fprintln(os.Stderr, "Retry from menu (1), change credentials (7), or exit (8).")
		} else {
			printLoginOutcome(info, err)
			if err == nil {
				askSaveCredentials(rt, client)
			}
		}
	}

	for {
		fmt.Println("\n┌" + strings.Repeat("─", menuInnerWidth) + "┐")
		fmt.Println(boxLine("      NWAFU SRUN Authentication Utility"))
		fmt.Println("├" + strings.Repeat("─", menuInnerWidth) + "┤")
		fmt.Println(boxLine("  1) Login"))
		fmt.Println(boxLine("  2) Force re-login"))
		fmt.Println(boxLine("  3) Logout"))
		fmt.Println(boxLine("  4) Status"))
		fmt.Println(boxLine("  5) Bypass billing"))
		fmt.Println(boxLine("  6) Settings"))
		fmt.Println(boxLine("  7) Change credentials"))
		fmt.Println(boxLine("  8) Exit"))
		fmt.Println("└" + strings.Repeat("─", menuInnerWidth) + "┘")

		command, err := readLine("Select an option [1-8]: ")
		if err != nil {
			exitOnEOF(err)
			continue
		}

		switch command {
		case "1":
			syncClientCredentials(client, rt)
			info, err := loginWithOnlineCheck(client, false)
			if err != nil && !errors.Is(err, srun.ErrStayOnline) {
				printErr(err)
				continue
			}
			printLoginOutcome(info, err)
			if err == nil {
				askSaveCredentials(rt, client)
			}
		case "2":
			syncClientCredentials(client, rt)
			info, err := loginWithOnlineCheck(client, true)
			if err != nil {
				printErr(err)
				continue
			}
			printLoginOutcome(info, nil)
			askSaveCredentials(rt, client)
		case "3":
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
		case "4":
			info, err := client.GetLoginInfo()
			if err != nil {
				printErr(err)
				continue
			}
			fmt.Print(srun.FormatStatusInfo(info))
		case "5":
			syncClientCredentials(client, rt)
			if client.MAC == "" {
				if _, err := client.GetLoginInfo(); err != nil && verbose {
					fmt.Fprintf(os.Stderr, "Warning: could not refresh MAC: %v\n", err)
				}
			}
			fmt.Println("Bypass requires kicking every session under the account at once")
			fmt.Println("for the RADIUS accounting desync to take effect. The default kicks")
			fmt.Println("only this device, which is safer if the account is shared.")
			kickAll := confirm("Kick ALL sessions on this account?", false)
			if err := runBypass(client, kickAll); err != nil {
				printErr(err)
				continue
			}
		case "6":
			settingsMenu(rt, client)
		case "7":
			changeCredentials(rt, client)
		case "8", "q", "quit", "exit":
			os.Exit(exitOK)
		default:
			fmt.Println("Invalid input. Please enter a number between 1 and 8.")
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

func warnAutoAuthPipeline(rt config.Runtime) {
	if !rt.AutoAuth {
		return
	}
	var parts []string
	if rt.Force {
		parts = append(parts, "force")
	}
	if rt.Bypass {
		parts = append(parts, "bypass")
	}
	if len(parts) == 0 {
		return
	}
	fmt.Fprintf(os.Stderr, "(config has auto_auth with %s; running full pipeline. Use --no-config for interactive menu.)\n",
		strings.Join(parts, "+"))
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
	wantPipeline := rt.HasCredentials() && (explicitCLI || rt.AutoAuth || pipelineFlags)

	if (explicitCLI || pipelineFlags) && !rt.HasCredentials() {
		fail(exitUsage, "missing credentials: provide -u and -p, set %s/%s, or use a config file with both fields",
			srun.EnvUsername, srun.EnvPassword)
	}

	if wantPipeline {
		warnAutoAuthPipeline(rt)
		nonInteractiveRun(rt)
		return
	}

	interactiveRun(&rt)
}
