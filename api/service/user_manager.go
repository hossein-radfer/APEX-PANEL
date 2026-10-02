package service

import (
	"context"
	"errors"
	"fmt"
	mathrand "math/rand"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/maahdima/mwp/api/adaptor/mikrotik"
	"github.com/maahdima/mwp/api/common"
	"github.com/maahdima/mwp/api/dataservice/model"
	"github.com/maahdima/mwp/api/http/schema"
	"github.com/maahdima/mwp/api/utils"
)

// UserManagerService manages RouterOS User Manager accounts (L2TP/PPTP/
// SSTP/OpenVPN) -- a fully independent subsystem from WgPeer/WireGuard,
// mirroring its permission/quota/scoping conventions but never sharing a
// table, a quota pool, or a reseller-suspension path with it.
type UserManagerService struct {
	db              *gorm.DB
	mikrotikAdaptor *mikrotik.Adaptor
	auditLog        *AuditLog
	botNotifier     *BotNotifier
	configFile      *UserManagerConfigFile
	licenseLimiter  freeTierLimiter
	logger          *zap.Logger
}

func NewUserManagerService(db *gorm.DB, mikrotikAdaptor *mikrotik.Adaptor, auditLog *AuditLog) *UserManagerService {
	return &UserManagerService{
		db:              db,
		mikrotikAdaptor: mikrotikAdaptor,
		auditLog:        auditLog,
		logger:          zap.L().Named("UserManagerService"),
	}
}

// SetBotNotifier wires the Telegram notifier after construction, mirroring
// WgPeer.SetBotNotifier -- safe to leave unset.
func (s *UserManagerService) SetBotNotifier(notifier *BotNotifier) {
	s.botNotifier = notifier
}

// SetLicenseLimiter wires the free-tier cap lookup in (فاز ۴-۱۲) -- see
// freeTierLimiter's own doc comment (reseller.go). Safe to leave unset.
func (s *UserManagerService) SetLicenseLimiter(limiter freeTierLimiter) {
	s.licenseLimiter = limiter
}

// ErrFreeTierUserManagerLimitReached mirrors ErrFreeTierPeerLimitReached
// exactly, for the "max_user_manager_accounts" free-tier cap.
var ErrFreeTierUserManagerLimitReached = errors.New("this license has expired and is running in restricted mode: the user manager account limit for the free tier has been reached")

// SuspendOldestForFreeTier mirrors WgPeer.SuspendOldestForFreeTier exactly
// (see its own doc comment for the full DB-only, RouterOS-call-deferred
// rationale) -- the oldest excess UserManagerAccount rows (by CreatedAt)
// beyond cap are force-disabled at the DB level via the same
// Disabled/SuspendedByQuota/WasActiveBeforeSuspend flags
// resumeQuotaSuspendedUserManagerAccounts already understands, and the
// periodic traffic-sync job reconciles the actual RouterOS account state on
// its own next tick.
func (s *UserManagerService) SuspendOldestForFreeTier(cap int64) (int, error) {
	if cap <= 0 {
		return 0, nil
	}

	var activeCount int64
	if err := s.db.Model(&model.UserManagerAccount{}).Where("disabled = ?", false).Count(&activeCount).Error; err != nil {
		return 0, err
	}

	excess := activeCount - cap
	if excess <= 0 {
		return 0, nil
	}

	var toSuspend []model.UserManagerAccount
	if err := s.db.Where("disabled = ?", false).Order("created_at ASC").Limit(int(excess)).Find(&toSuspend).Error; err != nil {
		return 0, err
	}

	suspended := 0
	for _, acct := range toSuspend {
		if err := s.db.Model(&model.UserManagerAccount{}).Where("id = ?", acct.ID).Updates(map[string]interface{}{
			"disabled":                  true,
			"suspended_by_quota":        true,
			"was_active_before_suspend": true,
		}).Error; err != nil {
			s.logger.Error("failed to suspend user manager account for free-tier cap", zap.Uint("account_id", acct.ID), zap.Error(err))
			continue
		}
		suspended++
	}

	return suspended, nil
}

// SetConfigFileService wires the admin-uploaded config file (OpenVPN
// .ovpn profile) storage after construction, same deferred-wiring reason
// as SetBotNotifier. Safe to leave unset -- DeleteAccount simply skips
// config file cleanup if nil.
func (s *UserManagerService) SetConfigFileService(configFile *UserManagerConfigFile) {
	s.configFile = configFile
}

// logResellerAction mirrors WgPeer.logResellerAction exactly.
func (s *UserManagerService) logResellerAction(resellerID *uint, action, description string) {
	if resellerID == nil {
		return
	}

	var reseller model.Reseller
	name := "unknown"
	if err := s.db.Select("name").First(&reseller, *resellerID).Error; err == nil {
		name = reseller.Name
	}

	if s.auditLog != nil {
		s.auditLog.Log(*resellerID, name, action, description)
	}

	if action == AuditActionUserManagerAccountCreated && s.botNotifier != nil {
		s.botNotifier.NotifyLiveLog(fmt.Sprintf("%s: %s", name, description))
	}
}

// ensureResellerCanCreateUserManagerAccounts mirrors
// WgPeer.isInterfaceAssignedToReseller's role (a permission gate), but
// reads a plain boolean column instead of a join table, since User Manager
// is one shared RouterOS resource with no per-reseller sub-resource to
// assign.
func (s *UserManagerService) ensureResellerCanCreateUserManagerAccounts(resellerID uint) error {
	var reseller model.Reseller
	if err := s.db.First(&reseller, resellerID).Error; err != nil {
		return err
	}
	if !reseller.CanCreateUserManagerAccounts {
		return fmt.Errorf("reseller is not permitted to create user manager accounts")
	}
	return nil
}

// ensureResellerUnderUserManagerAccountLimit mirrors
// WgPeer.ensureResellerUnderPeerLimit exactly, swapping in the User
// Manager-specific fields/model.
func (s *UserManagerService) ensureResellerUnderUserManagerAccountLimit(resellerID uint) error {
	var reseller model.Reseller
	if err := s.db.First(&reseller, resellerID).Error; err != nil {
		return err
	}
	if reseller.UserManagerMaxAccounts == nil {
		return nil
	}

	var count int64
	if err := s.db.Model(&model.UserManagerAccount{}).Where("reseller_id = ?", resellerID).Count(&count).Error; err != nil {
		return err
	}
	if int(count) >= *reseller.UserManagerMaxAccounts {
		return fmt.Errorf("reseller has reached its maximum allowed user manager accounts (%d)", *reseller.UserManagerMaxAccounts)
	}
	return nil
}

// ensureResellerCanUseGroup mirrors WgPeer.isInterfaceAssignedToReseller's
// "must be explicitly assigned, no empty-means-all fallback" contract, but
// against ResellerUserManagerGroup instead of ResellerInterface.
func (s *UserManagerService) ensureResellerCanUseGroup(resellerID uint, group string) error {
	var count int64
	if err := s.db.Model(&model.ResellerUserManagerGroup{}).
		Where("reseller_id = ? AND group_name = ?", resellerID, group).
		Count(&count).Error; err != nil {
		return err
	}
	if count == 0 {
		return fmt.Errorf("reseller is not permitted to use group %q", group)
	}
	return nil
}

// ensureResellerCanUseProfile mirrors ensureResellerCanUseGroup exactly,
// for RouterOS User Manager profiles.
func (s *UserManagerService) ensureResellerCanUseProfile(resellerID uint, profile string) error {
	var count int64
	if err := s.db.Model(&model.ResellerUserManagerProfile{}).
		Where("reseller_id = ? AND profile_name = ?", resellerID, profile).
		Count(&count).Error; err != nil {
		return err
	}
	if count == 0 {
		return fmt.Errorf("reseller is not permitted to use profile %q", profile)
	}
	return nil
}

func (s *UserManagerService) ensureUsernameIsUnique(username string) error {
	var existing model.UserManagerAccount
	if err := s.db.Where("username = ?", username).First(&existing).Error; err == nil {
		return fmt.Errorf("username %s is already in use", username)
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		s.logger.Error("username lookup failed", zap.Error(err))
		return err
	}
	return nil
}

// getAccountByIDScoped mirrors WgPeer.getPeerByIDScoped exactly.
func (s *UserManagerService) getAccountByIDScoped(id uint, resellerID *uint) (model.UserManagerAccount, error) {
	var account model.UserManagerAccount
	query := s.db.Where("id = ?", id)
	if resellerID != nil {
		query = query.Where("reseller_id = ?", *resellerID)
	}
	if err := query.First(&account).Error; err != nil {
		return model.UserManagerAccount{}, err
	}

	return account, nil
}

