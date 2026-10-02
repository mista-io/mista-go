package main

import (
	"context"
	"errors"
	"fmt"
	"log"

	"github.com/mista-io/mista-go"
)

func main() {
	ctx := context.Background()
	client := mista.NewClient("")

	group, err := client.ContactGroups.Create(ctx, "Developers")
	if err != nil {
		log.Fatal(err)
	}

	contact, err := client.Contacts.Create(ctx, group.UID, &mista.ContactFields{
		Phone:     "250780000001",
		FirstName: "Alice",
		LastName:  "Uwase",
	})
	var apiErr *mista.APIError
	switch {
	case errors.As(err, &apiErr):
		fmt.Println("Not added:", apiErr.Message)
	case err != nil:
		log.Fatal(err)
	default:
		fmt.Println("Added", contact.UID)
	}

	it := client.Contacts.ListAutoPaging(ctx, group.UID)
	for it.Next(ctx) {
		c := it.Current()
		fmt.Println(c.Phone, c.FirstName)
	}
	if err := it.Err(); err != nil {
		log.Fatal(err)
	}

	if _, err := client.Campaigns.SendToGroups(ctx, &mista.GroupCampaignParams{
		GroupUIDs: []string{group.UID},
		SenderID:  "YourBrand",
		Message:   "Welcome!",
	}); err != nil {
		log.Fatal(err)
	}
}
