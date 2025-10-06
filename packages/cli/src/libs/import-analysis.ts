import path from "path"

import fs from "fs-extra"
import * as ts from "typescript"

/**
 * Read and parse tsconfig, honoring "extends". Returns compiler options usable for module resolution.
 */
export function readTsCompilerOptions(tsconfigPath?: string) {
  if (!tsconfigPath) {
    const options = ts.getDefaultCompilerOptions()
    if (!options.moduleResolution) options.moduleResolution = ts.ModuleResolutionKind.NodeJs
    return { options, files: [] as string[] }
  }

  const configFile = ts.readJsonConfigFile(tsconfigPath, f => fs.readFileSync(f, "utf8"))
  const { options, fileNames, errors } = ts.parseJsonSourceFileConfigFileContent(
    configFile,
    ts.sys,
    path.dirname(tsconfigPath)
  )

  if (!options.moduleResolution) options.moduleResolution = ts.ModuleResolutionKind.NodeJs

  if (errors.length) {
    // Keep this utility logger-agnostic. Consumers can log if needed.
  }

  return { options, files: fileNames }
}

/**
 * Create a CompilerHost with normalized canonical file names (helps with path comparisons).
 */
export function createCompilerHost(options: ts.CompilerOptions) {
  const host = ts.createCompilerHost(options)
  const origGetCanonicalFileName = host.getCanonicalFileName.bind(host)
  host.getCanonicalFileName = f => path.normalize(origGetCanonicalFileName(f))
  return host
}

function isStringLiteralLike(node: ts.Node): node is ts.StringLiteralLike {
  return ts.isStringLiteral(node) || ts.isNoSubstitutionTemplateLiteral(node)
}

/**
 * Collect all module specifiers from a source file:
 *  - import ... from 'x'
 *  - export ... from 'x'
 *  - require('x')
 *  - import('x')
 */
function collectModuleSpecifiers(sf: ts.SourceFile): string[] {
  const specs: string[] = []

  function visit(node: ts.Node) {
    // import ... from 'x'
    if (
      ts.isImportDeclaration(node) &&
      node.moduleSpecifier &&
      isStringLiteralLike(node.moduleSpecifier)
    ) {
      specs.push(node.moduleSpecifier.text)
    }

    // export ... from 'x'
    if (
      ts.isExportDeclaration(node) &&
      node.moduleSpecifier &&
      isStringLiteralLike(node.moduleSpecifier)
    ) {
      specs.push(node.moduleSpecifier.text)
    }

    // require('x')
    if (
      ts.isCallExpression(node) &&
      ts.isIdentifier(node.expression) &&
      node.expression.text === "require" &&
      node.arguments.length === 1 &&
      isStringLiteralLike(node.arguments[0])
    ) {
      specs.push(node.arguments[0].text)
    }

    // import('x')
    if (
      ts.isCallExpression(node) &&
      node.expression.kind === ts.SyntaxKind.ImportKeyword &&
      node.arguments.length === 1 &&
      isStringLiteralLike(node.arguments[0])
    ) {
      specs.push(node.arguments[0].text)
    }

    ts.forEachChild(node, visit)
  }

  visit(sf)
  return specs
}

/**
 * Resolve a module specifier to an absolute file path using TypeScript's module resolution.
 */
function resolveModule(
  specifier: string,
  containingFile: string,
  options: ts.CompilerOptions,
  host: ts.ModuleResolutionHost
): string | undefined {
  const resolved = ts.resolveModuleName(specifier, containingFile, options, host)
  const primary = resolved.resolvedModule?.resolvedFileName
  if (!primary) return undefined
  return path.normalize(primary)
}

/**
 * Analyze a given source file for imports that resolve under the given target directory.
 * - Skips .d.ts files.
 * - Honors baseUrl/paths from tsconfig.
 */
export async function analyzeModelImports(
  sourceFile: string,
  targetDir: string,
  tsconfigPath?: string
): Promise<string[]> {
  const { options } = readTsCompilerOptions(tsconfigPath)
  const host = createCompilerHost(options)

  const program = ts.createProgram([sourceFile], options, host)
  const sf = program.getSourceFile(sourceFile)
  if (!sf) {
    return []
  }

  const specs = collectModuleSpecifiers(sf)

  const normalizedTarget = path.normalize(targetDir)
  const hits = new Set<string>()
  for (const spec of specs) {
    const resolved = resolveModule(spec, sourceFile, options, host)
    if (!resolved) continue
    if (resolved.endsWith(".d.ts")) continue // skip .d.ts as requested
    if (resolved.toLowerCase().startsWith(normalizedTarget.toLowerCase() + path.sep)) {
      hits.add(resolved)
    }
  }
  return Array.from(hits)
}

/**
 * Convert absolute file paths to side-effect import statements relative to a package root.
 * - For index.(ts|tsx|js), imports the folder.
 * - For other files, strips the extension.
 */
export function toVirtualEntryImports(resolvedFiles: string[], packageRoot: string): string[] {
  const uniq = new Set<string>()
  for (const abs of resolvedFiles) {
    let rel = path.relative(packageRoot, abs).replace(/\\/g, "/")
    if (/(^|\/)index\.(ts|tsx|js)$/.test(rel)) {
      rel = rel.replace(/\/index\.(ts|tsx|js)$/, "")
    } else {
      rel = rel.replace(/\.(ts|tsx|js)$/, "")
    }
    const spec = `./${rel}`
    uniq.add(`import '${spec}';`)
  }
  return Array.from(uniq)
}
