#!/usr/bin/env node
// Generates avatar SVG files into public/avatars/ at build time.
import { createAvatar } from '../node_modules/@dicebear/core/lib/index.js'
import { notionists } from '../node_modules/@dicebear/collection/lib/index.js'
import { writeFileSync, mkdirSync } from 'fs'
import { join, dirname } from 'path'
import { fileURLToPath } from 'url'

const __dirname = dirname(fileURLToPath(import.meta.url))
const outDir = join(__dirname, '../public/avatars')
mkdirSync(outDir, { recursive: true })

const BG_COLORS = [
  'b6e3f4', 'c0aede', 'd1d4f9', 'ffd5dc', 'ffdfbf',
  'c7f2a4', 'fde68a', 'a7f3d0', 'ddd6fe', 'fecaca',
  'bae6fd', 'fbcfe8', 'fed7aa', 'bbf7d0', 'e0e7ff', 'fef9c3',
]

const SEEDS = [
  'Aneka','Bailey','Casey','Dakota','Eden','Finley',
  'Gray','Harper','Indigo','Jordan','Kai','Lane',
  'Morgan','Nova','Ocean','Quinn',
]

SEEDS.forEach((seed, i) => {
  const id = `p${String(i + 1).padStart(2, '0')}`
  let svg = createAvatar(notionists, {
    seed,
    size: 200,
    backgroundColor: [BG_COLORS[i]],
    backgroundType: ['gradientLinear'],
  }).toString()
  // make SVG scale with container
  svg = svg.replace(/width="\d+"/, 'width="100%"').replace(/height="\d+"/, 'height="100%"')
  writeFileSync(join(outDir, `${id}.svg`), svg)
  console.log(`✓ ${id}.svg (${seed})`)
})

console.log(`\n✅ ${SEEDS.length} avatars → public/avatars/`)
