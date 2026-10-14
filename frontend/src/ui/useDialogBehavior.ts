import { useEffect, useRef, type MouseEvent } from 'react'

const focusableSelector =
  'a[href], button:not([disabled]), input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])'

// Shared by Modal and Drawer. While `active`: focuses the panel, closes on Escape, keeps Tab inside the
// panel, locks the page scroll, and on deactivation restores the scroll and the previously focused element.
export function useDialogBehavior(onClose: () => void, active = true) {
  const panelRef = useRef<HTMLDivElement>(null)
  const onCloseRef = useRef(onClose)

  useEffect(() => {
    onCloseRef.current = onClose
  })

  useEffect(() => {
    if (!active) return
    const panel = panelRef.current
    const previouslyFocused = document.activeElement instanceof HTMLElement ? document.activeElement : null
    const previousOverflow = document.body.style.overflow
    document.body.style.overflow = 'hidden'
    panel?.focus()

    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') {
        onCloseRef.current()
        return
      }
      if (event.key !== 'Tab' || panel === null) return
      const items = Array.from(panel.querySelectorAll<HTMLElement>(focusableSelector))
      const first = items[0]
      const last = items[items.length - 1]
      if (first === undefined || last === undefined) {
        event.preventDefault()
        panel.focus()
        return
      }
      const current = document.activeElement
      if (event.shiftKey && (current === first || current === panel || !panel.contains(current))) {
        event.preventDefault()
        last.focus()
      } else if (!event.shiftKey && (current === last || !panel.contains(current))) {
        event.preventDefault()
        first.focus()
      }
    }
    document.addEventListener('keydown', onKeyDown)
    return () => {
      document.removeEventListener('keydown', onKeyDown)
      document.body.style.overflow = previousOverflow
      if (previouslyFocused?.isConnected) previouslyFocused.focus()
    }
  }, [active])

  return panelRef
}

// The backdrop closes the dialog only when the press AND the click both land on it, so dragging a text
// selection from inside the panel and releasing over the backdrop does not close it.
export function useBackdropClose(onClose: () => void) {
  const pressedOnBackdrop = useRef(false)
  return {
    onMouseDown: (event: MouseEvent<HTMLElement>) => {
      pressedOnBackdrop.current = event.target === event.currentTarget
    },
    onClick: (event: MouseEvent<HTMLElement>) => {
      const close = pressedOnBackdrop.current && event.target === event.currentTarget
      pressedOnBackdrop.current = false
      if (close) onClose()
    },
  }
}
