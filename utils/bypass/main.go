package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

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
	loginFirst bool
	all        bool
	acid       string
	verbose    bool
	help       bool
	configPath string
	noConfig   bool
)

var stdin = bufio.NewReader(os.Stdin)

func init() {
	flag.StringVar(&username, "u", "", "Username")
	flag.StringVar(&username, "username", "", "Username")
	flag.StringVar(&password, "p", "", "Password (required with --login)")
	flag.StringVar(&password, "password", "", "Password")
	flag.BoolVar(&loginFirst, "login", false, "Full flow: logout, login, then bypass")
	flag.BoolVar(&all, "a", false, "Kick ALL sessions on the account (required for the bypass to actually take effect)")
	flag.BoolVar(&all, "all", false, "Kick ALL sessions on the account")
	flag.StringVar(&acid, "acid", "1", "Access controller ID (--login only)")
	flag.BoolVar(&verbose, "v", false, "Verbose output (stderr)")
	flag.BoolVar(&verbose, "verbose", false, "Verbose output")
	flag.BoolVar(&help, "h", false, "Help")
	flag.BoolVar(&help, "help", false, "Help")
	flag.StringVar(&configPath, "config", "", "Config file path")
	flag.BoolVar(&noConfig, "no-config", false, "Ignore config files")
}

func guide(argv string) {
	fmt.Printf("Usage:\n")
	fmt.Printf("  %s -u <username>              # bypass-only (must be online)\n", argv)
	fmt.Printf("  %s -u <user> -p <pass> --login\n", argv)
	fmt.Printf("  %s                            # username from config if saved\n", argv)
	fmt.Printf("\nOptions: -u -p --login -a -v --config --no-config -h\n")
	fmt.Printf("By default only sessions matching this device's MAC are kicked. Use -a/--all\n")
	fmt.Printf("to kick every session under the account, which is required for the RADIUS\n")
	fmt.Printf("accounting desync to actually take effect (also clears any other devices).\n")
	fmt.Printf("Environment: %s, %s\n", srun.EnvUsername, srun.EnvPassword)
}

func fail(code int, err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		if h := srun.Hint(err); h != "" {
			fmt.Fprintf(os.Stderr, "Hint: %s\n", h)
		}
	}
	os.Exit(code)
}

func confirm(prompt string, defaultYes bool) bool {
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return defaultYes
	}
	def := "n"
	if defaultYes {
		def = "y"
	}
	fmt.Printf("%s [%s]: ", prompt, def)
	line, err := stdin.ReadString('\n')
	if err != nil {
		return defaultYes
	}
	line = strings.ToLower(strings.TrimSpace(line))
	if line == "" {
		return defaultYes
	}
	return line == "y" || line == "yes"
}

func main() {
	flag.Parse()

	if help {
		guide(os.Args[0])
		os.Exit(exitOK)
	}

	loaded, err := config.Load(config.LoadOptions{
		ExplicitPath: configPath,
		NoConfig:     noConfig,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: %v\n", err)
		loaded = &config.Runtime{}
	}

	cli := config.CLIFlags{}
	flag.Visit(func(fl *flag.Flag) {
		switch fl.Name {
		case "u", "username":
			cli.UsernameSet = true
		case "p", "password":
			cli.PasswordSet = true
		case "acid":
			cli.AcIDSet = true
		case "a", "all":
			cli.AllSet = true
		}
	})
	cli.Username = username
	cli.Password = password
	cli.AcID = acid
	cli.All = all

	rt := config.Merge(loaded, cli, os.Getenv(srun.EnvUsername), os.Getenv(srun.EnvPassword))
	if rt.AcID == "" {
		rt.AcID = "1"
	}
	if rt.Username == "" {
		fail(exitUsage, fmt.Errorf("username required (-u, env %s, or config)", srun.EnvUsername))
	}
	if loginFirst && rt.Password == "" {
		fail(exitUsage, fmt.Errorf("password required with --login"))
	}

	kickAll := rt.All
	if !cli.AllSet && rt.All {
		if !term.IsTerminal(int(os.Stdin.Fd())) {
			fail(exitUsage, fmt.Errorf("config has all=true but stdin is not a TTY; pass -a explicitly to confirm kick-all"))
		}
		fmt.Println("Config has all=true (kick every session on the account).")
		if !confirm("Kick ALL sessions on this account?", false) {
			kickAll = false
			fmt.Println("Proceeding with own-MAC sessions only.")
		} else {
			kickAll = true
		}
	}

	client := srun.NewClient(rt.Username, rt.Password, rt.AcID)
	client.SetVerbose(verbose)

	if loginFirst {
		if err := client.QuietLogOut(); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: logout before login: %v\n", err)
		}
		time.Sleep(srun.LogoutSettleDelay)
		info, err := client.LogIn()
		if err != nil {
			fail(exitRuntime, err)
		}
		fmt.Print(srun.FormatLoginInfo(info))
	} else {
		info, err := client.GetLoginInfo()
		if err != nil {
			fail(exitRuntime, err)
		}
		fmt.Print(srun.FormatLoginInfo(info))
	}

	macFilter := client.MAC
	if kickAll {
		macFilter = ""
	}
	if macFilter == "" && !kickAll {
		fail(exitRuntime, fmt.Errorf("%w", srun.ErrMACUndetected))
	}

	fmt.Println("\n--- Bypass ---")
	kicked, sessions, err := srun.RunBypass(rt.Username, macFilter, verbose, true)
	if err != nil {
		fail(exitRuntime, err)
	}
	fmt.Printf("Kicked %d sessions with random fake MACs.\n", kicked)

	if len(sessions) == 0 {
		fmt.Println("No sessions visible after kick. Device should reconnect shortly.")
	} else {
		fmt.Printf("%d session(s) online after kick:\n", len(sessions))
		for _, sess := range sessions {
			tag := ""
			if client.MAC != "" && normalizeMACEqual(sess.MAC, client.MAC) {
				tag = "  (your device)"
			}
			fmt.Printf("  id=%s mac=%s%s\n", sess.ID, sess.MAC, tag)
		}
		if !kickAll {
			fmt.Println("Note: only your own MAC was kicked. Pass -a to kick ALL sessions and")
			fmt.Println("      actually trigger the accounting desync.")
		} else {
			fmt.Println("Tip: newly created session(s) are typically NOT billed.")
		}
	}
	fmt.Println("--- Done ---")
}

func normalizeMACEqual(a, b string) bool {
	return strings.ReplaceAll(strings.ToLower(a), "-", ":") ==
		strings.ReplaceAll(strings.ToLower(b), "-", ":")
}
