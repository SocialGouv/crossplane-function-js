// was unable to output literal multiline using mikefarah/yq, see https://github.com/mikefarah/yq/issues/1575
import path from 'path'
import fs from 'fs'
import YAML from 'yaml'

const compositionDir = path.join(process.cwd(), "tests/fixtures")
const file = fs.readFileSync(path.join(compositionDir, 'composition.yaml'), 'utf8')
const manifest = YAML.parse(file)
const inline = fs.readFileSync(path.join(compositionDir, 'composition.fn.ts'), { endoding: 'utf8' })
manifest.spec.pipeline[0].input.spec.source.inline = inline.toString()
process.stdout.write(YAML.stringify(manifest))