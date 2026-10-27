// Package stackload loads Pulumi state and preview data into a shared inventory.
// Its internal dependencies are limited to pulumi, ingest, and engine. It performs
// no history recording, dismissal updates, plugin startup, or ESC resolution.
// Backend error text never leaves the package; failures expose only safe categories.
package stackload
