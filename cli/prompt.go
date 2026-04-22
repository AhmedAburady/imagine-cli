package cli

import (
	"fmt"
	"os"
	"strings"

	"golang.org/x/term"

	"github.com/AhmedAburady/imagine-cli/config"
)

// PromptForAPIKey prompts the user to enter their Gemini API key and saves it.
// Called on first-run when no key is present in env or config.
func PromptForAPIKey() string {
	fmt.Println("\033[33mNo API key found.\033[0m")
	fmt.Println()
	fmt.Println("Get your free API key from: https://aistudio.google.com/app/apikey")
	fmt.Println()
	fmt.Print("Enter your Gemini API key: ")

	keyBytes, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Println()
	if err != nil {
		fmt.Printf("\033[31mError:\033[0m Failed to read input: %v\n", err)
		os.Exit(1)
	}

	key := strings.TrimSpace(string(keyBytes))
	if key == "" {
		fmt.Println("\033[31mError:\033[0m API key cannot be empty")
		os.Exit(1)
	}

	if err := config.SaveAPIKey(key); err != nil {
		fmt.Printf("\033[31mError:\033[0m Failed to save API key: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("\033[32m✓\033[0m API key saved successfully")
	fmt.Println()

	return key
}
