'use client'

/**
 * Uses the actual brand PNG with CSS blend-modes so the white background
 * disappears on any theme:
 *   dark mode  → invert(1) + mix-blend-mode:screen  (dark logo becomes white, white bg → transparent)
 *   light mode → mix-blend-mode:multiply             (white bg → transparent on light surface)
 *
 * Plain <img> on purpose: the app is a static export with images.unoptimized,
 * so next/image would add no optimization here, only default lazy-loading on
 * an above-the-fold header logo.
 */
export function LogoMark({ size = 22 }: { size?: number }) {
  return (
    <>
      {/* dark mode version */}
      {/* eslint-disable-next-line @next/next/no-img-element -- see note above */}
      <img
        src="/logo.png"
        width={size}
        height={size}
        alt="InfraCanvas"
        className="logo-dark"
        style={{ objectFit: 'contain', display: 'block' }}
      />
      {/* light mode version */}
      {/* eslint-disable-next-line @next/next/no-img-element -- see note above */}
      <img
        src="/logo.png"
        width={size}
        height={size}
        alt=""
        className="logo-light"
        style={{ objectFit: 'contain', display: 'block' }}
      />
    </>
  )
}
