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
	reported := make(chan float64, 1)

	go s.Run(ctx, 10*time.Millisecond, func(prob, modifier float64) {
		reported <- prob
		cancel()
	})

	select {
	case val := <-reported:
		if val <= 0 {
			t.Fatalf("expected probability to be greater than zero, got %v", val)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timed out waiting for report")
	}
}
