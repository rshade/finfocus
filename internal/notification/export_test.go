package notification

import "github.com/rshade/finfocus/internal/config"

// RegisterForTest sets the sender for a destination type.
func (d *Dispatcher) RegisterForTest(notificationType config.NotificationType, sender Sender) {
	d.register(notificationType, sender)
}
