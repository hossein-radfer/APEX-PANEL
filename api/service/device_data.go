package service

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/maahdima/mwp/api/adaptor/mikrotik"
	"github.com/maahdima/mwp/api/common"
	"github.com/maahdima/mwp/api/dataservice/model"
	"github.com/maahdima/mwp/api/http/schema"
	"github.com/maahdima/mwp/api/utils"
)

type IPApiResponse struct {
	Status  string `json:"status"`
	Country string `json:"country"`
	City    string `json:"city"`
	ISP     string `json:"isp"`
}

type DeviceData struct {
	db               *gorm.DB
	mikrotikAdaptor  *mikrotik.Adaptor
	serverService    *Server
	interfaceService *WgInterface
	peerService      *WgPeer
	logger           *zap.Logger
}

func NewDeviceData(db *gorm.DB, mikrotikAdaptor *mikrotik.Adaptor, serverService *Server, interfaceService *WgInterface, peerService *WgPeer) *DeviceData {
	return &DeviceData{
		db:               db,
		mikrotikAdaptor:  mikrotikAdaptor,
		serverService:    serverService,
		interfaceService: interfaceService,
		peerService:      peerService,
		logger:           zap.L().Named("DeviceDataService"),
	}
}

func (d *DeviceData) GetDailyTrafficUsage(rangeParam string) (*[]schema.DailyTrafficUsageResponse, error) {
	var trafficData []model.Traffic

	days, err := strconv.Atoi(rangeParam)
	if err != nil || days <= 0 {
		d.logger.Error("invalid range param", zap.String("range", rangeParam), zap.Error(err))
		return nil, fmt.Errorf("invalid range parameter: %s", rangeParam)
	}

	startDate := time.Now().AddDate(0, 0, -days)

	if err := d.db.
		Where("created_at >= ?", startDate.Unix()).
		Order("created_at ASC").
		Find(&trafficData).Error; err != nil {
		d.logger.Error("failed to fetch daily traffic usage data", zap.Error(err))
		return nil, err
	}

	var dailyTrafficUsages []schema.DailyTrafficUsageResponse
	for _, data := range trafficData {
		dailyUsage := schema.DailyTrafficUsageResponse{
			Date:          time.Unix(int64(data.CreatedAt), 0).Format("2006-01-02"),
			DownloadUsage: utils.BytesToGB(data.DownloadUsage),
			UploadUsage:   utils.BytesToGB(data.UploadUsage),
			TotalUsage:    utils.BytesToGB(data.TotalUsage),
		}
		dailyTrafficUsages = append(dailyTrafficUsages, dailyUsage)
	}

	return &dailyTrafficUsages, nil
}

// GetDeviceData assembles the admin dashboard's full summary from eight
// independent sub-fetches. Confirmed, reported production bug this fixes:
// this used to abort and return NO data at all (a bare 500, surfaced to
// the frontend as every single stats card silently rendering blank --
// StatsCard has no "error"/"no data" state, only isLoading vs. a value)
// the moment ANY ONE of the eight failed. Four of the eight
// (getDeviceInfo/getDeviceIdentity/getDNSConfig/GetPeersData) each make a
// LIVE network call to a Mikrotik router via mikrotikAdaptor -- and on a
// multi-server deployment, that adaptor's default client
// (common.MwpClients.GetClient(nil)) picks an ARBITRARY single server out
// of the configured set (Go map iteration order), not necessarily a
// healthy one. So a single slow/unreachable/misconfigured router --
// completely unrelated to whether the OTHER configured servers are fine,
// and unrelated to ServerInfo/InterfaceInfo/TrafficInfo, which are pure
// DB reads with no network dependency at all -- blacked out the entire
// admin dashboard, including numbers that had nothing to do with that
// router.
//
// Each sub-fetch is now independent: a failure is logged and that ONE
// section comes back nil (every field in schema.DeviceStatsResponse is
// already a pointer, so nil there was always a valid, serializable
// state -- this orchestration function was the only thing not taking
// advantage of that). The frontend is expected to render a section as
// "no data" rather than blank when its corresponding field is null,
// exactly like it already does for other genuinely-empty states
// elsewhere in the panel.
func (d *DeviceData) GetDeviceData() (*schema.DeviceStatsResponse, error) {
	serverStats, err := d.serverService.GetServersData()
	if err != nil {
		d.logger.Error("failed to fetch server stats", zap.Error(err))
		serverStats = nil
	}

	interfaceStats, err := d.interfaceService.GetInterfacesData()
	if err != nil {
		d.logger.Error("failed to fetch interface stats", zap.Error(err))
		interfaceStats = nil
	}

	peerStats, err := d.peerService.GetPeersData()
	if err != nil {
		d.logger.Error("failed to fetch peer stats", zap.Error(err))
		peerStats = nil
	}

	trafficInfo, err := d.getTrafficInfo()
	if err != nil {
		d.logger.Error("failed to fetch traffic info", zap.Error(err))
		trafficInfo = nil
	}

	info, err := d.getDeviceInfo()
	if err != nil {
		// getDeviceInfo already logs its own error.
		info = nil
	}

	identity, err := d.getDeviceIdentity()
	if err != nil {
		identity = nil
	}

	ipv4, err := d.getDeviceIpAddress()
	if err != nil {
		// getDeviceIpAddress already returns a non-nil (partial) result
		// alongside a non-nil error for most of its own internal
		// failures -- ipv4 is kept as-is here (may still carry a partial
		// IPv4/ISP) rather than discarded, matching its own established
		// "best effort" contract.
		d.logger.Warn("device IP address lookup incomplete", zap.Error(err))
	}

	dns, err := d.getDNSConfig()
	if err != nil {
		dns = nil
	}

	return &schema.DeviceStatsResponse{
		ServerInfo:        serverStats,
		InterfaceInfo:     interfaceStats,
		PeerInfo:          peerStats,
		TrafficInfo:       trafficInfo,
		DeviceInfo:        info,
		DeviceIdentity:    identity,
		DeviceIPv4Address: ipv4,
		DNSConfig:         dns,
	}, nil
}

