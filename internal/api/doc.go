// Package api is LAYER 06 of FORGE — the Orchestrator API.
//
// Week 10 of the platform build. Exposes an HTTP API (chi router) for the
// dashboard and external clients to interact with the platform. Routes
// delegate to the Coordinator and other layers via dependency injection.
// WebSocket endpoints stream real-time task logs and notifications.
package api