// EnsureAccountAccess mirrors WgPeer.EnsurePeerAccess, used by the share
// endpoints to confirm ownership without fetching the full response DTO.
func (s *UserManagerService) EnsureAccountAccess(id uint, resellerID *uint) error {
	_, err := s.getAccountByIDScoped(id, resellerID)
	return err
}

// CreateAccount creates a User Manager account, both on RouterOS and in the
// local database. resellerID nil means an admin-owned account.
//
// On a failure after the RouterOS user has already been created, no
// compensating rollback is issued -- this deliberately matches
// WgPeer.CreatePeer's existing behavior (which also leaves a partially
// created Mikrotik object behind if a later step fails), for consistency
// rather than introducing new asymmetric error-handling behavior.
// randomAccountCredential generates a short, pronounceable random string
// for a bulk-created account's username/password -- reuses the exact same
// adjective/noun word lists as V2RayPackageService.generateBulkPackageLabel
// (bulkLabelAdjectives/bulkLabelNouns, package-level vars already shared
// across this package) so all three bulk-creation flows (V2Ray packages,
// WireGuard peers, User Manager accounts) produce visually-consistent
// random names. A 3-digit random suffix (in addition to the batch index
// already making each call's inputs distinct) keeps two different bulk
// batches from ever colliding, since ensureUsernameIsUnique only rejects
// an EXACT duplicate rather than retrying with a new name itself.
func randomAccountCredential(index int) string {
	adjective := bulkLabelAdjectives[mathrand.Intn(len(bulkLabelAdjectives))]
	noun := bulkLabelNouns[mathrand.Intn(len(bulkLabelNouns))]
	return fmt.Sprintf("%s%s%d%03d", adjective, noun, index+1, mathrand.Intn(1000))
}

// ensureResellerUnderUserManagerAccountLimitForBatch mirrors
// ensureResellerUnderUserManagerAccountLimit but checks against a whole
// upcoming batch of `additional` accounts at once, rather than one -- used
// by BulkCreateAccounts so a reseller's limit is checked UP FRONT against
// the full requested count (matching V2RayPackageService.
// BulkCreatePackages' "whole batch or nothing" contract), instead of only
// being caught by CreateAccount's own per-call check partway through the
// loop.
func (s *UserManagerService) ensureResellerUnderUserManagerAccountLimitForBatch(resellerID uint, additional int) error {
	var reseller model.Reseller
	if err := s.db.First(&reseller, resellerID).Error; err != nil {
		return err
	}
	if reseller.UserManagerMaxAccounts == nil {
		return nil
	}

	var count int64
	if err := s.db.Model(&model.UserManagerAccount{}).Where("reseller_id = ?", resellerID).Count(&count).Error; err != nil {
		return err
	}
	if int(count)+additional > *reseller.UserManagerMaxAccounts {
		return fmt.Errorf("creating %d accounts would exceed the reseller's maximum allowed user manager accounts (%d, %d already exist)", additional, *reseller.UserManagerMaxAccounts, count)
	}
	return nil
}

// BulkCreateAccounts creates req.Count accounts sharing the same group/
// profile/protocol/traffic-limit/duration selection, each via a plain call
// to CreateAccount below (reusing its existing RouterOS creation, quota,
// and group/profile-access checks verbatim) -- mirrors
// V2RayPackageService.BulkCreatePackages' exact structure, adapted for
// User Manager's random-username-per-account identity (see
// randomAccountCredential) in place of a package's single shared
// CustomerLabel-only identity.
func (s *UserManagerService) BulkCreateAccounts(req *schema.BulkCreateUserManagerAccountRequest, resellerID *uint) ([]schema.BulkCreatedUserManagerAccount, error) {
	if resellerID != nil {
		if err := s.ensureResellerCanCreateUserManagerAccounts(*resellerID); err != nil {
			return nil, err
		}
		if err := s.ensureResellerUnderUserManagerAccountLimitForBatch(*resellerID, req.Count); err != nil {
			return nil, err
		}
	}

	expireTime := time.Now().UTC().AddDate(0, 0, req.DurationDays).Format("2006-01-02")

	results := make([]schema.BulkCreatedUserManagerAccount, 0, req.Count)
	for i := 0; i < req.Count; i++ {
		username := randomAccountCredential(i)
		password := randomAccountCredential(i)

		account, err := s.CreateAccount(&schema.CreateUserManagerAccountRequest{
			Username:     username,
			Password:     password,
			Group:        req.Group,
			Profile:      req.Profile,
			Protocols:    req.Protocols,
			SharedUsers:  req.SharedUsers,
			TrafficLimit: req.TrafficLimit,
			ExpireTime:   &expireTime,
		}, resellerID)
		if err != nil {
			s.logger.Error("bulk create failed partway through -- returning accounts created so far", zap.Int("created", len(results)), zap.Int("requested", req.Count), zap.Error(err))
			return results, fmt.Errorf("failed after creating %d of %d accounts: %w", len(results), req.Count, err)
		}

		// Sharing is off by default on every account -- a bulk-created batch
		// exists specifically to be exported with its share links, so
		// turning sharing on here makes the exported link resolve to a
		// working share page. Mirrors WgPeer.BulkCreatePeers/
		// V2RayPackageService.BulkCreatePackages' identical step.
		if updateErr := s.db.Model(&model.UserManagerAccount{}).Where("id = ?", account.Id).Update("is_shared", true).Error; updateErr != nil {
			s.logger.Warn("failed to enable sharing on bulk-created account, its share link will not resolve until sharing is turned on manually", zap.Uint("account_id", account.Id), zap.Error(updateErr))
		} else {
			account.IsShared = true
		}

		results = append(results, schema.BulkCreatedUserManagerAccount{
			UserManagerAccountResponse: *account,
			Password:                   password,
		})
	}

	return results, nil
}

