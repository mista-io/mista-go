package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/mista-io/mista-go"
)

func main() {
	ctx := context.Background()
	client := mista.NewClient("")

	v, err := client.Verify.Start(ctx, &mista.StartVerificationParams{To: "+250780000001", Channel: "sms"})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Code sent, sid", v.SID)

	fmt.Print("Code: ")
	code, _ := bufio.NewReader(os.Stdin).ReadString('\n')

	result, err := client.Verify.Check(ctx, v.SID, strings.TrimSpace(code))
	if err != nil {
		log.Fatal(err)
	}
	if result.Verified {
		fmt.Println("Verified")
	} else {
		fmt.Println("Rejected:", result.Reason)
	}
}