func (d *DeviceData) getTrafficInfo() (*schema.TrafficInfo, error) {
	var totalTraffic model.TotalTrafficUsage
	if err := d.db.FirstOrCreate(&totalTraffic, model.TotalTrafficUsage{Model: model.Model{ID: model.TotalTrafficUsageSingletonID}}).Error; err != nil {
		d.logger.Error("failed to fetch total traffic usage", zap.Error(err))
		return nil, err
	}

	// User Manager accounts are direct-copy accounted (DownloadUsage/
	// UploadUsage always mirror RouterOS's current cumulative counters, not
	// an accumulator -- see cmd/jobs/traffic.go's processUserManagerAccountUsage
	// doc comment), so a live SUM() across all accounts is the correct way
	// to fold them into the dashboard's total, unlike WireGuard's singleton
	// which is itself already an accumulator.
	var userManagerUsage int64
	if err := d.db.Model(&model.UserManagerAccount{}).
		Select("COALESCE(SUM(download_usage + upload_usage), 0)").
		Scan(&userManagerUsage).Error; err != nil {
		d.logger.Error("failed to sum user manager usage", zap.Error(err))
		return nil, err
	}

	return &schema.TrafficInfo{
		TotalUsage:       utils.BytesToGB(totalTraffic.TotalUsage + userManagerUsage),
		WireGuardUsage:   utils.BytesToGB(totalTraffic.TotalUsage),
		UserManagerUsage: utils.BytesToGB(userManagerUsage),
	}, nil
}

func (d *DeviceData) getDeviceInfo() (*schema.DeviceInfo, error) {
	info, err := d.mikrotikAdaptor.FetchDeviceInfo(context.Background())
	if err != nil {
		d.logger.Error("failed to fetch device resource", zap.Error(err))
		return nil, err
	}

	return &schema.DeviceInfo{
		BoardName:   info.BoardName,
		OSVersion:   info.Version,
		CpuArch:     info.ArchitectureName,
		Uptime:      info.Uptime,
		CpuLoad:     info.CPULoad,
		TotalMemory: info.TotalMemory,
		FreeMemory:  info.FreeMemory,
		TotalDisk:   info.TotalHDDSpace,
		FreeDisk:    info.FreeHDDSpace,
	}, nil
}

func (d *DeviceData) getDeviceIdentity() (*schema.DeviceIdentity, error) {
	identity, err := d.mikrotikAdaptor.FetchDeviceIdentity(context.Background())
	if err != nil {
		d.logger.Error("failed to fetch device identity", zap.Error(err))
		return nil, err
	}

	return &schema.DeviceIdentity{
		Identity: identity.Name,
	}, nil
}

func (d *DeviceData) getDeviceIpAddress() (*schema.DeviceIPv4Address, error) {
	deviceIpData := &schema.DeviceIPv4Address{}

	ipv4Addresses, err := d.mikrotikAdaptor.FetchIPv4Addresses(context.Background())
	if err != nil {
		d.logger.Error("failed to fetch IPv4 address", zap.Error(err))
		return deviceIpData, err
	}

	for _, ipv4 := range ipv4Addresses {
		if ipv4.Interface == common.IPv4DefaultInterface {
			deviceIpData.IPv4 = ipv4.Address
		}
	}

	// TODO : multi-server support
	var server model.Server
	if err := d.db.First(&server).Error; err != nil {
		d.logger.Error("failed to fetch server record from database", zap.Error(err))
		return deviceIpData, err
	}

	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}

	httpClient := &http.Client{
		Transport: transport,
		Timeout:   5 * time.Second,
	}

	ipApiURL := fmt.Sprintf("http://ip-api.com/json/%s?fields=status,message,country,city,isp", server.IPAddress)
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, ipApiURL, nil)
	if err != nil {
		d.logger.Error("failed to create IP API request", zap.Error(err))
		return deviceIpData, err
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		d.logger.Error("HTTP request to IP API failed", zap.Error(err))
		return deviceIpData, nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		d.logger.Error("unexpected response status from IP API", zap.Int("status", resp.StatusCode))
		return deviceIpData, nil
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		d.logger.Error("failed to read response body from IP API", zap.Error(err))
		return deviceIpData, nil
	}

	var respBody IPApiResponse
	if err := json.Unmarshal(body, &respBody); err != nil {
		d.logger.Error("failed to parse IP API response", zap.Error(err))
		return deviceIpData, nil
	}

	deviceIpData.ISP = respBody.ISP

	return deviceIpData, err
}

func (d *DeviceData) getDNSConfig() (*schema.DNSConfig, error) {
	dns, err := d.mikrotikAdaptor.FetchDNSConfig(context.Background())
	if err != nil {
		d.logger.Error("failed to fetch dns config", zap.Error(err))
		return nil, err
	}

	return &schema.DNSConfig{
		DnsServer: dns.Servers + dns.DynamicServers,
	}, nil
}