func (s *UserManagerService) CreateAccount(req *schema.CreateUserManagerAccountRequest, resellerID *uint) (*schema.UserManagerAccountResponse, error) {
	if s.licenseLimiter != nil {
		if maxAccounts, ok := s.licenseLimiter.GetFreeTierLimit("max_user_manager_accounts"); ok {
			var activeCount int64
			if err := s.db.Model(&model.UserManagerAccount{}).Count(&activeCount).Error; err != nil {
				return nil, err
			}
			if activeCount >= maxAccounts {
				return nil, ErrFreeTierUserManagerLimitReached
			}
		}
	}

	if resellerID != nil {
		if err := s.ensureResellerCanCreateUserManagerAccounts(*resellerID); err != nil {
			return nil, err
		}
		if err := s.ensureResellerUnderUserManagerAccountLimit(*resellerID); err != nil {
			return nil, err
		}
		if err := s.ensureResellerCanUseGroup(*resellerID, req.Group); err != nil {
			return nil, err
		}
		if err := s.ensureResellerCanUseProfile(*resellerID, req.Profile); err != nil {
			return nil, err
		}
	}

	if err := s.ensureUsernameIsUnique(req.Username); err != nil {
		return nil, err
	}

	sharedUsers := 1
	if req.SharedUsers != nil && *req.SharedUsers > 0 {
		sharedUsers = *req.SharedUsers
	}
	sharedUsersStr := fmt.Sprintf("%d", sharedUsers)

	ctx := context.Background()

	password := req.Password
	createdUser, err := s.mikrotikAdaptor.CreateUserManagerUser(ctx, mikrotik.UserManagerUser{
		Name:        req.Username,
		Password:    &password,
		SharedUsers: &sharedUsersStr,
	})
	if err != nil {
		s.logger.Error("failed to create user manager user on mikrotik", zap.Error(err))
		return nil, fmt.Errorf("failed to create user manager account: %w", err)
	}

	if _, err := s.mikrotikAdaptor.SetUserManagerUserGroup(ctx, createdUser.ID, req.Group); err != nil {
		s.logger.Error("failed to set user manager user group", zap.Error(err))
		return nil, fmt.Errorf("failed to assign group: %w", err)
	}

	profileLink, err := s.mikrotikAdaptor.CreateUserManagerUserProfile(ctx, mikrotik.UserManagerUserProfile{
		User:    req.Username,
		Profile: req.Profile,
	})
	if err != nil {
		s.logger.Error("failed to bind user manager profile", zap.Error(err))
		return nil, fmt.Errorf("failed to bind profile: %w", err)
	}

	if err := s.mikrotikAdaptor.ActivateUserManagerUserProfile(ctx, profileLink.ID); err != nil {
		s.logger.Error("failed to activate user manager profile", zap.Error(err))
		return nil, fmt.Errorf("failed to activate profile: %w", err)
	}

	var trafficLimit *int64
	if req.TrafficLimit != nil {
		bytes := utils.GBToBytes(*req.TrafficLimit)
		if bytes > 0 {
			trafficLimit = &bytes
		}
	}

	protocols := make([]model.UserManagerAccountProtocol, len(req.Protocols))
	for i, p := range req.Protocols {
		protocols[i] = model.UserManagerAccountProtocol(p)
	}

	account := model.UserManagerAccount{
		UUID:                  uuid.New().String(),
		Username:              req.Username,
		Password:              req.Password,
		RouterOSUserID:        createdUser.ID,
		RouterOSProfileLinkID: &profileLink.ID,
		Group:                 req.Group,
		Profile:               req.Profile,
		Protocols:             model.JoinProtocols(protocols),
		Comment:               req.Comment,
		SharedUsers:           sharedUsers,
		ResellerID:            resellerID,
		TrafficLimit:          trafficLimit,
		ExpireTime:            req.ExpireTime,
	}

	if err := s.db.Create(&account).Error; err != nil {
		s.logger.Error("failed to store user manager account in database", zap.Error(err))

		// A confirmed, reported bug: if this DB write fails for ANY reason
		// (this exact bug was a stale, since-fixed NOT NULL column, but any
		// other DB failure has the identical effect), the RouterOS user
		// created above at line ~215 was already left behind on the
		// router with no corresponding panel row -- so a retry with the
		// same username then failed a SECOND time, this time against
		// RouterOS itself, with "username already exists," and the admin
		// had no way to recover except manually removing the orphaned
		// user in Winbox/WebFig. Best-effort rollback here (mirrors
		// DeleteAccount's own "log a Warn, don't fail the outer
		// operation" pattern for the profile-link half) removes both the
		// profile-link relation and the RouterOS user itself, so the
		// username is immediately available again for a retry.
		if err := s.mikrotikAdaptor.DeleteUserManagerUserProfile(ctx, profileLink.ID); err != nil {
			s.logger.Warn("failed to roll back user manager profile link after a failed database write",
				zap.String("username", req.Username), zap.Error(err))
		}
		if err := s.mikrotikAdaptor.DeleteUserManagerUser(ctx, createdUser.ID); err != nil {
			s.logger.Warn("failed to roll back user manager user on mikrotik after a failed database write -- it may need manual removal",
				zap.String("username", req.Username), zap.Error(err))
		}

		return nil, fmt.Errorf("failed to store user manager account: %w", err)
	}

	s.seedUsageBaseline(ctx, createdUser.ID, account.Username)

	s.logResellerAction(resellerID, AuditActionUserManagerAccountCreated, fmt.Sprintf("Created user manager account %q (%s)", account.Username, account.Protocols))

	// A freshly created account cannot have an active PPP session yet.
	resp := s.transformAccountToResponse(account, false)
	return &resp, nil
}

// BulkImportAccounts syncs a batch of already-existing RouterOS User
// Manager accounts (created by an external script, not this panel) into
// the panel's own database, matched purely by username. This is
// deliberately NOT account creation: it never calls CreateUserManagerUser,
// never touches Group/Profile assignment on RouterOS, and never requires
// the reseller-group/profile permission checks CreateAccount enforces
// (ensureResellerCanUseGroup/ensureResellerCanUseProfile) -- those exist to
// gate what a reseller may CHOOSE when creating a new account, which is
// irrelevant here since the account and its group/profile already exist,
// chosen by whatever external process created them.
//
// Algorithm (see the exact requirements this was designed against):
//  1. One bulk fetch of /user-manager/user/print and one of
//     /user-manager/user-profile/print -- never one RouterOS call per
//     imported line, regardless of batch size.
//  2. Build an in-memory map keyed by username for O(1) lookup per line.
//  3. For each parsed TXT line, look up the username in the map. Group
//     comes directly off the matched UserManagerUser; Profile comes from
//     the matched UserManagerUserProfile link (if any).
//  4. TrafficLimit/ExpireTime always come from the TXT file, never from
//     any pre-existing state -- a fresh import always overwrites the
//     panel's own bookkeeping for that account with exactly what the file
//     says, ignoring whatever limit a previous import or manual edit set.
//     These are recorded ONLY in the panel's own database -- RouterOS User
//     Manager's own limitation/profile attributes are a different
//     mechanism entirely and are never touched by this import. The
//     existing usage-monitoring cronjob (CalculateUserManagerUsage) is
//     what actually enforces the new limit, on its next scheduled tick,
//     by comparing real RouterOS usage against whatever TrafficLimit now
//     sits in the database -- this method does not pre-emptively check
//     usage or disable anything itself.
//  5. A username already present in the panel's database is updated in
//     place (password/Group/Profile/RouterOSUserID left untouched --
//     only TrafficLimit/ExpireTime are refreshed from the file); a
//     username not yet present is inserted fresh, with a newly generated
//     random password (RouterOS never exposes an existing account's
//     plaintext password -- see model.UserManagerAccount.Password's own
//     doc comment on why this field is required; the admin can set a real
//     known password afterwards via ChangeAccountPassword). A .id is used
//     transiently during this pass (for the one-time monitor baseline
//     seed below) but is never treated as this row's permanent identity --
//     Username is, and remains, the only key used to match future imports
//     back to this row.
//  6. A username in the TXT file with no matching RouterOS account is
//     skipped entirely (not created) and reported back by name, so the
//     admin can see exactly which lines didn't match anything.
//  7. For every account actually imported/updated, seed
//     LastTotalDownload/LastTotalUpload from one MonitorUserManagerUser
//     call each (baseline current cumulative RouterOS counters) --
//     otherwise the next CalculateUserManagerUsage tick would compute the
//     account's entire historical lifetime usage as a single "new" delta
//     against the freshly-imported limit, incorrectly tripping it
//     immediately even for an account that's actually well within quota.
func (s *UserManagerService) BulkImportAccounts(fileContent string, resellerID *uint) (*schema.BulkImportAccountsResult, error) {
	if resellerID != nil {
		if err := s.ensureResellerCanCreateUserManagerAccounts(*resellerID); err != nil {
			return nil, err
		}
	}

	lines, malformed := ParseBulkImportFile(fileContent)

	ctx := context.Background()

	routerUsers, err := s.mikrotikAdaptor.FetchUserManagerUsers(ctx)
	if err != nil {
		s.logger.Error("bulk import: failed to fetch user manager users", zap.Error(err))
		return nil, fmt.Errorf("failed to fetch accounts from mikrotik: %w", err)
	}
	profileLinks, err := s.mikrotikAdaptor.FetchUserManagerUserProfileLinks(ctx)
	if err != nil {
		s.logger.Error("bulk import: failed to fetch user manager user-profile links", zap.Error(err))
		return nil, fmt.Errorf("failed to fetch account profiles from mikrotik: %w", err)
	}

	usersByName := make(map[string]mikrotik.UserManagerUser, len(routerUsers))
	for _, u := range routerUsers {
		usersByName[u.Name] = u
	}
	profileByUsername := make(map[string]string, len(profileLinks))
	for _, link := range profileLinks {
		profileByUsername[link.User] = link.Profile
	}

	result := &schema.BulkImportAccountsResult{Malformed: malformed}

	for _, line := range lines {
		routerUser, found := usersByName[line.Username]
		if !found {
			result.Skipped++
			result.SkippedUsernames = append(result.SkippedUsernames, line.Username)
			continue
		}

		var trafficLimit *int64
		if line.TrafficLimitGB != nil {
			bytes := utils.GBToBytes(*line.TrafficLimitGB)
			if bytes > 0 {
				trafficLimit = &bytes
			}
		}
		var expireTime *string
		if line.ExpireDays != nil {
			expiry := time.Now().UTC().AddDate(0, 0, *line.ExpireDays).Format("2006-01-02")
			expireTime = &expiry
		}

		group := ""
		if routerUser.Group != nil {
			group = *routerUser.Group
		}
		profile := profileByUsername[line.Username]

		var existing model.UserManagerAccount
		err := s.db.Where("username = ?", line.Username).First(&existing).Error
		if err == nil {
			existing.TrafficLimit = trafficLimit
			existing.ExpireTime = expireTime
			if err := s.db.Save(&existing).Error; err != nil {
				s.logger.Error("bulk import: failed to update existing account", zap.String("username", line.Username), zap.Error(err))
				continue
			}
			result.AlreadyImported++
			s.seedUsageBaseline(ctx, routerUser.ID, existing.Username)
			continue
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			s.logger.Error("bulk import: username lookup failed", zap.String("username", line.Username), zap.Error(err))
			continue
		}

		account := model.UserManagerAccount{
			UUID:           uuid.New().String(),
			Username:       line.Username,
			Password:       utils.RandomString(12),
			RouterOSUserID: routerUser.ID,
			Group:          group,
			Profile:        profile,
			Protocols: model.JoinProtocols([]model.UserManagerAccountProtocol{
				model.ProtocolL2TP, model.ProtocolPPTP, model.ProtocolSSTP, model.ProtocolOpenVPN,
			}),
			SharedUsers:  1,
			ResellerID:   resellerID,
			TrafficLimit: trafficLimit,
			ExpireTime:   expireTime,
		}
		if err := s.db.Create(&account).Error; err != nil {
			s.logger.Error("bulk import: failed to store new account", zap.String("username", line.Username), zap.Error(err))
			continue
		}
		result.Imported++
		s.seedUsageBaseline(ctx, routerUser.ID, account.Username)
	}

	s.logResellerAction(resellerID, AuditActionUserManagerAccountCreated,
		fmt.Sprintf("Bulk import: %d imported, %d already imported, %d skipped, %d malformed",
			result.Imported, result.AlreadyImported, result.Skipped, len(result.Malformed)))

	return result, nil
}

