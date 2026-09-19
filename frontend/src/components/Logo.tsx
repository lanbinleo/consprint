import { useId } from 'react'

// The sidebar/brand mark is the purple AP block.
export function Logo({ size = 36 }: { size?: number }) {
  return <ApMark size={size} />
}

export function ApMark({ size = 36 }: { size?: number }) {
  // Unique gradient id per instance: the logo is rendered more than once
  // (sidebar + mobile top bar), and a duplicated id makes later references
  // resolve to the first (possibly display:none) definition and break.
  const gradientId = useId()
  return (
    <svg width={size} height={size} viewBox="0 0 48 48" fill="none" role="img" aria-label="AP Psychology Hub">
      <defs>
        <linearGradient id={gradientId} x1="0" y1="0" x2="48" y2="48" gradientUnits="userSpaceOnUse">
          <stop stopColor="#9a4aa6" />
          <stop offset="1" stopColor="#76337f" />
        </linearGradient>
      </defs>
      <rect width="48" height="48" rx="10" fill={`url(#${gradientId})`} />
      {/* College Board style mark: heavy condensed "AP" wordmark on a rounded block */}
      <text
        x="24"
        y="30"
        textAnchor="middle"
        fontFamily="'Inter', 'Segoe UI', -apple-system, sans-serif"
        fontSize="19.5"
        fontWeight="800"
        letterSpacing="-0.5"
        fill="#ffffff"
      >
        AP
      </text>
      <path d="M13.5 36h21" stroke="rgba(255,255,255,0.55)" strokeWidth="2.2" strokeLinecap="round" />
    </svg>
  )
}
