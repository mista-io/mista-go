// MISTA_API_TOKEN=... go run ./examples/send-sms "+1555***4567"
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/mista-io/mista-go"
)

func main() {
	if len(os.Args) < 2 {
		log.Fatal("usage: send-sms <phone>")
	}
	ctx := context.Background()
	client := mista.NewClient("") // reads MISTA_API_TOKEN

	msg, err := client.SMS.Send(ctx, &mista.SendSMSParams{
		To:       os.Args[1],
		SenderID: "YourBrand",
		Message:  "Hello from Mista",
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Queued", msg.UID)

	latest, err := client.Logs.Get(ctx, msg.UID)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Status", latest.Status)
}
