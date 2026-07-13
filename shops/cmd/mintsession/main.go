// Command mintsession prints a valid session cookie for an email, signed with
// SESSION_SECRET. Useful for exercising authed routes with curl or a browser
// during development without doing the OAuth dance.
package main

import (
	"fmt"
	"log"
	"os"

	"github.com/ireydiak/shops/internal/auth"
)

func main() {
	if len(os.Args) != 2 {
		log.Fatal("usage: SESSION_SECRET=... mintsession user@example.com")
	}
	secret := os.Getenv("SESSION_SECRET")
	if secret == "" {
		log.Fatal("SESSION_SECRET is not set")
	}
	baseURL := os.Getenv("BASE_URL")
	if baseURL == "" {
		baseURL = "http://localhost:8080"
	}
	svc, err := auth.NewService(auth.Config{Secret: secret, BaseURL: baseURL})
	if err != nil {
		log.Fatal(err)
	}
	cookie, err := svc.MintSession(os.Args[1])
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("%s=%s\n", cookie.Name, cookie.Value)
}
