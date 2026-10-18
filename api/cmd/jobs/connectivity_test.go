package traffic

import (
	"context"
	"errors"
	"testing"

	"github.com/maahdima/mwp/api/adaptor/mikrotik"
	"github.com/maahdima/mwp/api/dataservice/model"
)

// unreachableAdaptor simulates a Mikrotik connection that always fails, for
// testing the CRITICAL outage-alert transition detection.
type unreachableAdaptor struct {
	fakeAdaptor
}

func (u *unreachableAdaptor) FetchWgPeers(ctx context.Context) ([]mikrotik.WireGuardPeer, error) {
	return nil, errors.New("connection refused")
}

// ForServer overrides fakeAdaptor's own embedded ForServer so it returns
// THIS unreachableAdaptor (preserving the FetchWgPeers override above),
// not the embedded fakeAdaptor's reachable behavior.
func (u *unreachableAdaptor) ForServer(serverID uint) MikrotikAdaptor {
	return u
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
	// openTestDB shares one literal in-memory SQLite DB across every test in
	// this package -- a unique server Name per test avoids the real
	// uniqueIndex on servers.name colliding across tests.
	if err := db.Create(&model.Server{Name: "test-server-outage-transition", IPAddress: "10.255.0.1", IsActive: true}).Error; err != nil {
		t.Fatalf("failed to seed server: %v", err)
	}
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
	if err := db.Create(&model.Server{Name: "test-server-recovery", IPAddress: "10.255.0.2", IsActive: true}).Error; err != nil {
		t.Fatalf("failed to seed server: %v", err)
	}
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
	if err := db.Create(&model.Server{Name: "test-server-noop", IPAddress: "10.255.0.3", IsActive: true}).Error; err != nil {
		t.Fatalf("failed to seed server: %v", err)
	}
	calc := NewTrafficCalculator(db, &unreachableAdaptor{}, nil)
	// No SetCriticalAlertNotifier call -- must not panic.
	calc.checkConnectivity()
}