// seedUsageBaseline seeds LastTotalDownload/LastTotalUpload for a
// just-created/imported/updated account from a single monitor call, so the
// next CalculateUserManagerUsage tick computes a correct delta instead of
// counting the account's entire pre-existing lifetime RouterOS usage as
// brand new. Called from both CreateAccount (a genuinely fresh RouterOS
// user should read back 0/0, but seeding it explicitly rather than
// trusting the DB column's zero-value default guards against the case
// where CreateUserManagerUser happens to reuse/rebind an existing RouterOS
// user with prior traffic) and BulkImportAccounts (which syncs
// already-existing accounts that may have arbitrary historical usage,
// where this seeding is essential, not just defensive). Best-effort: a
// failure here just means the account starts with a zero baseline (the
// pre-existing behavior before this seeding was added anywhere), logged
// but not fatal to the caller.
func (s *UserManagerService) seedUsageBaseline(ctx context.Context, routerOSUserID, username string) {
	result, err := s.mikrotikAdaptor.MonitorUserManagerUser(ctx, routerOSUserID)
	if err != nil {
		s.logger.Warn("failed to seed usage baseline, will start from zero", zap.String("username", username), zap.Error(err))
		return
	}

	download, downloadErr := utils.ParseRouterOSByteSize(result.TotalDownload)
	upload, uploadErr := utils.ParseRouterOSByteSize(result.TotalUpload)
	if downloadErr != nil || uploadErr != nil {
		s.logger.Warn("failed to parse usage baseline, will start from zero",
			zap.String("username", username), zap.Error(downloadErr), zap.Error(uploadErr))
		return
	}

	if err := s.db.Model(&model.UserManagerAccount{}).Where("username = ?", username).Updates(map[string]interface{}{
		"last_total_download": download,
		"last_total_upload":   upload,
	}).Error; err != nil {
		s.logger.Warn("failed to persist usage baseline", zap.String("username", username), zap.Error(err))
	}
}

// isAccountOnline does a single-account /ppp/active/print check, matching
// by username -- the same source GetAccountUsage/GetSelfSummary use. Only
// appropriate for single-record responses (Create/Update); listAccountsScoped
// fetches sessions once for the whole list instead of once per row.
func (s *UserManagerService) isAccountOnline(ctx context.Context, username string) bool {
	if s.mikrotikAdaptor == nil {
		return false
	}
	sessions, err := s.mikrotikAdaptor.FetchPPPActiveSessions(ctx)
	if err != nil {
		s.logger.Warn("failed to fetch ppp active sessions for online status", zap.String("username", username), zap.Error(err))
		return false
	}
	for _, session := range sessions {
		if session.Name == username {
			return true
		}
	}
	return false
}

// UpdateAccount updates an account's group/profile (re-synced to RouterOS
// if changed) and/or cosmetic DB-only fields.
func (s *UserManagerService) UpdateAccount(id uint, req *schema.UpdateUserManagerAccountRequest, resellerID *uint) (*schema.UserManagerAccountResponse, error) {
	account, err := s.getAccountByIDScoped(id, resellerID)
	if err != nil {
		return nil, err
	}

	ctx := context.Background()

	if req.Group != "" && req.Group != account.Group {
		if resellerID != nil {
			if err := s.ensureResellerCanUseGroup(*resellerID, req.Group); err != nil {
				return nil, err
			}
		}
		if _, err := s.mikrotikAdaptor.SetUserManagerUserGroup(ctx, account.RouterOSUserID, req.Group); err != nil {
			s.logger.Error("failed to update user manager user group", zap.Error(err))
			return nil, fmt.Errorf("failed to update group: %w", err)
		}
		account.Group = req.Group
	}

	if req.Profile != "" && req.Profile != account.Profile {
		if resellerID != nil {
			if err := s.ensureResellerCanUseProfile(*resellerID, req.Profile); err != nil {
				return nil, err
			}
		}
		newLink, err := s.relinkUserManagerProfile(ctx, &account, req.Profile)
		if err != nil {
			return nil, err
		}
		account.Profile = req.Profile
		account.RouterOSProfileLinkID = &newLink.ID
	} else if req.ExpireTime != nil && !equalExpireTime(req.ExpireTime, account.ExpireTime) {
		// A confirmed, reported bug: RouterOS User Manager profiles carry
		// their OWN validity window (e.g. a 30-day profile), started the
		// moment ActivateUserManagerUserProfile was first called -- entirely
		// separate from ExpireTime, which is this panel's own display-only
		// bookkeeping (see model.UserManagerAccount.ExpireTime's own doc
		// comment). Extending ExpireTime here used to just update that one
		// field with no RouterOS call at all, so RouterOS's own clock kept
		// running unaffected and could still lock the user out at the
		// ORIGINAL activation's expiry even though the panel displayed more
		// time remaining. Fix: extending the expiry re-provisions the SAME
		// profile (delete the old link, create+activate a fresh one) so
		// RouterOS's own validity window restarts too -- exactly the
		// sequence already used above when the profile itself changes,
		// just triggered by a time extension instead.
		newLink, err := s.relinkUserManagerProfile(ctx, &account, account.Profile)
		if err != nil {
			return nil, err
		}
		account.RouterOSProfileLinkID = &newLink.ID
	}

	if req.Disabled != nil && *req.Disabled != account.Disabled {
		disabledStr := "false"
		if *req.Disabled {
			disabledStr = "true"
		}
		if _, err := s.mikrotikAdaptor.SetUserManagerUserDisabled(ctx, account.RouterOSUserID, disabledStr); err != nil {
			s.logger.Error("failed to update user manager user disabled state", zap.Error(err))
			return nil, fmt.Errorf("failed to update status: %w", err)
		}
		account.Disabled = *req.Disabled
	}

	if req.Comment != nil {
		account.Comment = req.Comment
	}
	if req.SharedUsers != nil && *req.SharedUsers > 0 {
		account.SharedUsers = *req.SharedUsers
	}
	if req.TrafficLimit != nil {
		bytes := utils.GBToBytes(*req.TrafficLimit)
		if bytes > 0 {
			account.TrafficLimit = &bytes
		}
	}
	if req.ExpireTime != nil {
		account.ExpireTime = req.ExpireTime
	}
	if len(req.Protocols) > 0 {
		protocols := make([]model.UserManagerAccountProtocol, len(req.Protocols))
		for i, p := range req.Protocols {
			protocols[i] = model.UserManagerAccountProtocol(p)
		}
		account.Protocols = model.JoinProtocols(protocols)
	}

	if err := s.db.Save(&account).Error; err != nil {
		s.logger.Error("failed to update user manager account in database", zap.Error(err))
		return nil, fmt.Errorf("failed to update user manager account: %w", err)
	}

	resp := s.transformAccountToResponse(account, s.isAccountOnline(ctx, account.Username))
	return &resp, nil
}

