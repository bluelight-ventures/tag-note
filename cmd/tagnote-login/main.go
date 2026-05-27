package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/runminglu/tag-note/internal/apiclient"
)

func main() {
	reader := bufio.NewReader(os.Stdin)

	fmt.Print("Email: ")
	email, _ := reader.ReadString('\n')
	email = strings.TrimSpace(email)

	fmt.Print("Password: ")
	password, _ := reader.ReadString('\n')
	password = strings.TrimSpace(password)

	token, err := apiclient.Login(email, password)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("\nLogin successful! Set this environment variable:")
	fmt.Printf("export TAGNOTE_TOKEN=%s\n", token)
}
