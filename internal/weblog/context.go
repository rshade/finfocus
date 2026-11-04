package weblog

import "context"

type hubKey struct{}

// ContextWithHub returns a context carrying the hub.
func ContextWithHub(ctx context.Context, h *Hub) context.Context {
	return context.WithValue(ctx, hubKey{}, h)
}

// HubFromContext returns the hub stored in ctx, or nil when there is none.
func HubFromContext(ctx context.Context) *Hub {
	h, _ := ctx.Value(hubKey{}).(*Hub)
	return h
}