// relinkUserManagerProfile removes account's current RouterOS profile-link
// relation (if any) and creates+activates a fresh one for the given
// profile name -- shared by UpdateAccount's own profile-change branch and
// its expiry-extension branch, since both need the exact same underlying
// RouterOS sequence (see UpdateAccount's own doc comments at each call
// site for why). Deleting the old link is best-effort (already-gone is a
// success from this call's point of view, same rationale as
// DeleteAccount's own profile-link cleanup); creating/activating the new
// one is not -- a failure there is returned to the caller.
func (s *UserManagerService) relinkUserManagerProfile(ctx context.Context, account *model.UserManagerAccount, profile string) (*mikrotik.UserManagerUserProfile, error) {
	if account.RouterOSProfileLinkID != nil {
		if err := s.mikrotikAdaptor.DeleteUserManagerUserProfile(ctx, *account.RouterOSProfileLinkID); err != nil {
			s.logger.Warn("failed to remove old user manager profile link during re-link", zap.Error(err))
		}
	}

	newLink, err := s.mikrotikAdaptor.CreateUserManagerUserProfile(ctx, mikrotik.UserManagerUserProfile{
		User:    account.Username,
		Profile: profile,
	})
	if err != nil {
		s.logger.Error("failed to bind new user manager profile", zap.Error(err))
		return nil, fmt.Errorf("failed to bind new profile: %w", err)
	}
	if err := s.mikrotikAdaptor.ActivateUserManagerUserProfile(ctx, newLink.ID); err != nil {
		s.logger.Error("failed to activate new user manager profile", zap.Error(err))
		return nil, fmt.Errorf("failed to activate new profile: %w", err)
	}

	return newLink, nil
}

// equalExpireTime reports whether two possibly-nil ExpireTime pointers
// represent the same value -- used to detect a genuine extension (as
// opposed to the request simply echoing back the account's current,
// unchanged expiry) before paying the cost of a RouterOS profile re-link.
func equalExpireTime(a, b *string) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

// ToggleAccountStatus flips an account's disabled state.
func (s *UserManagerService) ToggleAccountStatus(id uint, resellerID *uint) error {
	account, err := s.getAccountByIDScoped(id, resellerID)
	if err != nil {
		return fmt.Errorf("account not found: %w", err)
	}

	disabled := !account.Disabled
	disabledStr := "false"
	if disabled {
		disabledStr = "true"
	}

	if _, err := s.mikrotikAdaptor.SetUserManagerUserDisabled(context.Background(), account.RouterOSUserID, disabledStr); err != nil {
		s.logger.Error("failed to toggle user manager user status on mikrotik", zap.Error(err))
		return fmt.Errorf("failed to toggle user manager account status: %w", err)
	}

	if err := s.db.Model(&account).Update("disabled", disabled).Error; err != nil {
		s.logger.Error("failed to toggle user manager account status in database", zap.Error(err))
		return fmt.Errorf("failed to toggle user manager account status: %w", err)
	}

	return nil
}

// ChangeAccountPassword sets a new password for an existing account, both
// on RouterOS and in the local database (which stores the plaintext value
// so it can be redisplayed on the admin edit dialog and the public share
// page -- see model.UserManagerAccount.Password's doc comment). Used both
// by the admin-facing "Change Password" action and by BulkImportAccounts
// (which seeds a freshly-generated random password for accounts synced
// from RouterOS, since RouterOS never exposes an existing account's
// plaintext password).
func (s *UserManagerService) ChangeAccountPassword(id uint, newPassword string, resellerID *uint) error {
	account, err := s.getAccountByIDScoped(id, resellerID)
	if err != nil {
		return fmt.Errorf("account not found: %w", err)
	}

	if _, err := s.mikrotikAdaptor.SetUserManagerUserPassword(context.Background(), account.RouterOSUserID, newPassword); err != nil {
		s.logger.Error("failed to change user manager user password on mikrotik", zap.String("username", account.Username), zap.Error(err))
		return fmt.Errorf("failed to change password: %w", err)
	}

	if err := s.db.Model(&account).Update("password", newPassword).Error; err != nil {
		s.logger.Error("failed to update user manager account password in database", zap.String("username", account.Username), zap.Error(err))
		return fmt.Errorf("failed to change password: %w", err)
	}

	return nil
}

// DeleteAccount removes an account, both on RouterOS (profile-link first,
// then the user itself -- matching the operator-confirmed command order
// exactly) and in the local database.
func (s *UserManagerService) DeleteAccount(id uint, resellerID *uint) error {
	account, err := s.getAccountByIDScoped(id, resellerID)
	if err != nil {
		return fmt.Errorf("account not found: %w", err)
	}

	ctx := context.Background()

	// A confirmed, reported bug -- STILL reproducing even after an earlier
	// fix that guarded against an empty RouterOSProfileLinkID, so the root
	// cause is broader than just "empty string": RouterOS rejects this
	// delete with "missing or invalid resource identifier" not only for an
	// empty ID, but also for an ID that no longer resolves to a real
	// profile-link relation on the router (e.g. it was already removed by
	// a previous partial delete attempt, changed/cleaned up directly on
	// the router outside this panel, or any other RouterOS-side drift this
	// codebase has no way to predict in advance). Since this step's ENTIRE
	// purpose is cleanup of a relation that, in every one of these cases,
	// is already gone anyway, its failure must never block the admin's
	// actual intent (delete this account) -- logged as a warning and
	// treated as non-fatal, exactly like every other "one dead/stale
	// external reference never blocks the rest of the operation"
	// convention already established elsewhere in this codebase (see
	// V2RayPackageLocation's own doc comment for the same principle
	// applied to x-ui panel calls).
	if account.RouterOSProfileLinkID != nil && *account.RouterOSProfileLinkID != "" {
		if err := s.mikrotikAdaptor.DeleteUserManagerUserProfile(ctx, *account.RouterOSProfileLinkID); err != nil {
			s.logger.Warn("failed to delete user manager profile link from mikrotik -- proceeding to delete the account anyway, since this relation is either already gone or was never valid",
				zap.Uint("account_id", account.ID), zap.String("username", account.Username), zap.Error(err))
		}
	}

	// A confirmed, reported bug: deleting a User Manager account from the
	// panel AFTER it was already removed directly on RouterOS (outside the
	// panel) failed here with RouterOS's own 404 "no such item" -- treated
	// as fatal, so the DB row was never cleaned up and the admin was stuck
	// with a phantom account they could never delete from the panel again.
	// Mirrors WgPeer's own identical fix for the same scenario (see
	// isMikrotikNotFoundError's doc comment): a 404 here means the intended
	// end state (this user gone from RouterOS) is already true, so it's
	// tolerated exactly like the profile-link cleanup above, while any
	// OTHER error (a genuine connectivity/permission failure) still blocks
	// the delete as before.
	if err := s.mikrotikAdaptor.DeleteUserManagerUser(ctx, account.RouterOSUserID); err != nil && !isMikrotikNotFoundError(err) {
		s.logger.Error("failed to delete user manager user from mikrotik", zap.Error(err))
		return fmt.Errorf("failed to delete user manager account: %w", err)
	}

	// A confirmed, reported bug: UserManagerUsedBytes is a LIVE SUM
	// recomputed every quota tick over currently-existing
	// UserManagerAccount rows (see applyUserManagerResellerQuota's own doc
	// comment) -- so hard-deleting an account used to silently erase its
	// historical usage from the owning reseller's total and every
	// report/invoice built on it. Folding this account's own usage into
	// Reseller.UserManagerDeletedUsageBytes BEFORE the delete preserves it:
	// the live-sum calculation adds this credit back on top of the SUM
	// over still-existing rows, so deleting an account can only ever
	// affect which accounts remain, never how much this reseller has
	// billably used to date.
	if account.ResellerID != nil {
		deletedUsage := account.DownloadUsage + account.UploadUsage
		if deletedUsage > 0 {
			if err := s.db.Model(&model.Reseller{}).Where("id = ?", *account.ResellerID).
				Update("user_manager_deleted_usage_bytes", gorm.Expr("user_manager_deleted_usage_bytes + ?", deletedUsage)).Error; err != nil {
				s.logger.Error("failed to credit deleted account's usage to reseller total", zap.Uint("reseller_id", *account.ResellerID), zap.Error(err))
				return fmt.Errorf("failed to preserve deleted account's usage: %w", err)
			}
		}
	}

	if err := s.db.Unscoped().Delete(&account).Error; err != nil {
		s.logger.Error("failed to delete user manager account from database", zap.Error(err))
		return fmt.Errorf("failed to delete user manager account from database: %w", err)
	}

	if account.HasConfigFile && s.configFile != nil {
		s.configFile.RemoveConfig(account.UUID)
	}

	s.logResellerAction(resellerID, AuditActionUserManagerAccountDeleted, fmt.Sprintf("Deleted user manager account %q (%s)", account.Username, account.Protocols))

	return nil
}

