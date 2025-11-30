package sim

import (
	"context"
	"log"
	"math"
	"math/rand"
	"sync"
	"time"
)

const defaultBaseDeathRate = 0.01

// Snapshot captures the current state of the simulation at a single point in
// time.
type Snapshot struct {
	TransmissionModifier        float64
	InfectionProbability        float64
	LockdownEnabled             bool
	HospitalCapacity            int
	DeathRateOverloadMultiplier float64
	CurrentInfected             int
	EffectiveDeathProbability   float64
	Overloaded                  bool
}

// Simulation tracks transmission probabilities and exposes knobs to adjust the
// spread model.
type Simulation struct {
	mu                          sync.RWMutex
	transmissionMod             float64
	modifierSet                 bool
	baseTransmission            float64
	baseDeathRate               float64
	hospitalCapacity            int
	deathRateOverloadMultiplier float64
	currentInfected             int
	rng                         *rand.Rand
	lockdownEnabled             bool
}

// New creates a simulation with the provided base transmission probability.
// If baseTransmission is zero, a default of 0.25 is used.
func New(baseTransmission float64) *Simulation {
	if baseTransmission <= 0 {
		baseTransmission = 0.25
	}
	SetCurrentSpeedModifier(1.0)
	return &Simulation{
		transmissionMod:             1.0,
		modifierSet:                 false,
		baseTransmission:            baseTransmission,
		baseDeathRate:               defaultBaseDeathRate,
		hospitalCapacity:            50,
		deathRateOverloadMultiplier: 2.0,
		currentInfected:             10,
		rng:                         rand.New(rand.NewSource(time.Now().UnixNano())),
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
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.infectionProbabilityLocked()
}

// StepPair simulates the chance that one agent infects another during a step.
func (s *Simulation) StepPair() bool {
	chance := s.InfectionProbability()
	return s.rng.Float64() < chance
}

// Run executes a simple loop that repeatedly samples infection events and
// forwards the computed probability back to the caller for monitoring.
func (s *Simulation) Run(ctx context.Context, interval time.Duration, report func(state Snapshot)) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.stepEpidemic()
			state := s.Snapshot()
			if report != nil {
				report(state)
			}
			log.Printf(
				"simulation step: modifier=%.2f probability=%.3f infected=%d overloaded=%t death_prob=%.3f",
				state.TransmissionModifier,
				state.InfectionProbability,
				state.CurrentInfected,
				state.Overloaded,
				state.EffectiveDeathProbability,
			)
		}
	}
}

// SetHospitalCapacity configures the maximum number of concurrent infections
// that can be treated. Non-positive values disable overload effects.
func (s *Simulation) SetHospitalCapacity(capacity int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if capacity < 0 {
		capacity = 0
	}
	s.hospitalCapacity = capacity
}

// HospitalCapacity returns the configured capacity.
func (s *Simulation) HospitalCapacity() int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.hospitalCapacity
}

// SetDeathRateOverloadMultiplier adjusts the scale factor applied to the death
// probability when capacity is exceeded.
func (s *Simulation) SetDeathRateOverloadMultiplier(multiplier float64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if multiplier < 1 {
		multiplier = 1
	}
	s.deathRateOverloadMultiplier = multiplier
}

// DeathRateOverloadMultiplier returns the overload multiplier.
func (s *Simulation) DeathRateOverloadMultiplier() float64 {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.deathRateOverloadMultiplier
}

// CurrentInfected returns the current infected count tracked by the simulation.
func (s *Simulation) CurrentInfected() int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.currentInfected
}

// Overloaded reports whether current infections exceed hospital capacity.
func (s *Simulation) Overloaded() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	_, overloaded := s.deathProbabilityLocked()
	return overloaded
}

// EffectiveDeathProbability returns the per-tick death probability after
// considering overload conditions.
func (s *Simulation) EffectiveDeathProbability() float64 {
	s.mu.RLock()
	defer s.mu.RUnlock()

	prob, _ := s.deathProbabilityLocked()
	return prob
}

// Snapshot returns a read-only copy of the simulation state.
func (s *Simulation) Snapshot() Snapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()

	deathProb, overloaded := s.deathProbabilityLocked()
	return Snapshot{
		TransmissionModifier:        s.currentTransmissionModifierLocked(),
		InfectionProbability:        s.infectionProbabilityLocked(),
		LockdownEnabled:             s.lockdownEnabled,
		HospitalCapacity:            s.hospitalCapacity,
		DeathRateOverloadMultiplier: s.deathRateOverloadMultiplier,
		CurrentInfected:             s.currentInfected,
		EffectiveDeathProbability:   deathProb,
		Overloaded:                  overloaded,
	}
}

func (s *Simulation) infectionProbabilityLocked() float64 {
	modifier := s.currentTransmissionModifierLocked()
	probability := s.baseTransmission * modifier
	return math.Min(probability, 1.0)
}

func (s *Simulation) currentTransmissionModifierLocked() float64 {
	if !s.modifierSet {
		return 1.0
	}
	return s.transmissionMod
}

func (s *Simulation) deathProbabilityLocked() (float64, bool) {
	overloaded := s.hospitalCapacity > 0 && s.currentInfected > s.hospitalCapacity
	probability := s.baseDeathRate
	if overloaded {
		probability *= s.deathRateOverloadMultiplier
	}
	probability = math.Max(probability, 0)
	probability = math.Min(probability, 1.0)
	return probability, overloaded
}

func (s *Simulation) stepEpidemic() {
	s.mu.Lock()
	defer s.mu.Unlock()

	infectionProbability := s.infectionProbabilityLocked()
	interactions := 5 + s.currentInfected/3
	newInfections := 0
	for i := 0; i < interactions; i++ {
		if s.rng.Float64() < infectionProbability {
			newInfections++
		}
	}

	s.currentInfected += newInfections

	deathProbability, _ := s.deathProbabilityLocked()
	deaths := 0
	for i := 0; i < s.currentInfected; i++ {
		if s.rng.Float64() < deathProbability {
			deaths++
		}
	}

	s.currentInfected -= deaths
	if s.currentInfected < 0 {
		s.currentInfected = 0
	}
}
