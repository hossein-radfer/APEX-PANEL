package service

import (
	"errors"
	"io"

	"go.uber.org/zap"
)

// ErrNotLicensed is returned by every HelpCenterService method when this
// install has no activated license key yet -- the Help Center is entirely
// license-panel-backed, so without a license key there is no account to
// authenticate the request as. Mirrors LicenseService's own "not yet
// activated" handling (see StoredLicenseKey's doc comment) rather than
// introducing a new error-handling convention.
var ErrNotLicensed = errors.New("this install is not licensed yet")

// HelpCenterService is a thin proxy: every method resolves this install's
// stored license key and forwards the request to license-panel's public
// Help Center API (see license.Client's Help Center methods), returning
// the raw JSON bytes for the HTTP handler to pass straight through. The
// browser never sees or sends the license key itself -- only this
// server-side service does, exactly like LicenseService.CheckForUpdate.
type HelpCenterService struct {
	licenseService *LicenseService
	logger         *zap.Logger
}

func NewHelpCenterService(licenseService *LicenseService) *HelpCenterService {
	return &HelpCenterService{licenseService: licenseService, logger: zap.L().Named("HelpCenterService")}
}

func (s *HelpCenterService) storedKey() (string, error) {
	key, err := s.licenseService.StoredLicenseKey()
	if err != nil {
		return "", err
	}
	if key == "" {
		return "", ErrNotLicensed
	}
	return key, nil
}

func (s *HelpCenterService) ListTickets() ([]byte, error) {
	key, err := s.storedKey()
	if err != nil {
		return nil, err
	}
	return s.licenseService.Client().ListTickets(key)
}

func (s *HelpCenterService) CreateTicket(subject, message string) ([]byte, error) {
	key, err := s.storedKey()
	if err != nil {
		return nil, err
	}
	return s.licenseService.Client().CreateTicket(key, subject, message)
}

func (s *HelpCenterService) GetTicketMessages(ticketID uint) ([]byte, error) {
	key, err := s.storedKey()
	if err != nil {
		return nil, err
	}
	return s.licenseService.Client().GetTicketMessages(key, ticketID)
}

func (s *HelpCenterService) PostTicketMessage(ticketID uint, body, kind, fileName string, fileContent io.Reader) ([]byte, error) {
	key, err := s.storedKey()
	if err != nil {
		return nil, err
	}
	return s.licenseService.Client().PostTicketMessage(key, ticketID, body, kind, fileName, fileContent)
}

func (s *HelpCenterService) SetTyping(ticketID uint) error {
	key, err := s.storedKey()
	if err != nil {
		return err
	}
	return s.licenseService.Client().SetTyping(key, ticketID)
}

func (s *HelpCenterService) GetTypingStatus(ticketID uint) ([]byte, error) {
	key, err := s.storedKey()
	if err != nil {
		return nil, err
	}
	return s.licenseService.Client().GetTypingStatus(key, ticketID)
}

func (s *HelpCenterService) DownloadAttachment(ticketID, messageID uint) (*licenseAttachmentStream, error) {
	key, err := s.storedKey()
	if err != nil {
		return nil, err
	}
	resp, err := s.licenseService.Client().DownloadAttachment(key, ticketID, messageID)
	if err != nil {
		return nil, err
	}
	return &licenseAttachmentStream{
		Body:               resp.Body,
		ContentType:        resp.Header.Get("Content-Type"),
		ContentDisposition: resp.Header.Get("Content-Disposition"),
	}, nil
}

// licenseAttachmentStream carries just what http.HelpCenterController
// needs to relay an attachment download response without depending on
// net/http.Response directly at the HTTP layer.
type licenseAttachmentStream struct {
	Body               io.ReadCloser
	ContentType        string
	ContentDisposition string
}