// ResetUsage zeroes this account's DISPLAYED DownloadUsage/UploadUsage --
// mirrors WgPeer/traffic.Calculator's "Reset Usage" button for WireGuard
// peers, adapted for User Manager's RouterOS-side counter having no
// confirmed reset call of its own (see model.UserManagerAccount.
// UsageOffsetDownload/UsageOffsetUpload's own doc comment for the full
// mechanism: this bumps the offset up to the CURRENT raw RouterOS total,
// which the next traffic-poll tick then subtracts back out to read 0,
// without ever touching RouterOS itself, LastTotalDownload/LastTotalUpload,
// or the reseller-level UserManagerQuotaBytes pool -- exactly the admin's
// own explicit requirement that a reset must never move reseller/global
// quota totals).
// ResetUsage zeroes one account's DISPLAYED usage (via the
// UsageOffset*/RouterOS-cumulative-counter mechanism -- see
// UsageOffsetDownload/UsageOffsetUpload's own doc comment) -- and, per
// the confirmed reported bug "ریست حجم کار نمی‌کند" (usage reset doesn't
// work, also affecting User Manager accounts), also clears a
// TrafficLimit-triggered disable: previously this only ever zeroed the
// usage counters, so an account processUserManagerAccountUsage had
// already disabled for exceeding its old limit stayed disabled on
// RouterOS forever, with usage now reading 0 but the account still
// unable to authenticate -- indistinguishable from "the reset didn't do
// anything" from the admin's point of view. Only re-enables when
// SuspendedByTrafficLimit is true (that job's own doing); an account an
// admin disabled manually for an unrelated reason is left exactly as
// they set it.
func (s *UserManagerService) ResetUsage(id uint, resellerID *uint) error {
	account, err := s.getAccountByIDScoped(id, resellerID)
	if err != nil {
		return fmt.Errorf("account not found: %w", err)
	}

	wasSuspendedByTrafficLimit := account.SuspendedByTrafficLimit
	if wasSuspendedByTrafficLimit {
		if _, err := s.mikrotikAdaptor.SetUserManagerUserDisabled(context.Background(), account.RouterOSUserID, "false"); err != nil {
			s.logger.Error("failed to re-enable user manager account on mikrotik after usage reset", zap.Uint("account_id", account.ID), zap.Error(err))
			return fmt.Errorf("failed to re-enable account: %w", err)
		}
	}

	updates := map[string]interface{}{
		"download_usage":        0,
		"upload_usage":          0,
		"usage_offset_download": account.LastTotalDownload,
		"usage_offset_upload":   account.LastTotalUpload,
		"first_notify":          false,
		"second_notify":         false,
		"third_notify":          false,
	}
	if wasSuspendedByTrafficLimit {
		updates["disabled"] = false
		updates["suspended_by_traffic_limit"] = false
	}
	if err := s.db.Model(&model.UserManagerAccount{}).Where("id = ?", account.ID).Updates(updates).Error; err != nil {
		s.logger.Error("failed to reset user manager account usage", zap.Uint("account_id", account.ID), zap.Error(err))
		return fmt.Errorf("failed to reset account usage: %w", err)
	}

	return nil
}

// BulkDeleteAccounts deletes each given account via the exact same
// DeleteAccount path one at a time -- mirrors WgPeer.BulkDeletePeers: no
// batch RouterOS call exists, and one account's failure (e.g. a stale
// profile-link reference) must never block deleting the rest. Used by the
// "expired/quota-exhausted" bulk cleanup action.
func (s *UserManagerService) BulkDeleteAccounts(ids []uint, resellerID *uint) (deleted []uint, failed map[uint]string) {
	failed = make(map[uint]string)
	for _, id := range ids {
		if err := s.DeleteAccount(id, resellerID); err != nil {
			failed[id] = err.Error()
			continue
		}
		deleted = append(deleted, id)
	}
	return deleted, failed
}

// ListAccounts returns accounts scoped to the caller -- nil resellerID
// (admin) returns only admin-owned accounts (reseller_id IS NULL); a
// non-nil resellerID returns that reseller's own accounts. Mirrors
// WgPeer.GetPeers/getPeersScoped's exact scoping convention.
func (s *UserManagerService) ListAccounts(resellerID *uint) (*[]schema.UserManagerAccountResponse, error) {
	return s.listAccountsScoped(resellerID)
}

// ListAccountsByReseller returns all accounts owned by a specific reseller
// -- used by the admin-facing "Reseller User Manager" page.
func (s *UserManagerService) ListAccountsByReseller(resellerID uint) (*[]schema.UserManagerAccountResponse, error) {
	return s.listAccountsScoped(&resellerID)
}

func (s *UserManagerService) listAccountsScoped(resellerID *uint) (*[]schema.UserManagerAccountResponse, error) {
	var accounts []model.UserManagerAccount
	query := s.db.Model(&model.UserManagerAccount{}).Order("id DESC")
	if resellerID != nil {
		query = query.Where("reseller_id = ?", *resellerID)
	} else {
		query = query.Where("reseller_id IS NULL")
	}
	if err := query.Find(&accounts).Error; err != nil {
		s.logger.Error("failed to fetch user manager accounts from database", zap.Error(err))
		return nil, fmt.Errorf("failed to fetch user manager accounts: %w", err)
	}

	// One bulk /ppp/active/print call for the whole list, not one per
	// account -- mirrors GetSelfSummary's pattern. Best-effort: a RouterOS
	// hiccup (or, in tests, no adaptor configured at all) degrades to
	// "nobody shown online" rather than failing the list.
	onlineUsernames := make(map[string]struct{})
	if s.mikrotikAdaptor != nil {
		if sessions, err := s.mikrotikAdaptor.FetchPPPActiveSessions(context.Background()); err != nil {
			s.logger.Warn("failed to fetch ppp active sessions for account list", zap.Error(err))
		} else {
			for _, session := range sessions {
				onlineUsernames[session.Name] = struct{}{}
			}
		}
	}

	responses := make([]schema.UserManagerAccountResponse, 0, len(accounts))
	for _, account := range accounts {
		_, isOnline := onlineUsernames[account.Username]
		responses = append(responses, s.transformAccountToResponse(account, isOnline))
	}

	return &responses, nil
}

// GetAccountUsage resolves online status (via /ppp/active/print, matching
// by username) and current usage (via /user-manager/user/monitor, parsed
// with utils.ParseRouterOSByteSize) for a single account. Uses the cached
// RouterOSUserID on the DB row rather than re-resolving it via
// /user-manager/user/print on every call, minimizing RouterOS round-trips.
func (s *UserManagerService) GetAccountUsage(id uint, resellerID *uint) (downloadBytes, uploadBytes int64, isOnline bool, err error) {
	account, err := s.getAccountByIDScoped(id, resellerID)
	if err != nil {
		return 0, 0, false, err
	}

	ctx := context.Background()

	sessions, err := s.mikrotikAdaptor.FetchPPPActiveSessions(ctx)
	if err != nil {
		s.logger.Error("failed to fetch ppp active sessions", zap.Error(err))
	} else {
		for _, session := range sessions {
			if session.Name == account.Username {
				isOnline = true
				break
			}
		}
	}

	result, err := s.mikrotikAdaptor.MonitorUserManagerUser(ctx, account.RouterOSUserID)
	if err != nil {
		s.logger.Error("failed to monitor user manager user", zap.Error(err))
		return 0, 0, isOnline, fmt.Errorf("failed to fetch usage: %w", err)
	}

	downloadBytes, err = utils.ParseRouterOSByteSize(result.TotalDownload)
	if err != nil {
		s.logger.Warn("failed to parse user manager download usage, leaving previous value", zap.String("raw", result.TotalDownload), zap.Error(err))
		downloadBytes = account.LastTotalDownload
	}

	uploadBytes, err = utils.ParseRouterOSByteSize(result.TotalUpload)
	if err != nil {
		s.logger.Warn("failed to parse user manager upload usage, leaving previous value", zap.String("raw", result.TotalUpload), zap.Error(err))
		uploadBytes = account.LastTotalUpload
	}

	return downloadBytes, uploadBytes, isOnline, nil
}

