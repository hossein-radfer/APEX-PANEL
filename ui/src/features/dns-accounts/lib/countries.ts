// Static list of common English country names for the allowed-countries
// geo-fence picker -- MWPanel has no structured country reference table
// anywhere in the codebase (see v2ray-sub/lib/country-flag.ts's own doc
// comment: only free-text sale titles exist), so this plain static list
// mirrors that same "no backend country table" precedent. Names match the
// English country names the backend's AllowedCountries field expects
// (CreateDNSAccountRequest.AllowedCountries doc comment).
export const COMMON_COUNTRIES: string[] = [
  'Iran',
  'United States',
  'United Kingdom',
  'Germany',
  'France',
  'Netherlands',
  'Turkey',
  'Canada',
  'Finland',
  'Sweden',
  'Switzerland',
  'Austria',
  'Italy',
  'Spain',
  'Poland',
  'Russia',
  'United Arab Emirates',
  'India',
  'Japan',
  'South Korea',
  'Singapore',
  'Hong Kong',
  'Australia',
  'Brazil',
  'China',
]
