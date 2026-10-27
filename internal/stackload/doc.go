// Package stackload loads Pulumi state and preview data into a shared inventory.
// Its internal dependencies are limited to pulumi, ingest, and engine. It performs
// no history recording, dismissal updates, plugin startup, or ESC resolution.
// State export and state file failures expose only safe categories, never backend
// text. A failed preview keeps Pulumi's error, as before the shared loader.
package stackload
