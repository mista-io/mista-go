// MISTA_WEBHOOK_SECRET=whsec_... go run ./examples/webhook-server
//
// Receives delivery report webhooks on http://localhost:8080/mista/dlr.
// Expose it with a tunnel (e.g. cloudflared or ngrok) and register the public URL
// with client.Webhooks.Set or in the dashboard under Developers → Settings.
package main

import (
	"errors"
	"io"
	"log"
	"net/http"
	"os"
	"sync"

	"github.com/mista-io/mista-go"
)

func main() {
	secret := os.Getenv("MISTA_WEBHOOK_SECRET")
	if secret == "" {
		log.Fatal("set MISTA_WEBHOOK_SECRET")
	}

	var seen sync.Map

	http.HandleFunc("/mista/dlr", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		if err != nil {
			http.Error(w, "cannot read body", http.StatusBadRequest)
			return
		}
		event, err := mista.VerifyWebhook(body, r.Header.Get(mista.WebhookSignatureHeader), secret)
		if errors.Is(err, mista.ErrWebhookSignature) {
			http.Error(w, "invalid signature", http.StatusBadRequest)
			return
		}
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if _, duplicate := seen.LoadOrStore(event.ID, true); duplicate {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		switch event.Type {
		case mista.EventMessageDelivered:
			log.Printf("%s delivered to %s", event.Data.UID, event.Data.To)
		case mista.EventMessageFailed:
			log.Printf("%s to %s: %s (%s)", event.Data.UID, event.Data.To, event.Data.Status, event.Data.StatusDetail)
		case mista.EventWebhookTest:
			log.Printf("test event %s", event.ID)
		}
		w.WriteHeader(http.StatusNoContent)
	})

	log.Println("listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
