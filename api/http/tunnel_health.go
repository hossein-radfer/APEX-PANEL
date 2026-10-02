package http

import (
	"net/http"
	"strconv"
	"time"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	"github.com/maahdima/mwp/api/dataservice/model"
	"github.com/maahdima/mwp/api/http/schema"
	"github.com/maahdima/mwp/api/service"
)

// TunnelHealthController exposes the "tunnel-ai" dashboard: current
// per-interface health, transition/action history, per-tunnel policy
// CRUD, the management redline list, and the dry-run/emergency-stop
// toggles. Nothing here ever writes to RouterOS directly -- mutating
// actions all go through TunnelHealthService's own decision engine,
// itself gated by IsDryRun/IsEmergencyStopped and
// TunnelRedlineValidator. Admin-only, same forbidUnlessAdmin gate as
// every other admin-facing controller.
type TunnelHealthController struct {
	tunnelHealthService              *service.TunnelHealthService
	policyService                    *service.TunnelPolicyService
	redline                          *service.TunnelRedlineValidator
	graphService                     *service.TunnelGraphService
	userManagerProtocolHealthService *service.UserManagerProtocolHealthService
	logger                           *zap.Logger
}

func NewTunnelHealthController(
	tunnelHealthService *service.TunnelHealthService,
	policyService *service.TunnelPolicyService,
	redline *service.TunnelRedlineValidator,
	graphService *service.TunnelGraphService,
	userManagerProtocolHealthService *service.UserManagerProtocolHealthService,
) *TunnelHealthController {
	return &TunnelHealthController{
		tunnelHealthService:              tunnelHealthService,
		policyService:                    policyService,
		redline:                          redline,
		graphService:                     graphService,
		userManagerProtocolHealthService: userManagerProtocolHealthService,
		logger:                           zap.L().Named("TunnelHealthController"),
	}
}

func toTunnelHealthStatusResponse(status model.TunnelHealthStatus) schema.TunnelHealthStatusResponse {
	return schema.TunnelHealthStatusResponse{
		Id:                               status.ID,
		InterfaceName:                    status.InterfaceName,
		Severity:                         status.Severity,
		InterfaceRunning:                 status.InterfaceRunning,
		InterfaceDisabled:                status.InterfaceDisabled,
		WorstPeerLastHandshakeAgeSeconds: status.WorstPeerLastHandshakeAgeSeconds,
		TxRxAsymmetryDetected:            status.TxRxAsymmetryDetected,
		LastPolledAt:                     status.LastPolledAt.Format(time.RFC3339),
	}
}

func toTunnelHealthEventResponse(event model.TunnelHealthEvent) schema.TunnelHealthEventResponse {
	resp := schema.TunnelHealthEventResponse{
		Id:            event.ID,
		InterfaceName: event.InterfaceName,
		FromStatus:    event.FromStatus,
		ToStatus:      event.ToStatus,
		Evidence:      event.Evidence,
		DetectedAt:    event.DetectedAt.Format(time.RFC3339),
	}
	if event.NotifiedAt != nil {
		notifiedAt := event.NotifiedAt.Format(time.RFC3339)
		resp.NotifiedAt = &notifiedAt
	}
	return resp
}

func toTunnelActionLogResponse(action model.TunnelActionLog) schema.TunnelActionLogResponse {
	return schema.TunnelActionLogResponse{
		Id:                 action.ID,
		InterfaceName:      action.InterfaceName,
		Level:              action.Level,
		CommandDescription: action.CommandDescription,
		CommandDetail:      action.CommandDetail,
		Simulated:          action.Simulated,
		Result:             action.Result,
		ExecutedAt:         action.ExecutedAt.Format(time.RFC3339),
	}
}

