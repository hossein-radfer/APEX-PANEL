import {
  RadialBar,
  RadialBarChart,
  PolarAngleAxis,
  ResponsiveContainer,
} from 'recharts'

interface ResourceGaugeProps {
  label: string
  percent: number | null
  detail?: string
  colorClassName?: string
}

// Small circular-progress gauge for a single hardware metric (CPU/Memory/
// Disk). percent === null renders a flat "N/A" ring instead of a 0% gauge,
// since those mean different things (unknown vs. genuinely idle).
export function ResourceGauge({
  label,
  percent,
  detail,
  colorClassName = 'fill-primary',
}: ResourceGaugeProps) {
  const clamped = percent === null ? 0 : Math.max(0, Math.min(100, percent))
  const data = [{ value: clamped, fill: 'currentColor' }]

  return (
    <div className='flex flex-col items-center gap-1'>
      <div
        className={`relative size-20 sm:size-24 ${colorClassName}`}
      >
        {/* recharts' ResponsiveContainer measures its direct parent, which
            must have a real non-zero size -- the size-20/size-24 classes on
            this wrapping div supply that (see PeersChart/TrafficChart's own
            ChartContainer usage for the same requirement). Fixes this gauge
            being permanently locked to a hardcoded 96x96px regardless of
            viewport, the one dashboard chart with zero responsive sizing. */}
        <ResponsiveContainer width='100%' height='100%'>
          <RadialBarChart
            cx='50%'
            cy='50%'
            innerRadius='72%'
            outerRadius='100%'
            barSize={8}
            data={data}
            startAngle={90}
            endAngle={-270}
          >
            <PolarAngleAxis
              type='number'
              domain={[0, 100]}
              angleAxisId={0}
              tick={false}
            />
            <RadialBar
              background={{ className: 'fill-muted' }}
              dataKey='value'
              cornerRadius={8}
              className={colorClassName}
            />
          </RadialBarChart>
        </ResponsiveContainer>
        <div className='absolute inset-0 flex items-center justify-center'>
          <span className='text-foreground text-sm font-semibold'>
            {percent === null ? 'نامشخص' : `${Math.round(clamped)}%`}
          </span>
        </div>
      </div>
      <span className='text-muted-foreground text-xs font-medium'>{label}</span>
      {detail && (
        <span className='text-muted-foreground/80 text-[11px]'>{detail}</span>
      )}
    </div>
  )
}
