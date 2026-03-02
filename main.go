package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"nwafu-srun/pkg/srun"
)

var (
	username string
	password string
	force    bool
	verbose  bool
	help     bool
)

func init() {
	flag.StringVar(&username, "u", "", "Username")
	flag.StringVar(&username, "username", "", "Username")
	flag.StringVar(&password, "p", "", "Password")
	flag.StringVar(&password, "password", "", "Password")
	flag.BoolVar(&force, "f", false, "Force login (logout then login directly without interactive menu, like login.py)")
	flag.BoolVar(&force, "force", false, "Force login")
	flag.BoolVar(&verbose, "v", false, "Verbose output (print request URLs and responses)")
	flag.BoolVar(&verbose, "verbose", false, "Verbose output")
	flag.BoolVar(&help, "h", false, "Help")
	flag.BoolVar(&help, "help", false, "Help")
}

func guide(argv string) {
	fmt.Printf("Usage:\n%s -u <username> -p <password> [-f] [-v]\n%s --username=<username> --password=<password> [--force] [--verbose]\n\n", argv, argv)
}

func formatCenter(s string, width int) string {
	if len(s) >= width {
		return s
	}
	leftPad := (width - len(s)) / 2
	rightPad := width - len(s) - leftPad
	return strings.Repeat(" ", leftPad) + s + strings.Repeat(" ", rightPad)
}

func main() {
	flag.Parse()

	if help {
		guide(os.Args[0])
		os.Exit(0)
	}

	if username == "" || password == "" {
		guide(os.Args[0])
		os.Exit(2)
	}

	client := srun.NewClient(username, password)
	client.Verbose = verbose

	// If force flag is provided, execute logout then login immediately, like login.py
	if force {
		client.LogOut()
		time.Sleep(3 * time.Second) // Add a delay to ensure Srun backend processes the logout
		client.LogIn()
		return
	}

	// Interactive mode like main.py
	var command string
	for {
		fmt.Printf("\n%s\n", formatCenter("NWAFU SRUN Authentication Utility", 28))
		fmt.Printf("%s\n", strings.Repeat("-", 31))
		fmt.Printf("%s-%s\n", formatCenter("1", 15), formatCenter("Login", 15))
		fmt.Printf("%s-%s\n", formatCenter("2", 15), formatCenter("Logout", 15))
		fmt.Printf("%s-%s\n", formatCenter("3", 15), formatCenter("Status", 15))
		fmt.Printf("%s-%s\n", formatCenter("4", 15), formatCenter("Exit", 15))
		fmt.Printf("%s\n\n", strings.Repeat("-", 31))

		fmt.Scanln(&command)

		switch command {
		case "1":
			client.LogIn()
		case "2":
			client.LogOut()
		case "3":
			client.GetLoginInfo()
		case "4":
			os.Exit(0)
		default:
			fmt.Printf("\n%s\n", formatCenter("Input error!", 28))
		}
	}
}
