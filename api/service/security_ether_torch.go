package service

import (
	"context"
	"strconv"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/maahdima/mwp/api/adaptor/mikrotik"
	"github.com/maahdima/mwp/api/dataservice/model"
)

// securityTorchDurationSeconds/securityTorchInterface are fixed rather than
// admin-configurable for this first version -- the admin explicitly asked
// for a fixed poll cadence ("هر ۱۰ ثانیه بگیر"), and the sample duration
// is kept short (1s) so this call comfortably finishes inside the shared
// Mikrotik HTTP client's default 5s timeout (see httphelper.Config's own
// doc comment) even accounting for network round-trip on top of RouterOS's
// own sampling time.
const securityTorchDurationSeconds = 1

// SecurityEtherTorchService polls `/tool/torch` on a single ether
// interface and records each flow as an EtherTrafficSample row, enriched
// with GeoIP -- the backend for the admin's "ترافیک کلی" (overall traffic)
// requirement on the Security page. Deliberately scoped to ONE interface
// per the admin's own request ("این رو فقط داخل اتریک بزار"), not every
// interface on the router.
type SecurityEtherTorchService struct {
	db              *gorm.DB
	mikrotikAdaptor *mikrotik.Adaptor
	geoIP           *GeoIPService
	logger          *zap.Logger
	interfaceName   string
}

func NewSecurityEtherTorchService(db *gorm.DB, mikrotikAdaptor *mikrotik.Adaptor, geoIP *GeoIPService, interfaceName string) *SecurityEtherTorchService {
	return &SecurityEtherTorchService{
		db:              db,
		mikrotikAdaptor: mikrotikAdaptor,
		geoIP:           geoIP,
		logger:          zap.L().Named("SecurityEtherTorchService"),
		interfaceName:   interfaceName,
	}
}

// Poll is the scheduled job entrypoint (registered on a 10s interval, see
// cmd/main.go). A failed torch call is logged and skipped -- same
// best-effort convention as every other collector in this codebase; a
// single missed tick is invisible in the resulting time series, not a
// reason to crash or retry aggressively.
func (s *SecurityEtherTorchService) Poll() {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	flows, err := s.mikrotikAdaptor.FetchTorchFlows(ctx, s.interfaceName, securityTorchDurationSeconds)
	if err != nil {
		s.logger.Warn("failed to poll torch", zap.String("interface", s.interfaceName), zap.Error(err))
		return
	}

	now := time.Now()
	for _, flow := range flows {
		if flow.SrcAddress == "" {
			continue
		}

		var geo *GeoIPLookupResult
		if resolved, err := s.geoIP.Lookup(flow.SrcAddress); err != nil {
			s.logger.Warn("geoip lookup failed for torch flow, recording without location", zap.String("ip", flow.SrcAddress), zap.Error(err))
			geo = &GeoIPLookupResult{}
		} else {
			geo = resolved
		}

		sample := model.EtherTrafficSample{
			Interface:        s.interfaceName,
			SrcAddress:       flow.SrcAddress,
			IPProtocol:       flow.Protocol,
			SrcPort:          parseOptionalInt(flow.SrcPort),
			TxBytesPerSecond: parseOptionalInt64(flow.TxRate),
			RxBytesPerSecond: parseOptionalInt64(flow.RxRate),
			TxPacketsRate:    parseOptionalInt64(flow.TxPacketsRate),
			RxPacketsRate:    parseOptionalInt64(flow.RxPacketsRate),
			Country:          geo.Country, Region: geo.Region, City: geo.City,
			Lat: geo.Lat, Lon: geo.Lon, Timezone: geo.Timezone,
			ASN: geo.ASN, ISP: geo.ISP,
			SampledAt: now,
		}
		if err := s.db.Create(&sample).Error; err != nil {
			s.logger.Error("failed to store ether traffic sample", zap.Error(err))
		}
	}
}

func parseOptionalInt(s string) *int {
	if s == "" {
		return nil
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return nil
	}
	return &v
}

func parseOptionalInt64(s string) int64 {
	if s == "" {
		return 0
	}
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0
	}
	return v
}
