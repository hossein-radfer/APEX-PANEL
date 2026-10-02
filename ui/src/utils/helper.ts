import { DeviceData } from '@/schema/dashboard.ts'

function getValueOrNA(value: string | number | undefined | null): string {
  if (value === undefined || value === null || value === '') return 'N/A'
  return String(value)
}

function formatGB(bytes: string | number | undefined): string | null {
  const num = Number(bytes)
  return isNaN(num) ? null : (num / 1_000_000_000).toFixed(2)
}

export function buildDeviceStats(
  stats: DeviceData['DeviceInfo'] | undefined
): { label: string; value: string }[] {
  const uptime = getValueOrNA(stats?.uptime)
  const cpuLoad = stats?.cpu_load != null ? `${stats.cpu_load}%` : 'N/A'

  const memoryUsed = formatGB(
    Number(stats?.total_memory) - Number(stats?.free_memory)
  )
  const memoryTotal = formatGB(stats?.total_memory)
  const memoryValue =
    memoryUsed && memoryTotal ? `${memoryUsed}/${memoryTotal} GB` : 'N/A'

  const diskUsed = formatGB(
    Number(stats?.total_disk) - Number(stats?.free_disk)
  )
  const diskTotal = formatGB(stats?.total_disk)
  const diskValue =
    diskUsed && diskTotal ? `${diskUsed}/${diskTotal} GB` : 'N/A'

  return [
    { label: 'Uptime', value: uptime },
    { label: 'CPU Load', value: cpuLoad },
    { label: 'Memory Usage', value: memoryValue },
    { label: 'Disk Usage', value: diskValue },
  ]
}

export interface DeviceResourceGauge {
  label: string
  percent: number | null
  detail: string | null
}

// Raw numeric ratios for the dashboard's circular-progress gauges --
// buildDeviceStats above only produces pre-formatted display strings, which
// throws away the used/total numbers a gauge needs to render its ring.
export function buildDeviceResourceGauges(
  stats: DeviceData['DeviceInfo'] | undefined
): {
  cpu: DeviceResourceGauge
  memory: DeviceResourceGauge
  disk: DeviceResourceGauge
} {
  const cpuLoad = stats?.cpu_load != null ? Number(stats.cpu_load) : NaN

  const totalMemory = Number(stats?.total_memory)
  const freeMemory = Number(stats?.free_memory)
  const memoryUsedBytes = totalMemory - freeMemory
  const memoryPercent =
    isFinite(totalMemory) && totalMemory > 0
      ? (memoryUsedBytes / totalMemory) * 100
      : NaN
  const memoryUsedGB = formatGB(memoryUsedBytes)
  const memoryTotalGB = formatGB(stats?.total_memory)

  const totalDisk = Number(stats?.total_disk)
  const freeDisk = Number(stats?.free_disk)
  const diskUsedBytes = totalDisk - freeDisk
  const diskPercent =
    isFinite(totalDisk) && totalDisk > 0
      ? (diskUsedBytes / totalDisk) * 100
      : NaN
  const diskUsedGB = formatGB(diskUsedBytes)
  const diskTotalGB = formatGB(stats?.total_disk)

  return {
    cpu: {
      label: 'پردازنده',
      percent: isNaN(cpuLoad) ? null : cpuLoad,
      detail: isNaN(cpuLoad) ? null : 'بار پردازش',
    },
    memory: {
      label: 'حافظه',
      percent: isNaN(memoryPercent) ? null : memoryPercent,
      detail:
        memoryUsedGB && memoryTotalGB
          ? `${memoryUsedGB} / ${memoryTotalGB} گیگابایت`
          : null,
    },
    disk: {
      label: 'دیسک',
      percent: isNaN(diskPercent) ? null : diskPercent,
      detail: diskUsedGB && diskTotalGB ? `${diskUsedGB} / ${diskTotalGB} گیگابایت` : null,
    },
  }
}

export function getAvatarInitials(name: string | undefined): string {
  if (!name) return ''

  const cleaned = name.trim().replace(/\s+/g, ' ')
  if (!cleaned) return ''

  const words = cleaned.split(' ')

  if (words.length >= 2) {
    return (words[0][0] + words[1][0]).toUpperCase()
  }

  const single = words[0]
  return single.slice(0, 2).toUpperCase()
}
