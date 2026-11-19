# Implementation plan

## Design

Keep all interaction in the browser chart. Store the viewport as a bounded
slice of loaded checkpoints. Use native controls for pan, zoom, reset, and
series toggles. Render chart marks in SVG and mirror deployment selection in
native buttons. Reuse the existing comparison form, result, and endpoints.

## Testing

Add frontend tests for navigation, threshold toggles, marker selection,
and stale diff responses. Use the existing populated Playwright History
test to verify controls, focus, and the chart at desktop and mobile widths.

## Constitution check

Read the existing redacted response and format no amounts in JavaScript.
Use numeric data only for chart geometry. This change adds no persistence,
service interface, or pricing calculations.
