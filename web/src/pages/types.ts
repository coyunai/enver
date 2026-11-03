import type { Overview } from '../types'

export type View =
  | 'dashboard'
  | 'sdks'
  | 'presets'
  | 'venvs'
  | 'packages'
  | 'doctor'
  | 'terminal'
  | 'settings'

export type TerminalTarget = {
  preset?: string
  venv?: string
  cwd?: string
}

export type PageProps = {
  overview: Overview
  refresh: () => Promise<void>
  onNavigate: (view: View) => void
  onOpenTerminal: (target: TerminalTarget) => void
}
