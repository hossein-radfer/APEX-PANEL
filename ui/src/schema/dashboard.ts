import { z } from 'zod'
import { createApiResponseSchema } from '@/schema/api-response.ts'

const recentOnlinePeerSchema = z.object({
  name: z.string(),
  last_seen: z.string(),
})

// Every section below is nullable: GetDeviceData (api/service/device_data.go)
// fetches each one independently and returns null for exactly the ones that
// failed (e.g. a Mikrotik router that's unreachable on a multi-server setup)
// rather than failing the whole request -- a confirmed, reported bug this
// mirrors on the frontend side: this schema previously required every
// section as non-nullable, so a single missing section made the ENTIRE
// response fail Zod parsing, throwing deviceDataResponseSchema.parse(data)
// inside fetchDeviceData and putting the whole query in an error state --
// every stats card on the dashboard rendered blank (StatsCard has no
// error/empty state, only isLoading vs. a value), not just the one section
// that actually had a problem. Consumers must handle each section being
// null and show an explicit "no data" state instead of rendering nothing.
export const deviceDataSchema = z.object({
  ServerInfo: z
    .object({
      total_servers: z.number(),
      active_servers: z.number(),
    })
    .nullable(),
  InterfaceInfo: z
    .object({
      total_interfaces: z.number(),
      active_interfaces: z.number(),
    })
    .nullable(),
  PeerInfo: z
    .object({
      recent_online_peers: z.nullable(z.array(recentOnlinePeerSchema)),
      total_peers: z.number(),
      online_peers: z.number(),
      offline_peers: z.number(),
      disabled_peers: z.number(),
    })
    .nullable(),
  TrafficInfo: z
    .object({
      total_usage: z.string(),
      wireguard_usage: z.string().optional(),
      user_manager_usage: z.string().optional(),
    })
    .nullable(),
  DeviceIdentity: z
    .object({
      identity: z.string(),
    })
    .nullable(),
  DeviceInfo: z
    .object({
      board_name: z.string(),
      os_version: z.string(),
      cpu_arch: z.string(),
      uptime: z.string(),
      cpu_load: z.string(),
      total_memory: z.string(),
      free_memory: z.string(),
      total_disk: z.string(),
      free_disk: z.string(),
    })
    .nullable(),
  DeviceIPv4Address: z.nullable(
    z.object({
      ipv4: z.string().optional(),
      isp: z.string().optional(),
    })
  ),
  DNSConfig: z.nullable(
    z.object({
      dns_servers: z.string(),
    })
  ),
})

export const trafficUsageSchema = z.object({
  interface_id: z.number(),
  date: z.string(),
  download: z.string(),
  upload: z.string(),
  total: z.string(),
})

export const dailyTrafficUsageSchema = z.array(trafficUsageSchema).nullable()

export const deviceDataResponseSchema =
  createApiResponseSchema(deviceDataSchema)

export const dailyTrafficUsageResponseSchema = createApiResponseSchema(
  dailyTrafficUsageSchema
)

export type DeviceData = z.infer<typeof deviceDataSchema>
export type DailyTrafficUsage = z.infer<typeof dailyTrafficUsageSchema>
