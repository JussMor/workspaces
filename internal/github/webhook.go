package github

import (
	"log"
	"net/http"
)

// HandleWebhook processes an inbound GitHub webhook event.
//
// Reads the X-GitHub-Event header to identify the event type, logs it,
// and returns 501 Not Implemented until the full handler is built.
//
// TODO(forge): implement per docs/platform-plan.jsx Week 9
func HandleWebhook(w http.ResponseWriter, r *http.Request) {
	event := r.Header.Get("X-GitHub-Event")
	log.Printf("github webhook: received event=%q (not yet handled)", event)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotImplemented)
	_, _ = w.Write([]byte(`{"error":"not implemented","layer":"github","week":"9"}`))
}
