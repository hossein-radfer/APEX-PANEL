import { useMemo, useState } from 'react'

export interface MapPoint {
  key: string
  lat: number
  lon: number
  label: string
  sublabel?: string
}

interface Props {
  points: MapPoint[]
}

// Equirectangular projection -- lon in [-180, 180] maps to x in [0, W],
// lat in [-90, 90] maps to y in [0, H] (inverted, since SVG y grows
// downward). No external tile service, no map library: a fully
// self-contained, hand-authored world silhouette (see CONTINENTS below --
// coarse, ~country-level accuracy, not coastline-traced from any dataset,
// so it carries no licensing baggage) plus a light graticule underneath and
// pin-style markers on top. Replaces an earlier version that was JUST the
// graticule with plain circles -- a confirmed, reported complaint: with no
// landmass drawn at all, it read as "a rectangle with a grid and a dot,"
// not a map, since nothing on screen conveyed where the point actually was.
const MAP_WIDTH = 720
const MAP_HEIGHT = 360

function project(lat: number, lon: number): { x: number; y: number } {
  const x = ((lon + 180) / 360) * MAP_WIDTH
  const y = ((90 - lat) / 180) * MAP_HEIGHT
  return { x, y }
}

// Coarse continent silhouettes in this component's own 720x360
// equirectangular pixel space (x=0 -> lon -180, y=0 -> lat +90) --
// hand-authored from generic geographic reference points (capes, gulfs,
// peninsulas), not traced from any copyrighted or licensed dataset, kept
// deliberately low-vertex since this widget only needs to read as "world
// map" at a glance, not survive zooming in.
const CONTINENTS = [
  'M36,64L56,60L90,66L104,78L112,88L124,114L136,126L148,140L162,148L176,152L192,162L202,163.4L206,164L202,161L194,157L186,149L182,147L174,143L166,136L166,128L180,122L192,120L198,130L200,116L210,110L212,100L220,96L226,90L232,88L240,86L248,78L236,64L206,56L190,46L170,42L140,38L104,40L78,40L48,38L30,48L36,64Z',
  'M206,164L216,158L238,158L260,180L290,190L284,206L274,226L264,234L246,248L236,260L230,270L224,284L220,290L216,284L214,270L218,246L220,216L220,190L202,178L206,164Z',
  'M326,150L326,138L342,116L380,106L410,116L430,118L446,156L462,156L440,184L430,216L426,234L416,246L396,248L384,216L384,192L378,180L378,170L364,168L344,170L328,156L326,150Z',
  'M342,94L342,104L348,108L366,106L374,94L384,90L392,100L406,104L418,98L430,108L430,118L456,120L472,126L482,130L496,136L506,150L514,164L520,154L534,136L548,148L556,164L568,178L578,156L576,138L580,140L604,120L602,116L608,100L614,104L618,108L620,96L628,86L640,78L650,70L672,60L686,66L696,50L720,44L700,40L640,34L580,28L530,24L480,36L450,46L420,40L410,38L388,44L370,58L382,66L376,70L368,78L356,82L350,84L342,94Z',
  'M270,14L320,16L320,28L310,40L290,48L270,60L256,58L248,50L250,40L240,28L230,20L240,16L270,14Z',
  'M350,64L354,63L358,66L360,74L363,75L361,78L356,80L350,80L348,76L344,72L348,70L348,66L350,64Z',
  'M620,116L626,112L634,110L640,107L644,100L642,94L636,100L632,108L624,112L620,116Z',
  'M586,224L604,216L618,210L624,204L632,204L642,206L644,202L648,208L652,218L660,226L666,236L662,248L660,254L654,256L640,256L632,250L624,244L608,246L590,248L588,238L586,224Z',
  'M706,248L710,254L716,256L714,259L716,263L712,262L708,262L705,260L706,255L706,248Z',
  'M0,310L80,320L160,312L240,320L320,312L400,320L480,312L560,320L640,312L720,310L720,360L0,360L0,310Z',
]

