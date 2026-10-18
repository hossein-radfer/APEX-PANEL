package mikrotik

import (
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/maahdima/mwp/api/common"
	"github.com/maahdima/mwp/api/http/schema"
)

// testServerID is an arbitrary fixed ID used to register the fake router's
// client in these tests -- MwpClients is keyed by server ID, not name (see
// MwpClients.SetClient's own doc comment), so every test here registers and
// looks up by this same constant.
const testServerID = uint(1)

// newTestAdaptorWithFakeRouter builds a *ServerAdaptor (scoped to
// testServerID) whose client points at a local httptest server -- mirrors
// how a real model.Server row/common.MwpClients.SetClient pairing works in
// production, just without a real router or database.
func newTestAdaptorWithFakeRouter(t *testing.T, handler http.HandlerFunc) *ServerAdaptor {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	parsed, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("failed to parse test server URL: %v", err)
	}
	_, portStr, err := net.SplitHostPort(parsed.Host)
	if err != nil {
		t.Fatalf("failed to split host/port: %v", err)
	}

	mwpClients := common.NewMwpClients(nil)
	isSSL := false
	mwpClients.SetClient(testServerID, &schema.CreateServerRequest{
		Name:      "test-server",
		IPAddress: "127.0.0.1",
		APIPort:   portStr,
		IsSSL:     &isSSL,
		Username:  "admin",
		Password:  "admin",
	})

	return NewAdaptor(mwpClients).ForServer(testServerID)
}

func TestFetchAllContainers_ReturnsListFromRouter(t *testing.T) {
	adaptor := newTestAdaptorWithFakeRouter(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rest/container" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]Container{
			{ID: "*1", Name: "xui-panel-1", Status: "running", Image: "alireza0/x-ui"},
			{ID: "*2", Name: "xui-panel-2", Status: "stopped", Image: "alireza0/x-ui"},
		})
	})

	containers, err := adaptor.FetchAllContainers(t.Context())
	if err != nil {
		t.Fatalf("FetchAllContainers failed: %v", err)
	}
	if len(containers) != 2 {
		t.Fatalf("expected 2 containers, got %d", len(containers))
	}
	if containers[0].Name != "xui-panel-1" || containers[0].Status != "running" {
		t.Errorf("unexpected first container: %+v", containers[0])
	}
	if containers[1].Name != "xui-panel-2" || containers[1].Status != "stopped" {
		t.Errorf("unexpected second container: %+v", containers[1])
	}
}

func TestFetchContainerByName_FindsMatchingContainer(t *testing.T) {
	adaptor := newTestAdaptorWithFakeRouter(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]Container{
			{ID: "*1", Name: "xui-panel-1", Status: "running"},
			{ID: "*2", Name: "xui-panel-2", Status: "stopped"},
		})
	})

	container, err := adaptor.FetchContainerByName(t.Context(), "xui-panel-2")
	if err != nil {
		t.Fatalf("FetchContainerByName failed: %v", err)
	}
	if container.Status != "stopped" {
		t.Errorf("expected status stopped, got %q", container.Status)
	}
}

func TestFetchContainerByName_ReturnsErrorWhenNotFound(t *testing.T) {
	adaptor := newTestAdaptorWithFakeRouter(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]Container{
			{ID: "*1", Name: "some-other-container", Status: "running"},
		})
	})

	_, err := adaptor.FetchContainerByName(t.Context(), "xui-panel-missing")
	if err == nil {
		t.Fatal("expected an error for a container name that doesn't exist on the router")
	}
}

func TestFetchAllContainers_UnknownServerReturnsError(t *testing.T) {
	mwpClients := common.NewMwpClients(nil)
	adaptor := NewAdaptor(mwpClients).ForServer(uint(9999))

	_, err := adaptor.FetchAllContainers(t.Context())
	if err == nil {
		t.Fatal("expected an error when no client is registered for the given server id")
	}
}
