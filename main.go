package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"
	"unicode/utf8"

	"golang.org/x/term"

	"nwafu-srun/pkg/config"
	"nwafu-srun/pkg/srun"
)

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
	flag.BoolVar(&all, "a", false, "Kick all devices on account during bypass")
	flag.BoolVar(&all, "all", false, "Kick all devices on account")
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
	fmt.Printf("    %s -u <user> -p <pass> [-f] [-b] [-a] [--acid N] [-v]\n", argv)
	fmt.Printf("\nConfig:\n")
	fmt.Printf("    %s --save-config [-u ...] [-p ...]\n", argv)
	fmt.Printf("    %s --config <path>   %s --no-config\n", argv, argv)
	fmt.Printf("\nOptions:\n")
	fmt.Printf("  -u, -p           Credentials (or env %s / %s)\n", srun.EnvUsername, srun.EnvPassword)
	fmt.Printf("  -f               Logout before login\n")
	fmt.Printf("  -b               Bypass billing after login\n")
	fmt.Printf("  -a               Kick all devices during bypass\n")
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

func readLine(prompt string) (string, error) {
	fmt.Print(prompt)
	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(line), nil
}

func readPassword(prompt string) (string, error) {
	fmt.Print(prompt)
	if term.IsTerminal(int(os.Stdin.Fd())) {
		b, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Println()
		if err != nil {
			return "", err
		}
		return string(b), nil
	}
	return readLine("")
}