export function SecurityWorldMap({ points }: Props) {
  const [hovered, setHovered] = useState<string | null>(null)

  const projected = useMemo(
    () =>
      points
        .filter((p) => Number.isFinite(p.lat) && Number.isFinite(p.lon))
        .map((p) => ({ ...p, ...project(p.lat, p.lon) })),
    [points]
  )

  const latLines = [-60, -30, 0, 30, 60]
  const lonLines = [-150, -120, -90, -60, -30, 0, 30, 60, 90, 120, 150]

  return (
    <div className='w-full overflow-x-auto'>
      <svg
        viewBox={`0 0 ${MAP_WIDTH} ${MAP_HEIGHT}`}
        role='img'
        aria-label='نقشه‌ی موقعیت جغرافیایی IP‌های متصل'
        className='w-full min-w-[280px] rounded-lg border'
        style={{ background: 'color-mix(in oklab, var(--chart-1) 6%, var(--muted))' }}
      >
        {/* Graticule (lat/lon grid), kept subtle -- a background reference,
            not the main content anymore. */}
        {lonLines.map((lon) => {
          const { x } = project(0, lon)
          return (
            <line
              key={`lon-${lon}`}
              x1={x}
              y1={0}
              x2={x}
              y2={MAP_HEIGHT}
              stroke='currentColor'
              strokeOpacity={0.06}
              strokeWidth={1}
            />
          )
        })}
        {latLines.map((lat) => {
          const { y } = project(lat, 0)
          return (
            <line
              key={`lat-${lat}`}
              x1={0}
              y1={y}
              x2={MAP_WIDTH}
              y2={y}
              stroke='currentColor'
              strokeOpacity={0.06}
              strokeWidth={1}
            />
          )
        })}

        {/* Landmasses -- what actually makes this legible as a world map. */}
        <g
          fill='color-mix(in oklab, var(--chart-1) 28%, var(--muted-foreground))'
          fillOpacity={0.55}
          stroke='color-mix(in oklab, var(--chart-1) 40%, var(--muted-foreground))'
          strokeOpacity={0.7}
          strokeWidth={0.75}
        >
          {CONTINENTS.map((d, i) => (
            <path key={i} d={d} />
          ))}
        </g>

        <rect
          x={0.5}
          y={0.5}
          width={MAP_WIDTH - 1}
          height={MAP_HEIGHT - 1}
          fill='none'
          stroke='currentColor'
          strokeOpacity={0.16}
        />

        {/* Points -- a pin shape (teardrop + hole), not a bare circle, so
            each one reads unambiguously as "a location marker" rather than
            an arbitrary dot that could be anything. */}
        {projected.map((p) => {
          const isHovered = hovered === p.key
          const scale = isHovered ? 1.25 : 1
          return (
            <g
              key={p.key}
              onMouseEnter={() => setHovered(p.key)}
              onMouseLeave={() => setHovered(null)}
              className='cursor-pointer'
              style={{ transition: 'transform 0.15s ease' }}
              transform={`translate(${p.x} ${p.y}) scale(${scale})`}
            >
              <path
                d='M0,-14 C6,-14 10,-9.5 10,-4.5 C10,2 0,12 0,12 C0,12 -10,2 -10,-4.5 C-10,-9.5 -6,-14 0,-14 Z'
                fill='var(--chart-1)'
                stroke='var(--background)'
                strokeWidth={1.5}
              />
              <circle cx={0} cy={-4.5} r={3.5} fill='var(--background)' />

              {isHovered && (
                <g transform={`scale(${1 / scale})`}>
                  <rect
                    x={Math.min(Math.max(-70, 4 - p.x), MAP_WIDTH - 144 - p.x)}
                    y={-14 * scale - 46}
                    width={140}
                    height={p.sublabel ? 40 : 24}
                    rx={6}
                    fill='var(--popover)'
                    stroke='currentColor'
                    strokeOpacity={0.15}
                  />
                  <text
                    x={Math.min(Math.max(0, 74 - p.x), MAP_WIDTH - 74 - p.x)}
                    y={-14 * scale - 46 + 16}
                    textAnchor='middle'
                    fontSize={11}
                    fill='var(--popover-foreground)'
                    fontWeight={600}
                  >
                    {p.label}
                  </text>
                  {p.sublabel && (
                    <text
                      x={Math.min(Math.max(0, 74 - p.x), MAP_WIDTH - 74 - p.x)}
                      y={-14 * scale - 46 + 32}
                      textAnchor='middle'
                      fontSize={10}
                      fill='var(--muted-foreground)'
                    >
                      {p.sublabel}
                    </text>
                  )}
                </g>
              )}
            </g>
          )
        })}
      </svg>
    </div>
  )
}