func toTunnelPolicyResponse(policy model.TunnelPolicy, config service.TunnelPolicyConfig) schema.TunnelPolicyResponse {
	return schema.TunnelPolicyResponse{
		Id:            policy.ID,
		InterfaceName: policy.InterfaceName,
		Detection: schema.TunnelPolicyDetectionResponse{
			PingFailThreshold:          config.Detection.PingFailThreshold,
			TxRxAsymmetryWindowMinutes: config.Detection.TxRxAsymmetryWindowMinutes,
			TxRxAsymmetryRatio:         config.Detection.TxRxAsymmetryRatio,
		},
		Level1: schema.TunnelPolicyLevel1Response{
			Action:      config.Level1.Action,
			WaitSeconds: config.Level1.WaitSeconds,
		},
		Level2: schema.TunnelPolicyLevel2Response{
			BackupTarget:    config.Level2.BackupTarget,
			BackupGatewayIP: config.Level2.BackupGatewayIP,
		},
		Level3: schema.TunnelPolicyLevel3Response{
			Enabled:          config.Level3.Enabled,
			Type:             config.Level3.Type,
			RemoteAddress:    config.Level3.RemoteAddress,
			LocalAddress:     config.Level3.LocalAddress,
			Masquerade:       config.Level3.Masquerade,
			GatewayIP:        config.Level3.GatewayIP,
			NewInterfaceName: config.Level3.NewInterfaceName,
		},
		Fallback: schema.TunnelPolicyFallbackResponse{
			StableDurationSeconds: config.Fallback.StableDurationSeconds,
		},
		AntiFlapping: schema.TunnelPolicyAntiFlappingResponse{
			MaxFlaps:      config.AntiFlapping.MaxFlaps,
			WindowMinutes: config.AntiFlapping.WindowMinutes,
			CooldownHours: config.AntiFlapping.CooldownHours,
		},
	}
}

func toRedlineEntryResponse(entry model.ManagementRedlineEntry) schema.ManagementRedlineEntryResponse {
	return schema.ManagementRedlineEntryResponse{
		Id:      entry.ID,
		Kind:    entry.Kind,
		Value:   entry.Value,
		Comment: entry.Comment,
	}
}

func (c *TunnelHealthController) ListStatuses(ctx echo.Context) error {
	if forbidden, err := forbidUnlessAdmin(ctx); forbidden {
		return err
	}

	statuses, err := c.tunnelHealthService.ListStatuses()
	if err != nil {
		return ctx.JSON(http.StatusInternalServerError, schema.InternalServerErrorResponse)
	}

	resp := make([]schema.TunnelHealthStatusResponse, 0, len(statuses))
	for _, status := range statuses {
		resp = append(resp, toTunnelHealthStatusResponse(status))
	}
	return ctx.JSON(http.StatusOK, schema.BasicResponseData[[]schema.TunnelHealthStatusResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data:          resp,
	})
}

func (c *TunnelHealthController) ListEvents(ctx echo.Context) error {
	if forbidden, err := forbidUnlessAdmin(ctx); forbidden {
		return err
	}

	limit := 100
	if raw := ctx.QueryParam("limit"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil {
			limit = parsed
		}
	}

	events, err := c.tunnelHealthService.ListEvents(limit)
	if err != nil {
		return ctx.JSON(http.StatusInternalServerError, schema.InternalServerErrorResponse)
	}

	resp := make([]schema.TunnelHealthEventResponse, 0, len(events))
	for _, event := range events {
		resp = append(resp, toTunnelHealthEventResponse(event))
	}
	return ctx.JSON(http.StatusOK, schema.BasicResponseData[[]schema.TunnelHealthEventResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data:          resp,
	})
}

func (c *TunnelHealthController) ListActions(ctx echo.Context) error {
	if forbidden, err := forbidUnlessAdmin(ctx); forbidden {
		return err
	}

	limit := 100
	if raw := ctx.QueryParam("limit"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil {
			limit = parsed
		}
	}

	actions, err := c.tunnelHealthService.ListActions(limit)
	if err != nil {
		return ctx.JSON(http.StatusInternalServerError, schema.InternalServerErrorResponse)
	}

	resp := make([]schema.TunnelActionLogResponse, 0, len(actions))
	for _, action := range actions {
		resp = append(resp, toTunnelActionLogResponse(action))
	}
	return ctx.JSON(http.StatusOK, schema.BasicResponseData[[]schema.TunnelActionLogResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data:          resp,
	})
}

func (c *TunnelHealthController) GetSettings(ctx echo.Context) error {
	if forbidden, err := forbidUnlessAdmin(ctx); forbidden {
		return err
	}

	return ctx.JSON(http.StatusOK, schema.BasicResponseData[schema.TunnelAiSettingsResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data: schema.TunnelAiSettingsResponse{
			DryRun:        c.tunnelHealthService.IsDryRun(),
			EmergencyStop: c.tunnelHealthService.IsEmergencyStopped(),
		},
	})
}

