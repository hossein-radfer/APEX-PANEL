package service

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/yeqown/go-qrcode/v2"
	"github.com/yeqown/go-qrcode/writer/standard"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/maahdima/mwp/api/adaptor/mikrotik"
	"github.com/maahdima/mwp/api/common"
	"github.com/maahdima/mwp/api/config"
	"github.com/maahdima/mwp/api/dataservice/model"
	"github.com/maahdima/mwp/api/utils"
)

var (
	peerQrCodesPath string
)

type QRCodeGenerator struct {
	db              *gorm.DB
	mikrotikAdaptor *mikrotik.Adaptor
	logger          *zap.Logger
}

func init() {
	appCfg := config.GetAppConfig()
	peerQrCodesPath = filepath.Join(appCfg.PeerFilesDir, "qrcode")
	if err := os.MkdirAll(peerQrCodesPath, os.ModePerm); err != nil {
		panic(fmt.Sprintf("failed to create QR code directory: %v", err))
	}
}

func NewQRCodeGenerator(db *gorm.DB, mikrotikAdaptor *mikrotik.Adaptor) *QRCodeGenerator {
	return &QRCodeGenerator{
		db:              db,
		mikrotikAdaptor: mikrotikAdaptor,
		logger:          zap.L().Named("QRCodeGenerator"),
	}
}

func (q *QRCodeGenerator) GetPeerQRCode(id uint) (qrcodePath string, err error) {
	var peer model.Peer

	if err = q.db.First(&peer, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			q.logger.Error("peer not found in database", zap.Uint("id", id))
			return
		}
		q.logger.Error("failed to get peer from database", zap.Uint("id", id), zap.Error(err))
		return
	}

	qrcodePath = fmt.Sprintf("%s/%s.jpeg", peerQrCodesPath, peer.UUID)

	if _, statErr := os.Stat(qrcodePath); statErr != nil {
		// Same on-miss regeneration as ConfigGenerator.GetPeerConfig --
		// see its doc comment for why this happens (DB-only backup
		// restore, or a server migration that didn't copy PeerFilesDir).
		q.logger.Warn("peer QR code missing on disk, regenerating from database", zap.Uint("id", id), zap.String("uuid", peer.UUID))
		if regenErr := q.regeneratePeerQRCodeFile(peer); regenErr != nil {
			q.logger.Error("failed to regenerate missing peer QR code", zap.Uint("id", id), zap.Error(regenErr))
			return "", regenErr
		}
	}

	return qrcodePath, nil
}

func (q *QRCodeGenerator) regeneratePeerQRCodeFile(peer model.Peer) error {
	var iface model.Interface
	if err := q.db.Where("name = ?", peer.Interface).First(&iface).Error; err != nil {
		return fmt.Errorf("failed to look up interface %q for QR code regeneration: %w", peer.Interface, err)
	}

	configText := buildPeerConfigString(peer, iface.PublicKey, q.mikrotikAdaptor, q.logger)
	return q.BuildPeerQRCode(configText, peer.UUID)
}

func (q *QRCodeGenerator) GetUserQRCode(uuid string) (qrcodePath string, err error) {
	var peer model.Peer

	if err = q.db.First(&peer, "uuid = ?", uuid).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			q.logger.Error("peer not found in database", zap.String("uuid", uuid))
			return
		}
		q.logger.Error("failed to get peer from database", zap.String("uuid", uuid), zap.Error(err))
		return
	}

	isSharable := utils.IsPeerSharable(peer.IsShared, peer.ShareExpireTime)
	if !isSharable {
		return "", common.ErrPeerNotShared
	}

	qrcodePath = fmt.Sprintf("%s/%s.jpeg", peerQrCodesPath, peer.UUID)

	if _, statErr := os.Stat(qrcodePath); statErr != nil {
		q.logger.Warn("shared peer QR code missing on disk, regenerating from database", zap.String("uuid", uuid))
		if regenErr := q.regeneratePeerQRCodeFile(peer); regenErr != nil {
			q.logger.Error("failed to regenerate missing shared peer QR code", zap.String("uuid", uuid), zap.Error(regenErr))
			return "", regenErr
		}
	}

	return qrcodePath, nil
}

func (q *QRCodeGenerator) BuildPeerQRCode(config string, uuid string) error {
	qrc, err := qrcode.New(config)
	if err != nil {
		fmt.Printf("could not generate QRCode: %v", err)
		return err
	}

	filePath := fmt.Sprintf("%s/%s.jpeg", peerQrCodesPath, uuid)

	w, err := standard.New(filePath)
	if err != nil {
		fmt.Printf("standard.New failed: %v", err)
		return err
	}

	if err = qrc.Save(w); err != nil {
		fmt.Printf("could not save image: %v", err)
	}

	return nil
}

func (q *QRCodeGenerator) RemovePeerQRCode(id uint) error {
	var peer model.Peer

	if err := q.db.First(&peer, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			q.logger.Error("peer not found in database", zap.Uint("id", id))
			return err
		}
		q.logger.Error("failed to get peer from database", zap.Uint("id", id), zap.Error(err))
		return err
	}

	qrcodePath := fmt.Sprintf("%s/%s.jpeg", peerQrCodesPath, peer.UUID)

	err := os.Remove(qrcodePath)
	if err != nil {
		q.logger.Error("failed to remove QRCode", zap.String("path", qrcodePath), zap.Error(err))
		return err
	}

	return nil
}
