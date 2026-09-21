// Thin Recharts wrappers themed off the app's CSS variables so charts follow
// the Notion-style tokens and the dark theme automatically. Pages compose
// these inside .chart-card layouts instead of configuring Recharts directly.

import { useEffect, useState, type ReactNode } from 'react'
import {
  Bar,
  BarChart,
  CartesianGrid,
  Cell,
  LabelList,
  Line,
  LineChart,
  Legend,
  Pie,
  PieChart,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from 'recharts'
import { useSession } from '../hooks/session'

export type ChartColor = 'accent' | 'green' | 'orange' | 'red' | 'muted'

type ChartColors = {
  accent: string
  green: string
  orange: string
  red: string
  muted: string
  teal: string
  blue: string
  line: string
  surface: string
  text: string
  palette: string[]
}

const LIGHT_FALLBACK: ChartColors = {
  accent: '#883d92', green: '#1aae39', orange: '#dd5b00', red: '#d33', muted: '#6f6a64',
  teal: '#0e9488', blue: '#3b6fd4',
  line: 'rgba(0,0,0,0.09)', surface: '#ffffff', text: 'rgba(0,0,0,0.92)',
  palette: [],
}

function readChartColors(): ChartColors {
  const styles = getComputedStyle(document.documentElement)
  const read = (name: string, fallbackValue: string) => styles.getPropertyValue(name).trim() || fallbackValue
  const base = {
    accent: read('--accent', LIGHT_FALLBACK.accent),
    green: read('--green', LIGHT_FALLBACK.green),
    orange: read('--orange', LIGHT_FALLBACK.orange),
    red: read('--red', LIGHT_FALLBACK.red),
    muted: read('--muted', LIGHT_FALLBACK.muted),
    teal: LIGHT_FALLBACK.teal,
    blue: LIGHT_FALLBACK.blue,
    line: read('--line', LIGHT_FALLBACK.line),
    surface: read('--surface', LIGHT_FALLBACK.surface),
    text: read('--text', LIGHT_FALLBACK.text),
  }
  return { ...base, palette: [base.accent, base.green, base.orange, base.muted, base.red, base.teal, base.blue] }
}

// useChartTheme resolves CSS variables once per theme change; recharts needs
// concrete color strings, and presentation attributes alone would miss
// color-mix() tokens. The read is deferred a frame because child effects run
// before the SessionProvider's DOM update lands on <html data-theme>.
export function useChartTheme(): ChartColors {
  const { theme } = useSession()
  const [colors, setColors] = useState<ChartColors>(readChartColors)
  useEffect(() => {
    const raf = requestAnimationFrame(() => setColors(readChartColors()))
    return () => cancelAnimationFrame(raf)
  }, [theme])
  return colors
}

function colorOf(colors: ChartColors, color: ChartColor | number): string {
  if (typeof color === 'number') return colors.palette[color % colors.palette.length]
  return colors[color]
}

function tooltipProps(colors: ChartColors) {
  return {
    contentStyle: {
      background: colors.surface,
      border: `1px solid ${colors.line}`,
      borderRadius: 8,
      fontSize: 12,
      padding: '6px 10px',
      boxShadow: '0 4px 18px rgba(0,0,0,0.08)',
    },
    labelStyle: { color: colors.text, fontWeight: 600, marginBottom: 2 },
    cursor: { stroke: colors.line, strokeWidth: 1 },
  }
}

const tickStyle = (colors: ChartColors) => ({ fontSize: 11, fill: colors.muted })

export function shortDate(value: string) {
  return value.length >= 10 ? value.slice(5) : value
}

export function ChartCard({
  title,
  subtitle,
  height = 232,
  children,
}: {
  title: string
  subtitle?: string
  height?: number
  children: ReactNode
}) {
  return (
    <div className="chart-card">
      <h3>{title}</h3>
      {subtitle && <p className="muted chart-subtitle">{subtitle}</p>}
      <div style={{ height }}>{children}</div>
    </div>
  )
}

export type TrendSeries = { key: string; label: string; color: ChartColor | number }

export function TrendChart({
  data,
  xKey,
  series,
}: {
  data: Record<string, unknown>[]
  xKey: string
  series: TrendSeries[]
}) {
  const colors = useChartTheme()
  return (
    <ResponsiveContainer width="100%" height="100%">
      <LineChart data={data} margin={{ top: 8, right: 12, bottom: 0, left: -14 }}>
        <CartesianGrid stroke={colors.line} strokeDasharray="3 3" vertical={false} />
        <XAxis
          dataKey={xKey}
          tick={tickStyle(colors)}
          tickLine={false}
          axisLine={{ stroke: colors.line }}
          tickFormatter={shortDate}
          minTickGap={28}
        />
        <YAxis tick={tickStyle(colors)} tickLine={false} axisLine={false} allowDecimals={false} width={42} />
        <Tooltip {...tooltipProps(colors)} />
        {series.map((s) => (
          <Line
            key={s.key}
            type="monotone"
            dataKey={s.key}
            name={s.label}
            stroke={colorOf(colors, s.color)}
            strokeWidth={2}
            dot={false}
            activeDot={{ r: 4, strokeWidth: 0 }}
            fill={colorOf(colors, s.color)}
          />
        ))}
        <Legend wrapperStyle={{ fontSize: 12 }} iconType="plainline" iconSize={14} />
      </LineChart>
    </ResponsiveContainer>
  )
}

export function BarsChart({
  data,
  xKey,
  series,
  stacked = false,
  tickFormatter,
}: {
  data: Record<string, unknown>[]
  xKey: string
  series: TrendSeries[]
  stacked?: boolean
  tickFormatter?: (value: string) => string
}) {
  const colors = useChartTheme()
  return (
    <ResponsiveContainer width="100%" height="100%">
      <BarChart data={data} margin={{ top: 8, right: 12, bottom: 0, left: -14 }} barCategoryGap="22%">
        <CartesianGrid stroke={colors.line} strokeDasharray="3 3" vertical={false} />
        <XAxis
          dataKey={xKey}
          tick={tickStyle(colors)}
          tickLine={false}
          axisLine={{ stroke: colors.line }}
          tickFormatter={tickFormatter ?? shortDate}
          minTickGap={12}
          interval="preserveStartEnd"
        />
        <YAxis tick={tickStyle(colors)} tickLine={false} axisLine={false} allowDecimals={false} width={42} />
        <Tooltip {...tooltipProps(colors)} cursor={{ fill: colors.line, opacity: 0.35 }} />
        {series.map((s) => (
          <Bar
            key={s.key}
            dataKey={s.key}
            name={s.label}
            stackId={stacked ? 'a' : undefined}
            fill={colorOf(colors, s.color)}
            radius={stacked ? [0, 0, 0, 0] : [3, 3, 0, 0]}
            maxBarSize={26}
          />
        ))}
        {series.length > 1 && <Legend wrapperStyle={{ fontSize: 12 }} iconType="circle" iconSize={9} />}
      </BarChart>
    </ResponsiveContainer>
  )
}

// Horizontal ranked bars (accuracy by unit, top concepts, notes usage).
export function HBarsChart({
  data,
  labelKey,
  valueKey,
  color = 'accent',
  valueName,
  valueSuffix,
}: {
  data: Record<string, unknown>[]
  labelKey: string
  valueKey: string
  color?: ChartColor
  valueName?: string
  valueSuffix?: string
}) {
  const colors = useChartTheme()
  return (
    <ResponsiveContainer width="100%" height="100%">
      <BarChart data={data} layout="vertical" margin={{ top: 4, right: 34, bottom: 0, left: 8 }} barCategoryGap="26%">
        <XAxis type="number" tick={tickStyle(colors)} tickLine={false} axisLine={false} allowDecimals={false} />
        <YAxis
          type="category"
          dataKey={labelKey}
          tick={{ fontSize: 11, fill: colors.text }}
          tickLine={false}
          axisLine={false}
          width={150}
          tickFormatter={(value: string) => (value.length > 22 ? `${value.slice(0, 21)}…` : value)}
        />
        <Tooltip {...tooltipProps(colors)} cursor={{ fill: colors.line, opacity: 0.35 }} />
        <Bar dataKey={valueKey} name={valueName ?? valueKey} fill={colorOf(colors, color)} radius={[0, 3, 3, 0]} maxBarSize={16}>
          {valueSuffix && (
            <LabelList
              dataKey={valueKey}
              position="right"
              style={{ fill: colors.muted, fontSize: 11 }}
              formatter={(value: unknown) => `${value ?? 0}${valueSuffix}`}
            />
          )}
        </Bar>
      </BarChart>
    </ResponsiveContainer>
  )
}

export function DoughnutChart({
  data,
  centerLabel,
  centerValue,
}: {
  data: { name: string; value: number; color: string }[]
  centerLabel?: string
  centerValue?: string | number
}) {
  const colors = useChartTheme()
  const total = data.reduce((sum, row) => sum + row.value, 0)
  if (total === 0) return null
  const center = (centerValue !== undefined || centerLabel) && (
    <div className="doughnut-center">
      <strong>{centerValue ?? total}</strong>
      {centerLabel && <small>{centerLabel}</small>}
    </div>
  )
  const legend = (
    <div className="doughnut-legend">
      {data.map((row) => (
        <span key={row.name}>
          <i style={{ background: row.color }} />
          {row.name}
          <b>{row.value}</b>
        </span>
      ))}
    </div>
  )
  if (data.length === 1) {
    // A lone 100% slice is emitted by recharts as a ~360° arc whose
    // endpoints nearly coincide — a degenerate SVG path browsers rasterize
    // as nothing. Draw the ring directly instead. Radii mirror the Pie
    // proportions (inner 64% / outer 88% of half the viewBox).
    return (
      <div className="doughnut-wrap">
        <div className="doughnut-plot">
          <svg width="100%" height="100%" viewBox="0 0 100 100" aria-hidden="true">
            <circle cx="50" cy="50" r="38" fill="none" stroke={data[0].color} strokeWidth="13" />
          </svg>
          {center}
        </div>
        {legend}
      </div>
    )
  }
  return (
    <div className="doughnut-wrap">
      <div className="doughnut-plot">
        <ResponsiveContainer width="100%" height="100%">
          <PieChart>
            <Pie
              data={data}
              dataKey="value"
              nameKey="name"
              innerRadius="64%"
              outerRadius="88%"
              paddingAngle={2}
              stroke="none"
            >
              {data.map((row) => (
                <Cell key={row.name} fill={row.color} />
              ))}
            </Pie>
            <Tooltip {...tooltipProps(colors)} />
          </PieChart>
        </ResponsiveContainer>
        {center}
      </div>
      {legend}
    </div>
  )
}
