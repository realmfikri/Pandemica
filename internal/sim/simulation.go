package sim

import (
	"context"
	"log"
	"math"
	"math/rand"
	"sync"
	"time"
)

// Simulation tracks transmission probabilities and exposes knobs to adjust the
// spread model.
type Simulation struct {
	mu               sync.RWMutex
	transmissionMod  float64
	modifierSet      bool
	baseTransmission float64
	rng              *rand.Rand
	lockdownEnabled  bool
}

// New creates a simulation with the provided base transmission probability.
// If baseTransmission is zero, a default of 0.25 is used.
func New(baseTransmission float64) *Simulation {
	if baseTransmission <= 0 {
		baseTransmission = 0.25
	}
	SetCurrentSpeedModifier(1.0)
	return &Simulation{
		transmissionMod:  1.0,
		modifierSet:      false,
		baseTransmission: baseTransmission,
		rng:              rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// SetLockdown applies a reduced movement speed when enabled and restores the
// default when disabled.
func (s *Simulation) SetLockdown(enabled bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.lockdownEnabled = enabled
	if enabled {
		SetCurrentSpeedModifier(0.1)
	} else {
		SetCurrentSpeedModifier(1.0)
	}
}

// LockdownEnabled reports the current lockdown flag.
func (s *Simulation) LockdownEnabled() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.lockdownEnabled
}

// UpdateTransmissionModifier records a UI-driven transmission modifier.
func (s *Simulation) UpdateTransmissionModifier(modifier float64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if modifier < 0 {
		modifier = 0
	} else if modifier > 1 {
		modifier = 1
	}

	s.transmissionMod = modifier
	s.modifierSet = true
}

// CurrentTransmissionModifier returns the effective modifier, defaulting to 1.0
// when the value has not been set.
func (s *Simulation) CurrentTransmissionModifier() float64 {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.modifierSet {
		return 1.0
	}

	return s.transmissionMod
}

// InfectionProbability applies the modifier to the base transmission rate and
// returns a capped probability used in the simulation loop.
func (s *Simulation) InfectionProbability() float64 {
	modifier := s.CurrentTransmissionModifier()
	probability := s.baseTransmission * modifier
	return math.Min(probability, 1.0)
}

// StepPair simulates the chance that one agent infects another during a step.
func (s *Simulation) StepPair() bool {
	chance := s.InfectionProbability()
	return s.rng.Float64() < chance
}

// Run executes a simple loop that repeatedly samples infection events and
// forwards the computed probability back to the caller for monitoring.
func (s *Simulation) Run(ctx context.Context, interval time.Duration, report func(probability, modifier float64)) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			prob := s.InfectionProbability()
			modifier := s.CurrentTransmissionModifier()
			if report != nil {
				report(prob, modifier)
			}
			log.Printf("simulation step: modifier=%.2f probability=%.3f", modifier, prob)
		}
	}
}