// UpdateSettings toggles dry-run and/or the emergency-stop kill switch --
// this is the admin's own "گزینه آزمایشی" control from the current
// request: turning dry_run OFF is what lets the engine actually send
// commands to RouterOS instead of only logging what it would do.
func (c *TunnelHealthController) UpdateSettings(ctx echo.Context) error {
	if forbidden, err := forbidUnlessAdmin(ctx); forbidden {
		return err
	}

	var req schema.UpdateTunnelAiSettingsRequest
	if err := ctx.Bind(&req); err != nil {
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	if req.DryRun != nil {
		if err := c.tunnelHealthService.SetDryRun(*req.DryRun); err != nil {
			c.logger.Error("failed to update dry-run setting", zap.Error(err))
			return ctx.JSON(http.StatusInternalServerError, schema.InternalServerErrorResponse)
		}
	}
	if req.EmergencyStop != nil {
		if err := c.tunnelHealthService.SetEmergencyStop(*req.EmergencyStop); err != nil {
			c.logger.Error("failed to update emergency-stop setting", zap.Error(err))
			return ctx.JSON(http.StatusInternalServerError, schema.InternalServerErrorResponse)
		}
	}

	return ctx.JSON(http.StatusOK, schema.BasicResponseData[schema.TunnelAiSettingsResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data: schema.TunnelAiSettingsResponse{
			DryRun:        c.tunnelHealthService.IsDryRun(),
			EmergencyStop: c.tunnelHealthService.IsEmergencyStopped(),
		},
	})
}

func (c *TunnelHealthController) ListPolicies(ctx echo.Context) error {
	if forbidden, err := forbidUnlessAdmin(ctx); forbidden {
		return err
	}

	policies, err := c.policyService.List()
	if err != nil {
		return ctx.JSON(http.StatusInternalServerError, schema.InternalServerErrorResponse)
	}

	resp := make([]schema.TunnelPolicyResponse, 0, len(policies))
	for _, policy := range policies {
		config := c.policyService.Parse(&policy)
		resp = append(resp, toTunnelPolicyResponse(policy, config))
	}
	return ctx.JSON(http.StatusOK, schema.BasicResponseData[[]schema.TunnelPolicyResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data:          resp,
	})
}

func (c *TunnelHealthController) UpdatePolicy(ctx echo.Context) error {
	if forbidden, err := forbidUnlessAdmin(ctx); forbidden {
		return err
	}

	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	var req schema.UpdateTunnelPolicyRequest
	if err := ctx.Bind(&req); err != nil {
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}
	if err := ctx.Validate(&req); err != nil {
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	var config service.TunnelPolicyConfig
	config.Detection.PingFailThreshold = req.Detection.PingFailThreshold
	config.Detection.TxRxAsymmetryWindowMinutes = req.Detection.TxRxAsymmetryWindowMinutes
	config.Detection.TxRxAsymmetryRatio = req.Detection.TxRxAsymmetryRatio
	config.Level1.Action = req.Level1.Action
	config.Level1.WaitSeconds = req.Level1.WaitSeconds
	config.Level2.BackupTarget = req.Level2.BackupTarget
	config.Level2.BackupGatewayIP = req.Level2.BackupGatewayIP
	config.Level3.Enabled = req.Level3.Enabled
	config.Level3.Type = req.Level3.Type
	config.Level3.RemoteAddress = req.Level3.RemoteAddress
	config.Level3.LocalAddress = req.Level3.LocalAddress
	config.Level3.Masquerade = req.Level3.Masquerade
	config.Level3.GatewayIP = req.Level3.GatewayIP
	config.Level3.NewInterfaceName = req.Level3.NewInterfaceName
	config.Fallback.StableDurationSeconds = req.Fallback.StableDurationSeconds
	config.AntiFlapping.MaxFlaps = req.AntiFlapping.MaxFlaps
	config.AntiFlapping.WindowMinutes = req.AntiFlapping.WindowMinutes
	config.AntiFlapping.CooldownHours = req.AntiFlapping.CooldownHours

	policy, err := c.policyService.Update(uint(id), config)
	if err != nil {
		c.logger.Error("failed to update tunnel policy", zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.InternalServerErrorResponse)
	}

	return ctx.JSON(http.StatusOK, schema.BasicResponseData[schema.TunnelPolicyResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data:          toTunnelPolicyResponse(*policy, config),
	})
}

func (c *TunnelHealthController) ListRedlineEntries(ctx echo.Context) error {
	if forbidden, err := forbidUnlessAdmin(ctx); forbidden {
		return err
	}

	entries, err := c.redline.ListEntries()
	if err != nil {
		return ctx.JSON(http.StatusInternalServerError, schema.InternalServerErrorResponse)
	}

	resp := make([]schema.ManagementRedlineEntryResponse, 0, len(entries))
	for _, entry := range entries {
		resp = append(resp, toRedlineEntryResponse(entry))
	}
	return ctx.JSON(http.StatusOK, schema.BasicResponseData[[]schema.ManagementRedlineEntryResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data:          resp,
	})
}

func (c *TunnelHealthController) AddRedlineEntry(ctx echo.Context) error {
	if forbidden, err := forbidUnlessAdmin(ctx); forbidden {
		return err
	}

	var req schema.CreateRedlineEntryRequest
	if err := ctx.Bind(&req); err != nil {
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}
	if err := ctx.Validate(&req); err != nil {
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	entry, err := c.redline.AddEntry(req.Kind, req.Value, req.Comment)
	if err != nil {
		c.logger.Error("failed to add redline entry", zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.InternalServerErrorResponse)
	}

	return ctx.JSON(http.StatusCreated, schema.BasicResponseData[schema.ManagementRedlineEntryResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data:          toRedlineEntryResponse(*entry),
	})
}

// GetLatestGraph returns the most recent dependency-graph snapshot for
// the active server -- spec section ب-2/ب-3's own "دیده‌بان گراف"
// inspector, letting an admin see exactly what NAT/mangle/routing-table/
// route/tunnel chain the engine last discovered.
func (c *TunnelHealthController) GetLatestGraph(ctx echo.Context) error {
	if forbidden, err := forbidUnlessAdmin(ctx); forbidden {
		return err
	}

	snapshot, nodes, edges, err := c.graphService.LatestGraphForActiveServer()
	if err != nil {
		return ctx.JSON(http.StatusInternalServerError, schema.InternalServerErrorResponse)
	}
	if snapshot == nil {
		return ctx.JSON(http.StatusOK, schema.BasicResponseData[*schema.GraphResponse]{
			BasicResponse: schema.OkBasicResponse,
			Data:          nil,
		})
	}

	nodeResp := make([]schema.GraphNodeResponse, 0, len(nodes))
	for _, n := range nodes {
		nodeResp = append(nodeResp, schema.GraphNodeResponse{
			Id:            n.ID,
			Type:          n.Type,
			MikrotikRefID: n.MikrotikRefID,
			Name:          n.Name,
			Properties:    n.PropertiesJSON,
		})
	}
	edgeResp := make([]schema.GraphEdgeResponse, 0, len(edges))
	for _, e := range edges {
		edgeResp = append(edgeResp, schema.GraphEdgeResponse{
			Id:         e.ID,
			FromNodeID: e.FromNodeID,
			ToNodeID:   e.ToNodeID,
			Relation:   e.Relation,
		})
	}

	return ctx.JSON(http.StatusOK, schema.BasicResponseData[*schema.GraphResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data: &schema.GraphResponse{
			Snapshot: schema.GraphSnapshotResponse{Id: snapshot.ID, TakenAt: snapshot.TakenAt.Format(time.RFC3339)},
			Nodes:    nodeResp,
			Edges:    edgeResp,
		},
	})
}

// GetTunnelMap returns the server-computed protocol/location/tunnel
// hierarchy for the latest discovery snapshot -- the admin's own
// explicit correction that the graph inspector must build this grouping
// itself, not hand the UI a flat node/edge list to reassemble.
func (c *TunnelHealthController) GetTunnelMap(ctx echo.Context) error {
	if forbidden, err := forbidUnlessAdmin(ctx); forbidden {
		return err
	}

	snapshot, _, _, err := c.graphService.LatestGraphForActiveServer()
	if err != nil {
		return ctx.JSON(http.StatusInternalServerError, schema.InternalServerErrorResponse)
	}
	if snapshot == nil {
		return ctx.JSON(http.StatusOK, schema.BasicResponseData[*schema.TunnelMapResponse]{
			BasicResponse: schema.OkBasicResponse,
			Data:          nil,
		})
	}

	groups, err := c.graphService.BuildTunnelMap(snapshot.ID)
	if err != nil {
		return ctx.JSON(http.StatusInternalServerError, schema.InternalServerErrorResponse)
	}

	protocolResp := make([]schema.GraphProtocolGroupResponse, 0, len(groups))
	for _, g := range groups {
		locationResp := make([]schema.GraphLocationGroupResponse, 0, len(g.Locations))
		for _, loc := range g.Locations {
			tunnelResp := make([]schema.GraphTunnelResponse, 0, len(loc.Tunnels))
			for _, t := range loc.Tunnels {
				natRules := make([]schema.NatRuleSummaryResponse, len(t.NatRules))
				for i, r := range t.NatRules {
					natRules[i] = schema.NatRuleSummaryResponse{
						Comment:           r.Comment,
						Port:              r.Port,
						Target:            r.Target,
						MangleRoutingMark: r.MangleRoutingMark,
						RoutingTable:      r.RoutingTable,
					}
				}
				tunnelResp = append(tunnelResp, schema.GraphTunnelResponse{
					InterfaceName: t.InterfaceName,
					InterfaceType: t.InterfaceType,
					NatRules:      natRules,
				})
			}
			locationResp = append(locationResp, schema.GraphLocationGroupResponse{
				Location: loc.Location,
				Tunnels:  tunnelResp,
			})
		}
		protocolResp = append(protocolResp, schema.GraphProtocolGroupResponse{
			Protocol:  g.Protocol,
			Locations: locationResp,
		})
	}

	return ctx.JSON(http.StatusOK, schema.BasicResponseData[*schema.TunnelMapResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data: &schema.TunnelMapResponse{
			SnapshotTakenAt: snapshot.TakenAt.Format(time.RFC3339),
			Protocols:       protocolResp,
		},
	})
}

// ListUserManagerProtocolStatuses returns spec section ب-7's own
// alert-only health rows for L2TP/PPTP/SSTP/OpenVPN.
func (c *TunnelHealthController) ListUserManagerProtocolStatuses(ctx echo.Context) error {
	if forbidden, err := forbidUnlessAdmin(ctx); forbidden {
		return err
	}

	statuses, err := c.userManagerProtocolHealthService.ListStatuses()
	if err != nil {
		return ctx.JSON(http.StatusInternalServerError, schema.InternalServerErrorResponse)
	}

	resp := make([]schema.UserManagerProtocolHealthStatusResponse, 0, len(statuses))
	for _, s := range statuses {
		resp = append(resp, schema.UserManagerProtocolHealthStatusResponse{
			Protocol:      string(s.Protocol),
			Healthy:       s.Healthy,
			RouterEnabled: s.RouterEnabled,
			PortReachable: s.PortReachable,
			LastCheckedAt: s.LastCheckedAt.Format(time.RFC3339),
		})
	}
	return ctx.JSON(http.StatusOK, schema.BasicResponseData[[]schema.UserManagerProtocolHealthStatusResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data:          resp,
	})
}

func (c *TunnelHealthController) ListUserManagerProtocolEvents(ctx echo.Context) error {
	if forbidden, err := forbidUnlessAdmin(ctx); forbidden {
		return err
	}

	limit := 100
	if raw := ctx.QueryParam("limit"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil {
			limit = parsed
		}
	}

	events, err := c.userManagerProtocolHealthService.ListEvents(limit)
	if err != nil {
		return ctx.JSON(http.StatusInternalServerError, schema.InternalServerErrorResponse)
	}

	resp := make([]schema.UserManagerProtocolHealthEventResponse, 0, len(events))
	for _, e := range events {
		item := schema.UserManagerProtocolHealthEventResponse{
			Id:         e.ID,
			Protocol:   string(e.Protocol),
			FromStatus: e.FromStatus,
			ToStatus:   e.ToStatus,
			Evidence:   e.Evidence,
			DetectedAt: e.DetectedAt.Format(time.RFC3339),
		}
		if e.NotifiedAt != nil {
			notifiedAt := e.NotifiedAt.Format(time.RFC3339)
			item.NotifiedAt = &notifiedAt
		}
		resp = append(resp, item)
	}
	return ctx.JSON(http.StatusOK, schema.BasicResponseData[[]schema.UserManagerProtocolHealthEventResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data:          resp,
	})
}

// ListHealthScoreHistory returns one tunnel's own recent early-warning
// health-score trend (oldest first) -- the panel's own trend-chart data
// source, query-scoped to ?interface_name since scores are per-tunnel
// history, unlike ListStatuses's own single current-state row per tunnel.
func (c *TunnelHealthController) ListHealthScoreHistory(ctx echo.Context) error {
	if forbidden, err := forbidUnlessAdmin(ctx); forbidden {
		return err
	}

	interfaceName := ctx.QueryParam("interface_name")
	if interfaceName == "" {
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	limit := 0
	if raw := ctx.QueryParam("limit"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil {
			limit = parsed
		}
	}

	scores, err := c.tunnelHealthService.ListHealthScoreHistory(interfaceName, limit)
	if err != nil {
		return ctx.JSON(http.StatusInternalServerError, schema.InternalServerErrorResponse)
	}

	resp := make([]schema.TunnelHealthScoreResponse, 0, len(scores))
	for _, score := range scores {
		resp = append(resp, schema.TunnelHealthScoreResponse{
			Score:     score.Score,
			SampledAt: score.SampledAt.Format(time.RFC3339),
		})
	}
	return ctx.JSON(http.StatusOK, schema.BasicResponseData[[]schema.TunnelHealthScoreResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data:          resp,
	})
}

// ListIncidentDiagnoses returns the self-healing engine's own root-cause
// guesses for recent confirmed_down incidents (newest first).
func (c *TunnelHealthController) ListIncidentDiagnoses(ctx echo.Context) error {
	if forbidden, err := forbidUnlessAdmin(ctx); forbidden {
		return err
	}

	limit := 100
	if raw := ctx.QueryParam("limit"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil {
			limit = parsed
		}
	}

	diagnoses, err := c.tunnelHealthService.ListIncidentDiagnoses(limit)
	if err != nil {
		return ctx.JSON(http.StatusInternalServerError, schema.InternalServerErrorResponse)
	}

	resp := make([]schema.TunnelIncidentDiagnosisResponse, 0, len(diagnoses))
	for _, d := range diagnoses {
		resp = append(resp, schema.TunnelIncidentDiagnosisResponse{
			Id:            d.ID,
			InterfaceName: d.InterfaceName,
			Cause:         d.Cause,
			Evidence:      d.Evidence,
			DiagnosedAt:   d.DiagnosedAt.Format(time.RFC3339),
		})
	}
	return ctx.JSON(http.StatusOK, schema.BasicResponseData[[]schema.TunnelIncidentDiagnosisResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data:          resp,
	})
}

// ListBackupProbeResults returns recent nightly active backup-path probe
// results (newest first) -- confirms whether each tunnel's OWN
// configured Level 2 backup gateway is actually reachable.
func (c *TunnelHealthController) ListBackupProbeResults(ctx echo.Context) error {
	if forbidden, err := forbidUnlessAdmin(ctx); forbidden {
		return err
	}

	limit := 100
	if raw := ctx.QueryParam("limit"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil {
			limit = parsed
		}
	}

	results, err := c.tunnelHealthService.ListBackupProbeResults(limit)
	if err != nil {
		return ctx.JSON(http.StatusInternalServerError, schema.InternalServerErrorResponse)
	}

	resp := make([]schema.TunnelBackupProbeResultResponse, 0, len(results))
	for _, r := range results {
		resp = append(resp, schema.TunnelBackupProbeResultResponse{
			Id:              r.ID,
			InterfaceName:   r.InterfaceName,
			BackupGatewayIP: r.BackupGatewayIP,
			Reachable:       r.Reachable,
			Evidence:        r.Evidence,
			ProbedAt:        r.ProbedAt.Format(time.RFC3339),
		})
	}
	return ctx.JSON(http.StatusOK, schema.BasicResponseData[[]schema.TunnelBackupProbeResultResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data:          resp,
	})
}

func (c *TunnelHealthController) RemoveRedlineEntry(ctx echo.Context) error {
	if forbidden, err := forbidUnlessAdmin(ctx); forbidden {
		return err
	}

	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	if err := c.redline.RemoveEntry(uint(id)); err != nil {
		c.logger.Error("failed to remove redline entry", zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.InternalServerErrorResponse)
	}

	return ctx.JSON(http.StatusOK, schema.OkBasicResponse)
}
