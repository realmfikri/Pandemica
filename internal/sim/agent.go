package sim

import "sync"

var (
	speedMu              sync.RWMutex
	CurrentSpeedModifier = 1.0
)

// SetCurrentSpeedModifier updates the package-level movement modifier used when
// calculating agent velocity. Values below zero are clamped to zero.
func SetCurrentSpeedModifier(modifier float64) {
	speedMu.Lock()
	defer speedMu.Unlock()

	if modifier < 0 {
		modifier = 0
	}

	CurrentSpeedModifier = modifier
}

// SpeedModifier returns the current movement modifier used by agents.
func SpeedModifier() float64 {
	speedMu.RLock()
	defer speedMu.RUnlock()

	return CurrentSpeedModifier
}

// Agent represents a moving participant in the simulation space.
// The base speed is scaled by the global CurrentSpeedModifier.
type Agent struct {
	X, Y       float64
	DirectionX float64
	DirectionY float64
	BaseSpeed  float64
}

// Step advances the agent's position by deltaSeconds, applying the global
// speed modifier to the base speed before movement.
func (a *Agent) Step(deltaSeconds float64) {
	modifier := SpeedModifier()
	speed := a.BaseSpeed * modifier
	a.X += a.DirectionX * speed * deltaSeconds
	a.Y += a.DirectionY * speed * deltaSeconds
}
