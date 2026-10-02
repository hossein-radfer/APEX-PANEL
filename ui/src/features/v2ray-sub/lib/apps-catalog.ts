import type { DetectedOS } from './detect-os'

export interface AppEntry {
  name: string
  description: string
  downloadUrl: string
  /** Deep-link URL builder -- receives the raw subscription URL, returns
   * the app-specific import URL (a bare custom scheme, e.g.
   * "v2rayng://install-sub?url=..."), or null if this app has no known
   * subscription-import deep-link scheme (Download is still offered).
   * Chrome for Android does not reliably navigate to a bare custom scheme
   * -- see androidPackageId below, which wraps this into an intent:// URL
   * on Android specifically. */
  buildImportUrl: ((subscriptionUrl: string) => string) | null
  /** This app's Android application ID (e.g. "com.v2ray.ang"), if it has
   * one -- required to build the intent:// wrapper Chrome for Android
   * needs to reliably honor a custom-scheme deep link (a confirmed,
   * well-documented Android/Chrome gotcha: a bare custom-scheme
   * navigation via a plain <a href> or window.location.href assignment
   * frequently silently no-ops in Chrome specifically, while other
   * Android browsers handle it fine). Omit for apps with no Android
   * build, or where buildImportUrl is null anyway. */
  androidPackageId?: string
  recommended?: boolean
}

export interface PlatformEntry {
  id: DetectedOS
  label: string
  apps: AppEntry[]
}

const encode = (url: string) => encodeURIComponent(url)

export const APPS_CATALOG: PlatformEntry[] = [
  {
    id: 'android',
    label: 'اندروید',
    apps: [
      {
        name: 'v2rayNG',
        description: 'محبوب‌ترین کلاینت V2Ray برای اندروید، رایگان و متن‌باز.',
        downloadUrl: 'https://github.com/2dust/v2rayNG/releases/latest',
        // "install-sub" (subscription import), NOT "install-config" (single
        // config import) -- a confirmed, reported bug: install-config is
        // meant for one raw vless://... string, not a subscription URL,
        // and has known parser truncation bugs on nested query strings
        // (2dust/v2rayNG#2859). install-sub is the app's own dedicated
        // subscription-URL host (2dust/v2rayNG#4141). No name= param --
        // multiple independent reports confirm it's unreliable as a query
        // param across app versions (2dust/v2rayNG#3470/#2901/#1658).
        buildImportUrl: (url) => `v2rayng://install-sub?url=${encode(url)}`,
        androidPackageId: 'com.v2ray.ang',
        recommended: true,
      },
      {
        name: 'v2Box',
        description: 'کلاینت مدرن با پشتیبانی از پروتکل‌های متعدد.',
        downloadUrl: 'https://play.google.com/store/apps/details?id=dev.hexasoftware.v2box',
        buildImportUrl: (url) => `v2box://install-sub?url=${encode(url)}`,
        androidPackageId: 'dev.hexasoftware.v2box',
      },
    ],
  },
  {
    id: 'ios',
    label: 'آی‌اواس',
    apps: [
      {
        name: 'v2Box',
        description: 'کلاینت رسمی برای آیفون و آیپد.',
        downloadUrl: 'https://apps.apple.com/app/v2box-v2ray-client/id6446814690',
        buildImportUrl: (url) => `v2box://install-sub?url=${encode(url)}`,
        recommended: true,
      },
      {
        name: 'Streisand',
        description: 'کلاینت رایگان و متن‌باز برای iOS.',
        downloadUrl: 'https://apps.apple.com/app/streisand/id6450534064',
        buildImportUrl: (url) => `streisand://import/${encode(url)}`,
      },
      {
        name: 'Shadowrocket',
        description: 'کلاینت پرکاربرد (پولی) با امکانات پیشرفته.',
        downloadUrl: 'https://apps.apple.com/app/shadowrocket/id932747118',
        buildImportUrl: (url) => `shadowrocket://add/sub://${btoa(url)}`,
      },
    ],
  },
  {
    id: 'windows',
    label: 'ویندوز',
    apps: [
      {
        name: 'v2rayN',
        description: 'کلاینت رایگان و پرکاربرد برای ویندوز.',
        downloadUrl: 'https://github.com/2dust/v2rayN/releases/latest',
        buildImportUrl: null,
        recommended: true,
      },
      {
        name: 'NekoRay',
        description: 'کلاینت گرافیکی با پشتیبانی از چند پروتکل.',
        downloadUrl: 'https://github.com/MatsuriDayo/nekoray/releases/latest',
        buildImportUrl: null,
      },
    ],
  },
  {
    id: 'macos',
    label: 'مک',
    apps: [
      {
        name: 'v2Box',
        description: 'کلاینت رسمی برای macOS.',
        downloadUrl: 'https://apps.apple.com/app/v2box-v2ray-client/id6446814690',
        buildImportUrl: (url) => `v2box://install-sub?url=${encode(url)}`,
        recommended: true,
      },
      {
        name: 'ClashX Meta',
        description: 'کلاینت گرافیکی محبوب برای مک.',
        downloadUrl: 'https://github.com/MetaCubeX/ClashX.Meta/releases/latest',
        buildImportUrl: null,
      },
    ],
  },
  {
    id: 'linux',
    label: 'لینوکس',
    apps: [
      {
        name: 'NekoRay',
        description: 'کلاینت گرافیکی رایگان برای لینوکس.',
        downloadUrl: 'https://github.com/MatsuriDayo/nekoray/releases/latest',
        buildImportUrl: null,
        recommended: true,
      },
      {
        name: 'v2rayA',
        description: 'کلاینت تحت وب برای لینوکس.',
        downloadUrl: 'https://github.com/v2rayA/v2rayA/releases/latest',
        buildImportUrl: null,
      },
    ],
  },
]
