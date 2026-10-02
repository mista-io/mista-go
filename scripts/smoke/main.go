// Read-only check against the live API. Never sends messages.
//
//	MISTA_API_TOKEN=... go run ./scripts/smoke
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/mista-io/mista-go"
)

func main() {
	if os.Getenv("MISTA_API_TOKEN") == "" {
		fmt.Println("MISTA_API_TOKEN is not set; skipping live smoke test.")
		return
	}
	var opts []mista.Option
	if u := os.Getenv("MISTA_BASE_URL"); u != "" {
		opts = append(opts, mista.WithBaseURL(u))
	}
	ctx := context.Background()
	client := mista.NewClient("", opts...)

	balance, err := client.Account.Balance(ctx)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("balance:", balance.RemainingUnit, "expires", balance.ExpiredOn)

	me, err := client.Account.Me(ctx)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("account:", me.Email, me.Timezone)
}
