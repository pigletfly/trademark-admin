import { createContext, useContext } from 'react'

export type Direction = 'ltr' | 'rtl'

export type DirectionContextType = {
  defaultDir: Direction
  dir: Direction
  setDir: (dir: Direction) => void
  resetDir: () => void
}

export const DirectionContext = createContext<DirectionContextType | null>(null)

export function useDirection() {
  const context = useContext(DirectionContext)
  if (!context) {
    throw new Error('useDirection must be used within a DirectionProvider')
  }
  return context
}
