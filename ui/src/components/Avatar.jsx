import { avatarColor, initials } from '../utils'

const SIZES = {
  xs: 'w-6 h-6 text-[10px]',
  sm: 'w-8 h-8 text-xs',
  md: 'w-10 h-10 text-sm',
  lg: 'w-14 h-14 text-base',
}

export function Avatar({ username, size = 'md' }) {
  return (
    <div
      className={`${SIZES[size] ?? SIZES.md} rounded-full flex items-center justify-center font-semibold text-white shrink-0`}
      style={{ background: avatarColor(username || '') }}
    >
      {initials(username)}
    </div>
  )
}