// GetSelfSummary aggregates a reseller's own User Manager accounts for
// their dashboard -- online count, total accounts, and the reseller-level
// quota/used/remaining/max fields. Mirrors WgPeer.GetSelfActivity's
// shape/pattern (peer.go): one bulk RouterOS call
// (FetchPPPActiveSessions) rather than one call per account, matched
// against the reseller's own accounts by username.
func (s *UserManagerService) GetSelfSummary(resellerID uint) (*schema.UserManagerSelfSummaryResponse, error) {
	var reseller model.Reseller
	if err := s.db.First(&reseller, resellerID).Error; err != nil {
		s.logger.Error("failed to fetch reseller for user manager summary", zap.Uint("resellerID", resellerID), zap.Error(err))
		return nil, err
	}

	var accounts []model.UserManagerAccount
	if err := s.db.Where("reseller_id = ?", resellerID).Find(&accounts).Error; err != nil {
		s.logger.Error("failed to fetch user manager accounts for summary", zap.Uint("resellerID", resellerID), zap.Error(err))
		return nil, err
	}

	usernames := make(map[string]struct{}, len(accounts))
	for _, a := range accounts {
		usernames[a.Username] = struct{}{}
	}

	online := 0
	if sessions, err := s.mikrotikAdaptor.FetchPPPActiveSessions(context.Background()); err != nil {
		// Best-effort, matching GetAccountShareDetails/GetAccountUsage's
		// precedent -- a RouterOS hiccup should degrade to "0 online
		// shown" rather than fail the whole dashboard summary.
		s.logger.Warn("failed to fetch ppp active sessions for user manager summary", zap.Uint("resellerID", resellerID), zap.Error(err))
	} else {
		for _, session := range sessions {
			if _, ok := usernames[session.Name]; ok {
				online++
			}
		}
	}

	// usedBytes is a LIVE SUM of these same accounts' own DisplayedUsage,
	// not the reseller's stored UserManagerUsedBytes column -- see
	// traffic.Calculator.applyUserManagerResellerQuota's own doc comment
	// for the confirmed drift bug this avoids (that stored column can lag
	// behind the true sum until the next traffic-job tick recomputes it;
	// reading it live here means the dashboard is never stale even
	// immediately after, e.g., a bulk import).
	var usedBytes int64
	for _, a := range accounts {
		usedBytes += a.DownloadUsage + a.UploadUsage
	}

	var remaining *int64
	if reseller.UserManagerQuotaBytes != nil {
		r := *reseller.UserManagerQuotaBytes - usedBytes
		if r < 0 {
			r = 0
		}
		remaining = &r
	}

	return &schema.UserManagerSelfSummaryResponse{
		OnlineAccounts: online,
		TotalAccounts:  len(accounts),
		QuotaBytes:     reseller.UserManagerQuotaBytes,
		UsedBytes:      usedBytes,
		RemainingBytes: remaining,
		MaxAccounts:    reseller.UserManagerMaxAccounts,
	}, nil
}

// buildUserManagerProtocolInfos builds one connection-info block per
// protocol account was actually granted -- a customer sold "OpenVPN only"
// must never see L2TP/PPTP/SSTP details, even though the shared
// credential would technically also authenticate there (see
// model.UserManagerAccount's doc comment). Extracted out of
// GetAccountShareDetails so ApplicationService.GetConnectConfigs (the
// mobile app's own connect-configs endpoint) can build the exact same
// per-protocol shape without duplicating the UserManagerProtocolConfig
// lookup loop.
func buildUserManagerProtocolInfos(db *gorm.DB, logger *zap.Logger, account model.UserManagerAccount) ([]schema.UserManagerShareProtocolInfo, error) {
	accountProtocols := model.ParseProtocols(account.Protocols)
	if len(accountProtocols) == 0 {
		return nil, fmt.Errorf("account has no protocols configured")
	}

	var defaultServerAddress string
	var server model.Server
	if err := db.First(&server).Error; err == nil {
		defaultServerAddress = server.IPAddress
	}

	protocolInfos := make([]schema.UserManagerShareProtocolInfo, 0, len(accountProtocols))
	for _, protocol := range accountProtocols {
		var protocolConfig model.UserManagerProtocolConfig
		if err := db.Where("protocol = ?", protocol).First(&protocolConfig).Error; err != nil {
			logger.Warn("skipping unconfigured protocol", zap.String("protocol", string(protocol)), zap.Error(err))
			continue
		}

		serverAddress := utils.DerefString(protocolConfig.ServerAddress)
		if serverAddress == "" {
			serverAddress = defaultServerAddress
		}

		protocolInfos = append(protocolInfos, schema.UserManagerShareProtocolInfo{
			Protocol:           string(protocol),
			ServerAddress:      serverAddress,
			Port:               protocolConfig.Port,
			HasCertificateFile: protocolConfig.HasCertificateFile,
			HasClientAppFile:   protocolConfig.HasClientAppFile,
			Notes:              protocolConfig.Notes,
		})
	}
	return protocolInfos, nil
}

// GetAccountShareDetails is the public, unauthenticated lookup behind the
// connection-info share page.
func (s *UserManagerService) GetAccountShareDetails(uuidStr string) (*schema.UserManagerAccountShareDetailsResponse, error) {
	var account model.UserManagerAccount
	if err := s.db.First(&account, "uuid = ?", uuidStr).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			s.logger.Error("user manager account not found in database", zap.String("uuid", uuidStr))
			return nil, err
		}
		s.logger.Error("failed to find user manager account in database", zap.Error(err))
		return nil, err
	}

	if !utils.IsPeerSharable(account.IsShared, account.ShareExpireTime) {
		return nil, common.ErrUserManagerAccountNotShared
	}

	protocolInfos, err := buildUserManagerProtocolInfos(s.db, s.logger, account)
	if err != nil {
		return nil, err
	}

	// Usage figures come from the DB-cached counters the traffic job
	// (CalculateUserManagerUsage) maintains, NOT a live Mikrotik round-trip
	// -- this is a public, unauthenticated endpoint and its core job
	// (showing quota/usage) must not depend on RouterOS being reachable at
	// the exact moment a customer opens their share link.
	totalUsageBytes := account.DownloadUsage + account.UploadUsage

	var usagePercent, trafficLimit *string
	if account.TrafficLimit != nil && *account.TrafficLimit > 0 {
		trafficLimit = utils.Ptr(utils.BytesToGB(*account.TrafficLimit))
		percent := float64(totalUsageBytes) / float64(*account.TrafficLimit) * 100
		usagePercent = utils.Ptr(fmt.Sprintf("%.1f", percent))
	}

	// Online status is best-effort here (unlike GetPeerDetails, which
	// requires it): a Mikrotik outage must not take down the whole share
	// page, only degrade the "online" badge to "unknown/offline".
	isOnline := false
	if sessions, err := s.mikrotikAdaptor.FetchPPPActiveSessions(context.Background()); err != nil {
		s.logger.Warn("failed to fetch ppp active sessions for share page online status", zap.String("uuid", uuidStr), zap.Error(err))
	} else {
		for _, session := range sessions {
			if session.Name == account.Username {
				isOnline = true
				break
			}
		}
	}

	return &schema.UserManagerAccountShareDetailsResponse{
		Username:      account.Username,
		Password:      account.Password,
		Protocols:     protocolInfos,
		ExpireTime:    account.ExpireTime,
		HasConfigFile: account.HasConfigFile,
		TrafficLimit:  trafficLimit,
		DownloadUsage: utils.BytesToGB(account.DownloadUsage),
		UploadUsage:   utils.BytesToGB(account.UploadUsage),
		TotalUsage:    utils.BytesToGB(totalUsageBytes),
		UsagePercent:  usagePercent,
		IsOnline:      isOnline,
	}, nil
}

// EnsureProtocolForSharedAccount validates that uuid belongs to a currently
// shared account (same IsShared/ShareExpireTime gate as
// GetAccountShareDetails) AND that requestedProtocol is one of the
// protocols this specific account was granted, then returns it typed --
// used to authorize the public per-protocol certificate/client-app file
// downloads without requiring a second, separate credential (knowing a
// valid share link already proves the caller is an intended recipient), while
// still preventing a customer sold only e.g. OpenVPN from downloading the
// L2TP certificate by guessing the query param.
func (s *UserManagerService) EnsureProtocolForSharedAccount(uuidStr string, requestedProtocol string) (model.UserManagerAccountProtocol, error) {
	var account model.UserManagerAccount
	if err := s.db.First(&account, "uuid = ?", uuidStr).Error; err != nil {
		return "", err
	}

	if !utils.IsPeerSharable(account.IsShared, account.ShareExpireTime) {
		return "", common.ErrUserManagerAccountNotShared
	}

	for _, p := range model.ParseProtocols(account.Protocols) {
		if string(p) == requestedProtocol {
			return p, nil
		}
	}
	return "", gorm.ErrRecordNotFound
}

