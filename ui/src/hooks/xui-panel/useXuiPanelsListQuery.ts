import { useQuery } from '@tanstack/react-query'
import { fetchXuiPanelsList } from '@/api/xui-panel.ts'

// GET /api/xui-panel is admin-only server-side (see XuiPanelController.
// ListPanels -> forbidUnlessAdmin). `enabled` defaults to true so every
// existing admin-only call site keeps working unchanged; callers reachable
// by a reseller (e.g. V2RayForm, used on both the admin and reseller V2Ray
// Packages pages) MUST pass `enabled: false` for a reseller session, or
// this query 403s the instant it mounts -- and the app's global 403
// handler (see main.tsx's QueryCache.onError) redirects the ENTIRE page to
// /403 for any failed query, not just ones the user directly triggered.
// That was a confirmed, reported bug: a reseller opening "Add Package"
// was bounced to a full "Access Forbidden" page before ever submitting
// the form, purely because this background fetch ran unconditionally.
export const useXuiPanelsListQuery = (enabled = true) =>
  useQuery({
    queryKey: ['xui_panels_list'],
    queryFn: fetchXuiPanelsList,
    enabled,
  })
