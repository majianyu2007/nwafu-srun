package main

import (
	"flag"
	"fmt"
	"os"
	"time"

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

func init() {
	flag.StringVar(&username, "u", "", "Username")
	flag.StringVar(&username, "username", "", "Username")
	flag.StringVar(&password, "p", "", "Password (required with --login)")
	flag.StringVar(&password, "password", "", "Password")
	flag.BoolVar(&loginFirst, "login", false, "Full flow: logout, login, then bypass")
	flag.BoolVar(&all, "a", false, "Kick all devices on account")
	flag.BoolVar(&all, "all", false, "Kick all devices on account")
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
		fail(exitUsage, err)
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
	if rt.All {
		macFilter = ""
	}
	if macFilter == "" && !rt.All {
		fail(exitRuntime, fmt.Errorf("%w", srun.ErrMACUndetected))
	}

	fmt.Println("\n--- Bypass ---")
	kicked, sessions, err := srun.RunBypass(rt.Username, macFilter, verbose, true)
	if err != nil {
		fail(exitRuntime, err)
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
	fmt.Println("--- Done ---")
}
