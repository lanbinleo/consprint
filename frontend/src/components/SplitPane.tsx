import { useRef, useState, type ReactNode } from 'react'

// Two-pane layout with a pointer-drag resize handle, extracted from the
// Terms page column resizing. Panes scroll independently; on narrow screens
// (media query) the panes stack vertically and the handle hides.
export function SplitPane({
  left,
  right,
  initialLeft = 420,
  minLeft = 260,
  maxLeft = 760,
  leftLabel,
  rightLabel,
}: {
  left: ReactNode
  right: ReactNode
  initialLeft?: number
  minLeft?: number
  maxLeft?: number
  leftLabel?: string
  rightLabel?: string
}) {
  const [leftWidth, setLeftWidth] = useState(initialLeft)
  const handleRef = useRef<HTMLDivElement>(null)

  function startResize(event: React.PointerEvent<HTMLDivElement>) {
    event.preventDefault()
    const handle = handleRef.current
    if (!handle) return
    const startX = event.clientX
    const startWidth = leftWidth
    handle.setPointerCapture(event.pointerId)
    const onMove = (move: PointerEvent) => {
      // RTL layouts would flip the drag axis; this app is LTR-only.
      const width = Math.round(Math.min(Math.max(startWidth + (move.clientX - startX), minLeft), maxLeft))
      setLeftWidth(width)
    }
    const onUp = () => {
      handle.removeEventListener('pointermove', onMove)
      handle.removeEventListener('pointerup', onUp)
      handle.removeEventListener('pointercancel', onUp)
    }
    handle.addEventListener('pointermove', onMove)
    handle.addEventListener('pointerup', onUp)
    handle.addEventListener('pointercancel', onUp)
  }

  return (
    <div className="split-pane" style={{ '--pane-left': `${leftWidth}px` } as React.CSSProperties}>
      <div className="pane pane-left" role="region" aria-label={leftLabel}>
        {left}
      </div>
      <div
        ref={handleRef}
        className="pane-handle"
        role="separator"
        aria-orientation="vertical"
        onPointerDown={startResize}
        onDoubleClick={() => setLeftWidth(initialLeft)}
      />
      <div className="pane pane-right" role="region" aria-label={rightLabel}>
        {right}
      </div>
    </div>
  )
}