func confirm(prompt string, defaultYes bool) bool {
	def := "n"
	if defaultYes {
		def = "y"
	}
	ans, err := readLine(fmt.Sprintf("%s [%s]: ", prompt, def))
	if err != nil || ans == "" {
		return defaultYes
	}
	ans = strings.ToLower(ans)
	return ans == "y" || ans == "yes"
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

func runBypass(client *srun.Client, username string, kickAll bool) error {
	fmt.Println("\n--- Bypass Mode ---")
	macFilter := client.MAC
	if kickAll {
		macFilter = ""
	}
	if macFilter == "" && !kickAll {
		return fmt.Errorf("%w", srun.ErrMACUndetected)
	}

	kicked, sessions, err := srun.RunBypass(username, macFilter, verbose, true)
	if err != nil {
		return err
	}
	fmt.Printf("Kicked %d sessions\n", kicked)

	if len(sessions) == 0 {
		fmt.Println("No sessions found (device may be reconnecting)")
	} else {
		fmt.Printf("%d sessions remaining:\n", len(sessions))
		for _, sess := range sessions {
			tag := ""
			if client.MAC != "" && sess.MAC != client.MAC {
				tag = "  <-- NOT yours!"
			}
			fmt.Printf("  id=%s mac=%s%s\n", sess.ID, sess.MAC, tag)
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
		if err := runBypass(client, rt.Username, rt.All); err != nil {
			printErr(err)
			os.Exit(exitRuntime)
		}
	}
}

func warnPlaintextSave(path string) error {
	fmt.Fprintf(os.Stderr, "\nWARNING: Password will be stored in PLAIN TEXT at:\n  %s\n", path)
	fmt.Fprintln(os.Stderr, "Do not share this file or your user account. Press Enter to confirm, Ctrl+C to cancel.")
	_, err := readLine("")
	return err
}

func askSaveCredentials(rt config.Runtime, client *srun.Client) {
	if noConfig || rt.SavePromptDisabled {
		return
	}
	if rt.File.CredentialsMatch(rt.Username, rt.Password) {
		return
	}

	fmt.Println("\nSave these credentials for auto-login next time?")
	fmt.Println("  [y] Yes   [n] No (this time)   [N] Never ask again")
	ans, err := readLine("> ")
	if err != nil {
		return
	}
	switch strings.ToLower(ans) {
	case "n", "no", "":
		return
	case "never", "N":
		if err := config.SaveNeverAskMarker(rt.Paths); err != nil {
			fmt.Fprintf(os.Stderr, "Could not save preference: %v\n", err)
		} else {
			fmt.Println("Save prompt disabled (preference stored in user config dir).")
		}
		return
	case "y", "yes":
		// continue
	default:
		return
	}

	auto := confirm("Enable auto-auth on next launch (no args)?", true)
	path := config.DefaultPath(rt.Paths)
	if err := warnPlaintextSave(path); err != nil {
		return
	}

	f := config.File{
		Version:  config.CurrentVersion,
		Username: rt.Username,
		Password: rt.Password,
		AcID:     rt.AcID,
		AutoAuth: auto,
		Force:    rt.Force,
		Bypass:   rt.Bypass,
		All:      rt.All,
	}
	if err := config.Save(path, &f); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to save config: %v\n", err)
		return
	}
	fmt.Printf("Config saved to %s\n", path)
}

func loginWithOnlineCheck(client *srun.Client, forceRelogin bool) (*srun.LoginInfo, error) {
	if !forceRelogin {
		if info, err := client.GetLoginInfo(); err == nil {
			msg := fmt.Sprintf("Already online as %s, IP %s", info.Username, info.IP)
			if !confirm(msg+". Continue with new login?", false) {
				return info, nil
			}
		}
	}
	if forceRelogin {
		if err := client.QuietLogOut(); err != nil && verbose {
			fmt.Fprintf(os.Stderr, "Warning: logout: %v\n", err)
		}
		time.Sleep(srun.LogoutSettleDelay)
	}
	return client.LogIn()
}

func settingsMenu(rt *config.Runtime, client *srun.Client) {
	for {
		autoStr := "OFF"
		if rt.AutoAuth {
			autoStr = "ON"
		}
		fmt.Printf("\n--- Settings (auto-auth: %s) ---\n", autoStr)
		fmt.Println("  1) Save current credentials as config")
		fmt.Println("  2) Toggle auto-auth in memory (save via option 1)")
		fmt.Println("  3) Show config paths and redacted contents")
		fmt.Println("  4) Delete config files")
		fmt.Println("  5) Re-enable save prompt")
		fmt.Println("  6) Back")
		choice, err := readLine("> ")
		if err != nil {
			return
		}
		switch choice {
		case "1":
			path := config.DefaultPath(rt.Paths)
			if err := warnPlaintextSave(path); err != nil {
				continue
			}
			f := config.File{
				Version:  config.CurrentVersion,
				Username: client.Username,
				Password: client.Password,
				AcID:     client.AcID,
				AutoAuth: rt.AutoAuth,
				Force:    rt.Force,
				Bypass:   rt.Bypass,
				All:      rt.All,
			}
			if err := config.Save(path, &f); err != nil {
				printErr(err)
			} else {
				fmt.Printf("Saved to %s\n", path)
			}
		case "2":
			rt.AutoAuth = !rt.AutoAuth
			fmt.Printf("auto_auth is now %v (use option 1 to persist)\n", rt.AutoAuth)
		case "3":
			showConfigInfo(*rt)
		case "4":
			if rt.Paths.User != "" {
				if err := os.Remove(rt.Paths.User); err != nil && !os.IsNotExist(err) {
					fmt.Fprintf(os.Stderr, "remove %s: %v\n", rt.Paths.User, err)
				} else if err == nil {
					fmt.Printf("Deleted %s\n", rt.Paths.User)
				}
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

func interactiveRun(rt config.Runtime) {
	if rt.Username == "" {
		u, err := readLine("Username: ")
		if err != nil {
			fail(exitUsage, "%v", err)
		}
		rt.Username = u
	}
	if rt.Password == "" {
		p, err := readPassword("Password: ")
		if err != nil {
			fail(exitUsage, "%v", err)
		}
		rt.Password = p
	}

	client := newClient(rt)

	for {
		fmt.Printf("\n%s\n", formatCenter("NWAFU SRUN Authentication Utility", 28))
		fmt.Printf("%s\n", strings.Repeat("-", 31))
		fmt.Printf("%s-%s\n", formatCenter("1", 15), formatCenter("Login", 15))
		fmt.Printf("%s-%s\n", formatCenter("2", 15), formatCenter("Force re-login", 15))
		fmt.Printf("%s-%s\n", formatCenter("3", 15), formatCenter("Logout", 15))
		fmt.Printf("%s-%s\n", formatCenter("4", 15), formatCenter("Status", 15))
		fmt.Printf("%s-%s\n", formatCenter("5", 15), formatCenter("Bypass billing", 15))
		fmt.Printf("%s-%s\n", formatCenter("6", 15), formatCenter("Settings", 15))
		fmt.Printf("%s-%s\n", formatCenter("7", 15), formatCenter("Exit", 15))
		fmt.Printf("%s\n\n", strings.Repeat("-", 31))

		command, err := readLine("")
		if err != nil {
			fail(exitUsage, "%v", err)
		}

		switch command {
		case "1":
			info, err := loginWithOnlineCheck(client, false)
			if err != nil {
				printErr(err)
				continue
			}
			fmt.Print(srun.FormatLoginInfo(info))
			askSaveCredentials(rt, client)
		case "2":
			info, err := loginWithOnlineCheck(client, true)
			if err != nil {
				printErr(err)
				continue
			}
			fmt.Print(srun.FormatLoginInfo(info))
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
			fmt.Print(srun.FormatLoginInfo(info))
		case "5":
			if client.MAC == "" {
				if info, err := client.GetLoginInfo(); err == nil {
					_ = info
				}
			}
			kickAll := confirm("Kick ALL devices on this account? (default: only this device)", false)
			if err := runBypass(client, rt.Username, kickAll); err != nil {
				printErr(err)
				continue
			}
		case "6":
			settingsMenu(&rt, client)
		case "7":
			os.Exit(exitOK)
		default:
			fmt.Printf("\n%s\n", formatCenter("Input error!", 28))
		}
	}
}

func doSaveConfig(rt config.Runtime) {
	path := config.DefaultPath(rt.Paths)
	if !rt.HasCredentials() {
		fail(exitUsage, "--save-config requires username and password (-u/-p or env)")
	}
	if err := warnPlaintextSave(path); err != nil {
		os.Exit(exitUsage)
	}
	f := config.File{
		Version:  config.CurrentVersion,
		Username: rt.Username,
		Password: rt.Password,
		AcID:     rt.AcID,
		AutoAuth: rt.AutoAuth,
		Force:    rt.Force,
		Bypass:   rt.Bypass,
		All:      rt.All,
	}
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
		fail(exitUsage, "load config: %v", err)
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

	// Non-interactive: explicit -u/-p, env creds with -f/-b, or config auto_auth.
	explicitCLI := cliFlags.UsernameSet && cliFlags.PasswordSet
	pipelineFlags := cliFlags.ForceSet || cliFlags.BypassSet
	if rt.HasCredentials() && (explicitCLI || rt.AutoAuth || pipelineFlags) {
		nonInteractiveRun(rt)
		return
	}

	interactiveRun(rt)
}
