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

// ResourceSampler is the Reports section's periodic Mikrotik CPU/memory
// history collector -- see model.ResourceSample's own doc comment for why
// this exists (FetchDeviceInfo only ever returns the CURRENT reading, with
// no history anywhere in this codebase prior to this).
type ResourceSampler struct {
	db              *gorm.DB
	mikrotikAdaptor *mikrotik.Adaptor
	logger          *zap.Logger
}

func NewResourceSampler(db *gorm.DB, mikrotikAdaptor *mikrotik.Adaptor) *ResourceSampler {
	return &ResourceSampler{
		db:              db,
		mikrotikAdaptor: mikrotikAdaptor,
		logger:          zap.L().Named("ResourceSampler"),
	}
}

// Sample takes one reading -- called on the same scheduler cadence as the
// traffic jobs (see cmd/main.go's job registration). A Mikrotik-side
// failure here is logged and simply skips this tick's sample rather than
// erroring, matching every other job's own "one dead connection never
// crashes the process" convention.
func (s *ResourceSampler) Sample() {
	info, err := s.mikrotikAdaptor.FetchDeviceInfo(context.Background())
	if err != nil {
		s.logger.Warn("failed to fetch device info for resource sample", zap.Error(err))
		return
	}

	cpuLoad, err := strconv.ParseFloat(info.CPULoad, 64)
	if err != nil {
		s.logger.Warn("failed to parse cpu-load for resource sample", zap.String("raw", info.CPULoad), zap.Error(err))
		return
	}

	totalMem, err := strconv.ParseFloat(info.TotalMemory, 64)
	if err != nil || totalMem <= 0 {
		s.logger.Warn("failed to parse total-memory for resource sample", zap.String("raw", info.TotalMemory), zap.Error(err))
		return
	}
	freeMem, err := strconv.ParseFloat(info.FreeMemory, 64)
	if err != nil {
		s.logger.Warn("failed to parse free-memory for resource sample", zap.String("raw", info.FreeMemory), zap.Error(err))
		return
	}

	memUsedPercent := (totalMem - freeMem) / totalMem * 100
	if memUsedPercent < 0 {
		memUsedPercent = 0
	}

	sample := model.ResourceSample{
		Timestamp:         time.Now().Unix(),
		CPULoadPercent:    cpuLoad,
		MemoryUsedPercent: memUsedPercent,
	}
	if err := s.db.Create(&sample).Error; err != nil {
		s.logger.Warn("failed to persist resource sample", zap.Error(err))
	}
}
