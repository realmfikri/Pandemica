package sim

import (
	"context"
	"testing"
	"time"
)

func TestInfectionProbabilityDefaults(t *testing.T) {
	s := New(0)
	if got := s.CurrentTransmissionModifier(); got != 1.0 {
		t.Fatalf("expected default modifier 1.0, got %v", got)
	}

	expected := 0.25
	if prob := s.InfectionProbability(); prob != expected {
		t.Fatalf("expected probability %v, got %v", expected, prob)
	}
}

func TestModifierApplied(t *testing.T) {
	s := New(0.5)
	s.UpdateTransmissionModifier(0.4)

	if got := s.CurrentTransmissionModifier(); got != 0.4 {
		t.Fatalf("expected modifier 0.4, got %v", got)
	}

	expected := 0.2
	if prob := s.InfectionProbability(); prob != expected {
		t.Fatalf("expected probability %v, got %v", expected, prob)
	}
}

func TestZeroModifierAllowed(t *testing.T) {
	s := New(0.5)
	s.UpdateTransmissionModifier(0)

	if got := s.CurrentTransmissionModifier(); got != 0 {
		t.Fatalf("expected modifier to be set to 0, got %v", got)
	}

	if prob := s.InfectionProbability(); prob != 0 {
		t.Fatalf("expected probability 0 when modifier is zero, got %v", prob)
	}
}

func TestRunReports(t *testing.T) {
	s := New(0.2)
	s.UpdateTransmissionModifier(0.5)

	ctx, cancel := context.WithCancel(context.Background())
	reported := make(chan Snapshot, 1)

	go s.Run(ctx, 10*time.Millisecond, func(state Snapshot) {
		reported <- state
		cancel()
	})

	select {
	case state := <-reported:
		if state.InfectionProbability <= 0 {
			t.Fatalf("expected probability to be greater than zero, got %v", state.InfectionProbability)
		}
		if state.CurrentInfected <= 0 {
			t.Fatalf("expected infected count to be tracked, got %v", state.CurrentInfected)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timed out waiting for report")
	}
}

func TestOverloadBoostsDeathProbability(t *testing.T) {
	s := New(0.2)
	s.SetHospitalCapacity(2)
	s.SetDeathRateOverloadMultiplier(3)
	s.currentInfected = 5

	prob := s.EffectiveDeathProbability()
	expected := defaultBaseDeathRate * 3
	if prob != expected {
		t.Fatalf("expected overloaded death probability %v, got %v", expected, prob)
	}

	if !s.Overloaded() {
		t.Fatal("expected simulation to be overloaded")
	}
}

func TestLockdownTogglesSpeedModifier(t *testing.T) {
	s := New(0.2)
	t.Cleanup(func() {
		SetCurrentSpeedModifier(1.0)
	})

	s.SetLockdown(true)
	if !s.LockdownEnabled() {
		t.Fatal("expected lockdown to be enabled")
	}
	if got := SpeedModifier(); got != 0.1 {
		t.Fatalf("expected speed modifier to drop to 0.1, got %v", got)
	}

	s.SetLockdown(false)
	if s.LockdownEnabled() {
		t.Fatal("expected lockdown to be disabled")
	}
	if got := SpeedModifier(); got != 1.0 {
		t.Fatalf("expected speed modifier to reset to 1.0, got %v", got)
	}
}

func TestApplyControlSettings(t *testing.T) {
	s := New(0.3)
	t.Cleanup(func() {
		SetCurrentSpeedModifier(1.0)
	})

	snapshot := s.ApplyControlSettings(ControlSettings{
		TransmissionModifier:        0.75,
		LockdownEnabled:             true,
		HospitalCapacity:            -5,
		DeathRateOverloadMultiplier: 0.5,
	})

	if snapshot.TransmissionModifier != 0.75 {
		t.Fatalf("expected transmission modifier 0.75, got %v", snapshot.TransmissionModifier)
	}
	if !snapshot.LockdownEnabled {
		t.Fatalf("expected lockdown to be enabled")
	}
	if snapshot.HospitalCapacity != 0 {
		t.Fatalf("expected negative capacity to clamp to 0, got %v", snapshot.HospitalCapacity)
	}
	if snapshot.DeathRateOverloadMultiplier != 1 {
		t.Fatalf("expected overload multiplier to clamp to 1, got %v", snapshot.DeathRateOverloadMultiplier)
	}
	if SpeedModifier() != 0.1 {
		t.Fatalf("expected lockdown to adjust speed modifier to 0.1, got %v", SpeedModifier())
	}
}
