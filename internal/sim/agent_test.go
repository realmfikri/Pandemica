package sim

import "testing"

func TestAgentStepUsesSpeedModifier(t *testing.T) {
	t.Cleanup(func() {
		SetCurrentSpeedModifier(1.0)
	})

	SetCurrentSpeedModifier(0.5)
	agent := Agent{BaseSpeed: 2, DirectionX: 1, DirectionY: 0}
	agent.Step(1.0)

	if agent.X != 1.0 {
		t.Fatalf("expected X to advance by 1.0, got %v", agent.X)
	}
	if agent.Y != 0 {
		t.Fatalf("expected Y to remain unchanged, got %v", agent.Y)
	}
}
