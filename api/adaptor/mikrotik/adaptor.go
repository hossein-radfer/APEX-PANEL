package mikrotik

import (
	"go.uber.org/zap"

	"github.com/maahdima/mwp/api/common"
	"github.com/maahdima/mwp/api/utils/httphelper"
)

type Adaptor struct {
	mwpClients *common.MwpClients
	logger     *zap.Logger
}

// NewAdaptor creates a new instance of the Mikrotik adaptor
func NewAdaptor(mwpClients *common.MwpClients) *Adaptor {
	return &Adaptor{
		mwpClients: mwpClients,
		logger:     zap.L().Named("MikrotikAdaptor"),
	}
}

// ForServer returns a view of this adaptor scoped to one specific Mikrotik
// server -- every method in this package that used to call
// a.mwpClients.GetClient(nil) (silently picking "whichever client the map
// happens to return first," correct only because this codebase used to
// support exactly one server) now hangs off *ServerAdaptor instead, so a
// caller must always name which server it means. Cheap to call repeatedly
// (it only wraps two pointers); callers typically derive serverID from the
// Peer/Interface/UserManagerAccount row they're already operating on,
// which all now carry their own ServerID column.
func (a *Adaptor) ForServer(serverID uint) *ServerAdaptor {
	return &ServerAdaptor{adaptor: a, serverID: serverID}
}

// ServerAdaptor is *Adaptor narrowed to one server -- see Adaptor.ForServer.
type ServerAdaptor struct {
	adaptor  *Adaptor
	serverID uint
}

func (s *ServerAdaptor) client() *httphelper.Client {
	return s.adaptor.mwpClients.GetClient(&s.serverID)
}
