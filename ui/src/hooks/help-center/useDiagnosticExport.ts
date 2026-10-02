import { useState } from 'react'
import { toast } from 'sonner'
import { useDeviceDataQuery } from '@/hooks/dashboard/useDeviceDataQuery.ts'
import { useLicenseStatusQuery } from '@/hooks/license/useLicenseStatusQuery.ts'
import { fetchLogs } from '@/api/logs.ts'

// One-click diagnostic bundle: server hardware stats, license status,
// panel version, and the last 100 log entries, assembled entirely
// client-side from data already reachable by an admin session (no new
// backend export endpoint) and packaged as a JSON file -- so support can
// see exactly what's wrong without a back-and-forth of "what version are
// you on" / "send me your logs." Extracted from DiagnosticExportButton so
// both the desktop inline button and the mobile action-menu item can
// trigger the same export without duplicating this logic.
export function useDiagnosticExport(onExported: (file: File) => void) {
  const [isExporting, setIsExporting] = useState(false)
  const { data: deviceData } = useDeviceDataQuery()
  const { data: license } = useLicenseStatusQuery()

  const runExport = async () => {
    setIsExporting(true)
    try {
      const logs = await fetchLogs({ limit: 100 })

      const bundle = {
        exported_at: new Date().toISOString(),
        panel_version: __APP_VERSION__,
        license: license
          ? {
              activated: license.activated,
              valid: license.valid,
              plan_name: license.plan_name,
              expires_at: license.expires_at,
              server_count: license.server_count,
              max_servers: license.max_servers,
              in_grace_period: license.in_grace_period,
            }
          : null,
        server: deviceData
          ? {
              identity: deviceData.DeviceIdentity?.identity,
              board_name: deviceData.DeviceInfo?.board_name,
              os_version: deviceData.DeviceInfo?.os_version,
              cpu_arch: deviceData.DeviceInfo?.cpu_arch,
              uptime: deviceData.DeviceInfo?.uptime,
              cpu_load: deviceData.DeviceInfo?.cpu_load,
              total_memory: deviceData.DeviceInfo?.total_memory,
              free_memory: deviceData.DeviceInfo?.free_memory,
              total_disk: deviceData.DeviceInfo?.total_disk,
              free_disk: deviceData.DeviceInfo?.free_disk,
              ipv4: deviceData.DeviceIPv4Address?.ipv4,
            }
          : null,
        stats: deviceData
          ? {
              servers: deviceData.ServerInfo,
              interfaces: deviceData.InterfaceInfo,
              peers: deviceData.PeerInfo,
              traffic: deviceData.TrafficInfo,
            }
          : null,
        recent_logs: logs.entries,
      }

      const jsonContent = JSON.stringify(bundle, null, 2)
      const file = new File(
        [jsonContent],
        `mwp-diagnostic-${new Date().toISOString().replace(/[:.]/g, '-')}.json`,
        { type: 'application/json' }
      )
      onExported(file)
      toast.success('گزارش تشخیصی پیوست شد')
    } catch {
      toast.error('ساخت گزارش تشخیصی ناموفق بود')
    } finally {
      setIsExporting(false)
    }
  }

  return { runExport, isExporting }
}
