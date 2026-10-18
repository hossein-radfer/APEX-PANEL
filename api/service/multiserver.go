package service

import (
	"gorm.io/gorm"

	"github.com/maahdima/mwp/api/dataservice/model"
)

// ForEachActiveServer runs fn once per active (model.Server.IsActive)
// server row, continuing past any individual failure rather than stopping
// at the first one -- a problem reaching ONE router must never hide data
// from every other registered server, matching this codebase's established
// "one dead source never blocks the rest" convention (see SyncPeersAllServers,
// SecurityIPCollectorService.Collect, ReportsLiveService.GetOnlineUsers for
// existing examples of the same shape). Returns every server's error keyed
// by its own ID, nil for a server that succeeded; an empty (non-nil) map
// means every server succeeded.
//
// A server with IsActive=false is skipped here deliberately -- "inactive"
// means "don't poll/change this server automatically," not "pretend it
// doesn't exist"; callers that need every server regardless of IsActive
// (e.g. GetServersData's own admin-facing counts) query model.Server
// directly instead of using this helper.
func ForEachActiveServer(db *gorm.DB, fn func(server model.Server) error) map[uint]error {
	var servers []model.Server
	db.Where("is_active = ?", true).Find(&servers)

	errs := make(map[uint]error, len(servers))
	for _, srv := range servers {
		errs[srv.ID] = fn(srv)
	}
	return errs
}
