package traffic

import (
	"context"
	"errors"
	"testing"

	"github.com/maahdima/mwp/api/adaptor/mikrotik"
)

// unreachableAdaptor simulates a Mikrotik connection that always fails, for
// testing the CRITICAL outage-alert transition detection.
type unreachableAdaptor struct {
	fakeAdaptor
}

func (u *unreachableAdaptor) FetchWgPeers(ctx context.Context) ([]mikrotik.WireGuardPeer, error) {
	return nil, errors.New("connection refused")
}

// spyCriticalAlertNotifier records every NotifyCriticalAlert call so tests
// can assert exactly how many times (and with what message) an alert fired.
type spyCriticalAlertNotifier struct {
	messages []string
}

func (s *spyCriticalAlertNotifier) NotifyCriticalAlert(message string) {
	s.messages = append(s.messages, message)
}

func TestCheckConnectivity_AlertsOnceOnOutageTransition(t *testing.T) {
	db := openTestDB(t)
	calc := NewTrafficCalculator(db, &unreachableAdaptor{}, nil)
	spy := &spyCriticalAlertNotifier{}
	calc.SetCriticalAlertNotifier(spy)

	// First tick: transitions from (assumed) reachable to unreachable --
	// must alert exactly once.
	calc.checkConnectivity()
	if len(spy.messages) != 1 {
		t.Fatalf("expected exactly 1 alert after first outage tick, got %d: %v", len(spy.messages), spy.messages)
	}

	// Second and third ticks: still unreachable, must NOT alert again --
	// this is the whole point of transition-based (not level-based) alerting.
	calc.checkConnectivity()
	calc.checkConnectivity()
	if len(spy.messages) != 1 {
		t.Fatalf("expected still exactly 1 alert after repeated outage ticks (no spam), got %d: %v", len(spy.messages), spy.messages)
	}
}

func TestCheckConnectivity_AlertsOnRecovery(t *testing.T) {
	db := openTestDB(t)
	calc := NewTrafficCalculator(db, &unreachableAdaptor{}, nil)
	spy := &spyCriticalAlertNotifier{}
	calc.SetCriticalAlertNotifier(spy)

	calc.checkConnectivity() // outage begins
	if len(spy.messages) != 1 {
		t.Fatalf("expected 1 alert after outage tick, got %d", len(spy.messages))
	}

	// Swap to a reachable adaptor and tick again -- must fire a recovery
	// alert exactly once.
	calc.mikrotikAdaptor = &fakeAdaptor{}
	calc.checkConnectivity()
	if len(spy.messages) != 2 {
		t.Fatalf("expected 2 alerts total after recovery tick, got %d: %v", len(spy.messages), spy.messages)
	}

	// Ticking again while still reachable must not alert a third time.
	calc.checkConnectivity()
	if len(spy.messages) != 2 {
		t.Fatalf("expected still 2 alerts after a stable-reachable tick, got %d: %v", len(spy.messages), spy.messages)
	}
}

func TestCheckConnectivity_NoOpWithoutNotifier(t *testing.T) {
	db := openTestDB(t)
	calc := NewTrafficCalculator(db, &unreachableAdaptor{}, nil)
	// No SetCriticalAlertNotifier call -- must not panic.
	calc.checkConnectivity()
}
