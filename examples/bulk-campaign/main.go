package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/mista-io/mista-go"
)

func main() {
	ctx := context.Background()
	client := mista.NewClient("")

	// Broadcast: one message to many numbers, tomorrow at 09:00 in the account timezone.
	kigali, err := time.LoadLocation("Africa/Kigali")
	if err != nil {
		log.Fatal(err)
	}
	now := time.Now().In(kigali)
	tomorrow9am := time.Date(now.Year(), now.Month(), now.Day()+1, 9, 0, 0, 0, kigali)

	broadcast, err := client.Campaigns.Bulk(ctx, &mista.BulkCampaignParams{
		SenderID:     "LOYALTY",
		Recipients:   []string{"+1555***4567", "+1555***7890"},
		Message:      "Double points this weekend!",
		ScheduleTime: mista.FormatScheduleTime(tomorrow9am),
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Broadcast", broadcast.UID, broadcast.Status)

	// Personalized: a different message per number.
	personalized, err := client.Campaigns.Bulk(ctx, &mista.BulkCampaignParams{
		SenderID: "LOYALTY",
		Personalized: []mista.PersonalizedRecipient{
			{To: "+1555***4567", Message: "Hi Alice, you have 120 points."},
			{To: "+1555***7890", Message: "Hi Bob, you have 45 points."},
		},
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Personalized", personalized.UID, personalized.RecipientCount)
}
