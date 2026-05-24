export interface AvatarPreset {
  id: string
  label: string
  url: string
}

const SEEDS = [
  'Aneka', 'Bailey', 'Casey', 'Dakota', 'Eden', 'Finley',
  'Gray', 'Harper', 'Indigo', 'Jordan', 'Kai', 'Lane',
  'Morgan', 'Nova', 'Ocean', 'Quinn',
]

export const AVATAR_PRESETS: AvatarPreset[] = SEEDS.map((seed, i) => {
  const id = `p${String(i + 1).padStart(2, '0')}`
  return { id, label: seed, url: `/avatars/${id}.svg` }
})

export const PRESET_PREFIX = 'preset:'

export function isPreset(avatarUrl: string): boolean {
  return !!avatarUrl?.startsWith(PRESET_PREFIX)
}

export function getPreset(avatarUrl: string): AvatarPreset | undefined {
  const id = avatarUrl?.slice(PRESET_PREFIX.length)
  return AVATAR_PRESETS.find(p => p.id === id)
}

export function toPresetUrl(id: string): string {
  return `${PRESET_PREFIX}${id}`
}
