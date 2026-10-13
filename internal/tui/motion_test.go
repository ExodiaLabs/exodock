package tui

import "testing"

func TestStartupMotionBudgetAndDisplayPreferences(t *testing.T) {
	for _, mode := range []string{"animated", "reduced-motion", "no-color"} {
		t.Run(mode, func(t *testing.T) {
			m := visualModel(t, "dark", false, mode == "no-color")
			m.opts.ReducedMotion = mode == "reduced-motion"
			m.Init()
			if mode == "animated" {
				if m.startupFrames < 1 || m.startupFrames > 9 {
					t.Fatal("startup animation exceeds nine 100ms frames")
				}
			} else if m.startupFrames != 0 || m.motionActive {
				t.Fatal("display preference still starts animation")
			}
		})
	}
}

func TestMotionSchedulesOnlyWhileVisibleWorkIsActive(t *testing.T) {
	m := visualModel(t, "dark", false, false)
	if cmd := m.scheduleMotion(); cmd != nil || m.motionActive {
		t.Fatal("idle workspace scheduled animation work")
	}
	m.refreshing = true
	if cmd := m.scheduleMotion(); cmd == nil || !m.motionActive {
		t.Fatal("pending Docker work does not schedule loading feedback")
	}
	if cmd := m.scheduleMotion(); cmd != nil {
		t.Fatal("loading feedback scheduled duplicate ticks")
	}
	m.refreshing = false
	_, cmd := m.Update(motionMsg{generation: m.motionGeneration})
	if cmd != nil || m.motionActive {
		t.Fatal("animation kept ticking after work completed")
	}
}

func TestStartupMotionEndsWithoutIdleTickLoop(t *testing.T) {
	m := visualModel(t, "dark", false, false)
	m.startupFrames = 6
	if m.scheduleMotion() == nil {
		t.Fatal("startup motion did not begin")
	}
	for n := 0; n < 6; n++ {
		_, cmd := m.Update(motionMsg{generation: m.motionGeneration})
		if n < 5 && cmd == nil {
			t.Fatal("startup sequence ended before its bounded frames")
		}
		if n == 5 && cmd != nil {
			t.Fatal("startup sequence scheduled an idle animation tick")
		}
	}
	if m.startupFrames != 0 || m.motionActive {
		t.Fatal("startup sequence has no bounded completion")
	}
}

func TestStaleMotionTicksAndCancellationCannotRestartAnimation(t *testing.T) {
	m := visualModel(t, "dark", false, false)
	m.startupFrames = 6
	m.scheduleMotion()
	frame, remaining, generation := m.animationFrame, m.startupFrames, m.motionGeneration
	m.Update(motionMsg{generation: generation - 1})
	if m.animationFrame != frame || m.startupFrames != remaining || m.motionGeneration != generation {
		t.Fatal("stale animation tick changed current motion")
	}
	m.cancel()
	_, cmd := m.Update(motionMsg{generation: generation})
	if cmd != nil || m.scheduleMotion() != nil {
		t.Fatal("cancelled application rescheduled motion")
	}
}

func TestReducedMotionAndNoColorSuppressAnimation(t *testing.T) {
	for _, mode := range []string{"reduced-motion", "no-color"} {
		t.Run(mode, func(t *testing.T) {
			m := visualModel(t, "dark", false, mode == "no-color")
			m.opts.ReducedMotion = mode == "reduced-motion"
			m.startupFrames, m.refreshing = 6, true
			if m.scheduleMotion() != nil || m.motionActive {
				t.Fatal("display preference still schedules animated feedback")
			}
		})
	}
}