// GetAccountShareStatus/UpdateAccountShareStatus/UpdateAccountShareExpire
// mirror WgPeer's peer-share endpoints exactly, scoped to a UserManagerAccount.
func (s *UserManagerService) GetAccountShareStatus(id uint, resellerID *uint) (*schema.UserManagerAccountShareStatusResponse, error) {
	account, err := s.getAccountByIDScoped(id, resellerID)
	if err != nil {
		return nil, err
	}

	var uuidPtr *string
	if account.IsShared {
		uuidPtr = &account.UUID
	}

	return &schema.UserManagerAccountShareStatusResponse{
		IsShared:   account.IsShared,
		UUID:       uuidPtr,
		ExpireTime: account.ShareExpireTime,
	}, nil
}

func (s *UserManagerService) UpdateAccountShareStatus(id uint, resellerID *uint) error {
	account, err := s.getAccountByIDScoped(id, resellerID)
	if err != nil {
		return fmt.Errorf("account not found: %w", err)
	}

	if err := s.db.Model(&account).Update("is_shared", !account.IsShared).Error; err != nil {
		s.logger.Error("failed to update user manager account share status", zap.Error(err))
		return fmt.Errorf("failed to update share status: %w", err)
	}

	return nil
}

func (s *UserManagerService) UpdateAccountShareExpire(id uint, expireTime *string, resellerID *uint) error {
	account, err := s.getAccountByIDScoped(id, resellerID)
	if err != nil {
		return fmt.Errorf("account not found: %w", err)
	}

	if !account.IsShared {
		return fmt.Errorf("user manager account is not shared, cannot set expire time")
	}

	if err := s.db.Model(&account).Update("share_expire_time", expireTime).Error; err != nil {
		s.logger.Error("failed to update user manager account share expire time", zap.Error(err))
		return fmt.Errorf("failed to update share expire time: %w", err)
	}

	return nil
}

// ListGroups/ListProfiles are thin RouterOS passthroughs -- no local
// caching, since RouterOS is the source of truth and groups/profiles are
// only listed by this panel, never created/edited here.
func (s *UserManagerService) ListGroups() ([]schema.UserManagerGroupResponse, error) {
	groups, err := s.mikrotikAdaptor.FetchUserManagerUserGroups(context.Background())
	if err != nil {
		return nil, err
	}

	resp := make([]schema.UserManagerGroupResponse, 0, len(groups))
	for _, g := range groups {
		resp = append(resp, schema.UserManagerGroupResponse{Name: g.Name})
	}
	return resp, nil
}

func (s *UserManagerService) ListProfiles() ([]schema.UserManagerProfileResponse, error) {
	profiles, err := s.mikrotikAdaptor.FetchUserManagerProfiles(context.Background())
	if err != nil {
		return nil, err
	}

	resp := make([]schema.UserManagerProfileResponse, 0, len(profiles))
	for _, p := range profiles {
		resp = append(resp, schema.UserManagerProfileResponse{Name: p.Name})
	}
	return resp, nil
}

// GetProtocolConfigs/UpsertProtocolConfig are plain DB CRUD for the global,
// admin-only per-protocol port/settings table. Admin-only enforcement
// happens at the HTTP layer, matching ResellerController's convention.
func (s *UserManagerService) GetProtocolConfigs() ([]schema.UserManagerProtocolConfigResponse, error) {
	var configs []model.UserManagerProtocolConfig
	if err := s.db.Find(&configs).Error; err != nil {
		s.logger.Error("failed to fetch user manager protocol configs", zap.Error(err))
		return nil, err
	}

	resp := make([]schema.UserManagerProtocolConfigResponse, 0, len(configs))
	for _, c := range configs {
		resp = append(resp, schema.UserManagerProtocolConfigResponse{
			Id:                 c.ID,
			Protocol:           string(c.Protocol),
			Port:               c.Port,
			ServerAddress:      c.ServerAddress,
			CertificateName:    c.CertificateName,
			Enabled:            c.Enabled,
			Notes:              c.Notes,
			HasCertificateFile: c.HasCertificateFile,
			HasClientAppFile:   c.HasClientAppFile,
		})
	}
	return resp, nil
}

func (s *UserManagerService) UpsertProtocolConfig(req *schema.UpsertUserManagerProtocolConfigRequest) (*schema.UserManagerProtocolConfigResponse, error) {
	var existing model.UserManagerProtocolConfig
	err := s.db.Where("protocol = ?", req.Protocol).First(&existing).Error
	if err == nil {
		existing.Port = req.Port
		existing.ServerAddress = req.ServerAddress
		existing.CertificateName = req.CertificateName
		existing.Enabled = req.Enabled
		existing.Notes = req.Notes
		if err := s.db.Save(&existing).Error; err != nil {
			s.logger.Error("failed to update user manager protocol config", zap.Error(err))
			return nil, err
		}
	} else if errors.Is(err, gorm.ErrRecordNotFound) {
		existing = model.UserManagerProtocolConfig{
			Protocol:        model.UserManagerAccountProtocol(req.Protocol),
			Port:            req.Port,
			ServerAddress:   req.ServerAddress,
			CertificateName: req.CertificateName,
			Enabled:         req.Enabled,
			Notes:           req.Notes,
		}
		if err := s.db.Create(&existing).Error; err != nil {
			s.logger.Error("failed to create user manager protocol config", zap.Error(err))
			return nil, err
		}
	} else {
		s.logger.Error("failed to look up user manager protocol config", zap.Error(err))
		return nil, err
	}

	return &schema.UserManagerProtocolConfigResponse{
		Id:                 existing.ID,
		Protocol:           string(existing.Protocol),
		Port:               existing.Port,
		ServerAddress:      existing.ServerAddress,
		CertificateName:    existing.CertificateName,
		Enabled:            existing.Enabled,
		Notes:              existing.Notes,
		HasCertificateFile: existing.HasCertificateFile,
		HasClientAppFile:   existing.HasClientAppFile,
	}, nil
}

func (s *UserManagerService) transformAccountToResponse(account model.UserManagerAccount, isOnline bool) schema.UserManagerAccountResponse {
	statuses := s.transformAccountStatus(account)

	var trafficLimit *string
	if account.TrafficLimit != nil {
		trafficLimit = utils.Ptr(utils.BytesToGB(*account.TrafficLimit))
	}

	accountProtocols := model.ParseProtocols(account.Protocols)
	protocolStrings := make([]string, len(accountProtocols))
	for i, p := range accountProtocols {
		protocolStrings[i] = string(p)
	}

	return schema.UserManagerAccountResponse{
		Id:            account.ID,
		UUID:          account.UUID,
		Username:      account.Username,
		Group:         account.Group,
		Profile:       account.Profile,
		Protocols:     protocolStrings,
		Disabled:      account.Disabled,
		Comment:       account.Comment,
		SharedUsers:   account.SharedUsers,
		TrafficLimit:  trafficLimit,
		ExpireTime:    account.ExpireTime,
		TotalUsage:    utils.BytesToGB(account.DownloadUsage + account.UploadUsage),
		Status:        statuses,
		IsShared:      account.IsShared,
		HasConfigFile: account.HasConfigFile,
		ResellerID:    account.ResellerID,
		IsOnline:      isOnline,
	}
}

func (s *UserManagerService) transformAccountStatus(account model.UserManagerAccount) []schema.UserManagerAccountStatus {
	var statuses []schema.UserManagerAccountStatus

	if account.Disabled {
		statuses = append(statuses, schema.InactiveUserManagerAccount)
	} else {
		statuses = append(statuses, schema.ActiveUserManagerAccount)
	}

	if account.ExpireTime != nil {
		expireTime, err := time.Parse("2006-01-02", *account.ExpireTime)
		if err == nil && time.Now().After(expireTime) {
			statuses = append(statuses, schema.ExpiredUserManagerAccount)
		}
	}

	if account.TrafficLimit != nil {
		totalUsed := account.DownloadUsage + account.UploadUsage
		if totalUsed > *account.TrafficLimit {
			statuses = append(statuses, schema.SuspendedUserManagerAccount)
		}
	}

	return statuses
}
