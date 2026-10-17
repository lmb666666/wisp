/** Wisp 品牌标识：W 与一缕光。 */
export function AppLogo({ className = "size-6" }: { className?: string }) {
  return (
    <svg viewBox="0 0 64 64" className={className} aria-hidden="true">
      <rect width="64" height="64" rx="14" fill="#b85a2f" />
      <g transform="translate(-2.5 0)">
        <path
          d="M14 24.5c1.75 13.5 4.75 20.5 9.25 20.5 4.5 0 6.875-6.75 8.75-13.5 1.875 6.75 4.25 13.5 8.75 13.5 4.5 0 7.75-7.25 9.5-19.75.375-2.5 1-4.25 2.25-5.5"
          fill="none"
          strokeWidth="5.25"
          strokeLinecap="round"
          strokeLinejoin="round"
          stroke="#fff6e8"
        />
        <path
          d="M53 9c.55 2.5 1.5 3.45 4 4-2.5.55-3.45 1.5-4 4-.55-2.5-1.5-3.45-4-4 2.5-.55 3.45-1.5 4-4z"
          fill="#ffe0ad"
        />
      </g>
    </svg>
  )
}
