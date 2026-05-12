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
	flag.BoolVar(&force, "f", false, "Force login (logout then login directly without interactive menu)")
	flag.BoolVar(&force, "force", false, "Force login")
	flag.BoolVar(&verbose, "v", false, "Verbose output (print request URLs and responses)")
	flag.BoolVar(&verbose, "verbose", false, "Verbose output")
	flag.BoolVar(&help, "h", false, "Help")
	flag.BoolVar(&help, "help", false, "Help")
}

func guide(argv string) {
	fmt.Printf("Usage:\n")
	fmt.Printf("  Interactive mode:\n")
	fmt.Printf("    %s [-u <username>] [-p <password>] [-v]\n", argv)
	fmt.Printf("  Force mode:\n")
	fmt.Printf("    %s -u <username> -p <password> -f [-v]\n", argv)
	fmt.Printf("\nOptions:\n")
	fmt.Printf("  -u, --username  Username (optional in interactive mode)\n")
	fmt.Printf("  -p, --password  Password (optional in interactive mode)\n")
	fmt.Printf("  -f, --force     Force login (logout then login, requires -u and -p)\n")
	fmt.Printf("  -v, --verbose   Verbose output\n")
	fmt.Printf("  -h, --help      Show this help\n")
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

	if force && (username == "" || password == "") {
		fmt.Println("Error: -f/--force requires -u/--username and -p/--password")
		guide(os.Args[0])
		os.Exit(2)
	}

	// Interactive mode: prompt for credentials if not provided via flags
	if !force && username == "" {
		fmt.Print("Username: ")
		fmt.Scanln(&username)
	}
	if !force && password == "" {
		fmt.Print("Password: ")
		fmt.Scanln(&password)
	}

	client := srun.NewClient(username, password)
	client.Verbose = verbose

	// If force flag is provided, execute logout then login immediately
	if force {
		client.LogOut()
		time.Sleep(3 * time.Second) // Add a delay to ensure Srun backend processes the logout
		client.LogIn()
		return
	}

	// Interactive mode
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
