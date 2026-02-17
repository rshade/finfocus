package logging

import (
	"context"
	"time"

	"github.com/rs/zerolog"
)

// PhaseTimer tracks elapsed time for named phases within a multi-step operation.
// Use StartPhase to begin timing a phase, then call Done on the returned
// PhaseTimer to log the phase completion with elapsed_ms.
type PhaseTimer struct {
	start     time.Time
	log       *zerolog.Logger
	ctx       context.Context
	component string
	operation string
}

// StartPhase begins timing a named phase within an operation. Call Done on the
// returned PhaseTimer to log completion with elapsed_ms. The phase start is
// logged at Debug level; completion is logged at Info level.
func StartPhase(ctx context.Context, component, operation, phase string) PhaseTimer {
	log := FromContext(ctx)
	log.Debug().
		Ctx(ctx).
		Str("component", component).
		Str("operation", operation).
		Str("phase", phase).
		Msg(operation + " phase starting")
	return PhaseTimer{start: time.Now(), log: log, ctx: ctx, component: component, operation: operation}
}

// Done logs the phase completion at Info level with the elapsed time in milliseconds.
func (t PhaseTimer) Done(phase string) {
	t.log.Info().
		Ctx(t.ctx).
		Str("component", t.component).
		Str("operation", t.operation).
		Str("phase", phase).
		Int64("elapsed_ms", time.Since(t.start).Milliseconds()).
		Msg(t.operation + " phase complete")
}

// Elapsed returns the duration since the phase started.
func (t PhaseTimer) Elapsed() time.Duration {
	return time.Since(t.start)
}
