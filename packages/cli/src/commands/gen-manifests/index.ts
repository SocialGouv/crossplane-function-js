import { tmpdir } from "os"
import path from "path"
import { fileURLToPath } from "url"

import { createLogger, getLatestKubernetesVersion } from "@crossplane-js/libs"
import { Command } from "commander"
import { build } from "esbuild"
import type { BuildOptions, Plugin } from "esbuild"
import fs from "fs-extra"
import { v4 as uuidv4 } from "uuid"
import YAML from "yaml"

// Create a logger for this module
const moduleLogger = createLogger("gen-manifests")

// Define interfaces for the manifest structure
interface ManifestMetadata {
  name: string
}

interface CompositeTypeRef {
  apiVersion: string
  kind: string
}

interface FunctionRef {
  name: string
}

interface SourceSpec {
  inline: string
  dependencies?: Record<string, string>
  yarnLock?: string
  tsConfig?: string
}

interface InputSpec {
  spec: {
    source: SourceSpec
    dependencies?: unknown
  }
}

interface PipelineStep {
  step?: string
  functionRef?: FunctionRef
  input?: InputSpec
}

interface Manifest {
  metadata?: ManifestMetadata
  spec?: {
    compositeTypeRef?: CompositeTypeRef
    pipeline?: PipelineStep[]
  }
}

interface TypeScriptConfig {
  extends?: string
  compilerOptions?: {
    baseUrl?: string
    paths?: Record<string, string[]>
    [key: string]: any
  }
  [key: string]: any
}

/**
 * Loads and resolves a TypeScript configuration file, handling extends
 * @param configPath Path to the tsconfig.json file
 * @param baseDir Base directory for resolving relative paths
 * @returns Resolved TypeScript configuration
 */
function loadTsConfig(configPath: string, _baseDir?: string): TypeScriptConfig | null {
  try {
    if (!fs.existsSync(configPath)) {
      return null
    }

    const configContent = fs.readFileSync(configPath, { encoding: "utf8" })
    const config: TypeScriptConfig = JSON.parse(configContent)
    const configDir = path.dirname(configPath)

    // If the config extends another config, load and merge it
    if (config.extends) {
      const extendsPath = path.resolve(configDir, config.extends)
      const baseConfig = loadTsConfig(extendsPath, configDir)

      if (baseConfig) {
        // Merge configurations, with current config taking precedence
        const mergedConfig: TypeScriptConfig = {
          ...baseConfig,
          ...config,
          compilerOptions: {
            ...baseConfig.compilerOptions,
            ...config.compilerOptions,
          },
        }
        return mergedConfig
      }
    }

    return config
  } catch (error) {
    moduleLogger.debug(`Failed to load TypeScript config from ${configPath}: ${error}`)
    return null
  }
}

/**
 * Converts TypeScript path mappings to esbuild alias format
 * @param tsConfig TypeScript configuration
 * @param baseDir Base directory for resolving relative paths
 * @returns esbuild alias configuration
 */
function convertTsPathsToEsbuildAlias(
  tsConfig: TypeScriptConfig,
  baseDir: string
): Record<string, string> | undefined {
  const { compilerOptions } = tsConfig
  if (!compilerOptions?.paths) {
    return undefined
  }

  const { paths } = compilerOptions
  const alias: Record<string, string> = {}

  for (const [pattern, mappings] of Object.entries(paths)) {
    if (mappings.length === 0) continue

    // Take the first mapping (most common case)
    const mapping = mappings[0]

    // Remove the /* suffix from pattern and mapping if present
    const cleanPattern = pattern.replace(/\/\*$/, "")
    const cleanMapping = mapping.replace(/\/\*$/, "")

    // Resolve the mapping path relative to the base directory
    const resolvedMapping = path.resolve(baseDir, cleanMapping)

    alias[cleanPattern] = resolvedMapping
  }

  return Object.keys(alias).length > 0 ? alias : undefined
}

/**
 * Bundles a TypeScript file using esbuild
 * @param filePath Path to the TypeScript file
 * @param embedDeps Whether to embed dependencies or keep them external
 * @param customConfig Custom esbuild configuration
 * @returns Promise<string> The bundled code
 */
async function bundleTypeScript(
  filePath: string,
  embedDeps: boolean = false,
  customConfig: Partial<BuildOptions> = {}
): Promise<string> {
  const tempDir = path.join(tmpdir(), `xfuncjs-bundle-${uuidv4()}`)
  const outputFile = path.join(tempDir, "bundle.js")

  moduleLogger.debug(`Bundling TypeScript file: ${filePath}`)
  moduleLogger.debug(`Using temporary directory: ${tempDir}`)

  try {
    // Create temp directory for output only
    fs.mkdirSync(tempDir, { recursive: true })

    // Get original file size for logging
    const originalSize = fs.statSync(filePath).size

    // Check if models/index.ts exists and prepare to auto-import it
    const packageRoot = process.cwd()
    const modelsIndexPath = path.join(packageRoot, "models", "index.ts")
    const shouldAutoImportModels = fs.existsSync(modelsIndexPath)

    // Create virtual entry content that imports models and re-exports the function
    const relativeFunctionPath = path.relative(packageRoot, filePath).replace(/\\/g, "/")
    let virtualEntryContent = ""

    if (shouldAutoImportModels) {
      virtualEntryContent += `import './models';\n`
      moduleLogger.debug(`Auto-importing models from: ${modelsIndexPath}`)
    }

    // Import and re-export the function as default
    virtualEntryContent += `export { default } from './${relativeFunctionPath}';\n`

    moduleLogger.debug(`Virtual entry content:\n${virtualEntryContent}`)

    // Load TypeScript configuration and extract aliases
    const functionDir = path.dirname(filePath)
    let tsConfig: TypeScriptConfig | null = null
    let esbuildAlias: Record<string, string> | undefined = undefined

    // Try to load tsconfig.json from function directory first, then from current working directory
    const functionTsConfigPath = path.join(functionDir, "tsconfig.json")
    const cwdTsConfigPath = path.join(process.cwd(), "tsconfig.json")

    let configPath: string | null = null
    let configBaseDir: string | null = null

    if (fs.existsSync(functionTsConfigPath)) {
      tsConfig = loadTsConfig(functionTsConfigPath, functionDir)
      configPath = functionTsConfigPath
      configBaseDir = functionDir
      moduleLogger.debug(
        `Loaded TypeScript config from function directory: ${functionTsConfigPath}`
      )
    } else if (fs.existsSync(cwdTsConfigPath)) {
      tsConfig = loadTsConfig(cwdTsConfigPath, process.cwd())
      configPath = cwdTsConfigPath
      configBaseDir = process.cwd()
      moduleLogger.debug(
        `Loaded TypeScript config from current working directory: ${cwdTsConfigPath}`
      )
    }

    // Convert TypeScript path aliases to esbuild aliases
    if (tsConfig && configPath && configBaseDir) {
      const configDir = path.dirname(configPath)
      const baseUrl = tsConfig.compilerOptions?.baseUrl || "."
      const resolvedBaseDir = path.resolve(configDir, baseUrl)

      moduleLogger.debug(`TypeScript config found at: ${configPath}`)
      moduleLogger.debug(`Config directory: ${configDir}`)
      moduleLogger.debug(`Base URL: ${baseUrl}`)
      moduleLogger.debug(`Resolved base directory: ${resolvedBaseDir}`)
      moduleLogger.debug(`TypeScript paths: ${JSON.stringify(tsConfig.compilerOptions?.paths)}`)

      esbuildAlias = convertTsPathsToEsbuildAlias(tsConfig, resolvedBaseDir)

      if (esbuildAlias && Object.keys(esbuildAlias).length > 0) {
        moduleLogger.info(
          `Applied TypeScript path aliases to esbuild: ${JSON.stringify(esbuildAlias)}`
        )
        moduleLogger.info(`Base directory for aliases: ${resolvedBaseDir}`)
      } else {
        moduleLogger.debug(`No TypeScript path aliases found or converted`)
      }
    }

    // Default esbuild options optimized for readability using stdin
    const defaultOptions: BuildOptions = {
      stdin: {
        contents: virtualEntryContent,
        resolveDir: packageRoot,
        sourcefile: "virtual-entry.ts",
      },
      bundle: true,
      format: "esm",
      sourcemap: true,
      target: "esnext",
      outfile: outputFile,
      minify: false,
      keepNames: true,
      legalComments: "inline",
      // Enable TypeScript decorators
      tsconfigRaw: {
        compilerOptions: {
          experimentalDecorators: true,
        },
      },
      // Add TypeScript path aliases if available
      ...(esbuildAlias && { alias: esbuildAlias }),
    }

    // If embedDeps is false, add plugin to keep dependencies external
    if (!embedDeps) {
      // Create a plugin to mark all non-relative imports as external, except for aliases
      const externalizeNpmDepsPlugin: Plugin = {
        name: "externalize-npm-deps",
        setup(build) {
          // Filter for imports that don't start with ./ or ../ (non-relative imports)
          build.onResolve({ filter: /^[^./]/ }, args => {
            moduleLogger.debug(`Processing import: ${args.path} from ${args.importer || "entry"}`)

            // Skip if this is an entry point (no importer means it's an entry point)
            if (!args.importer) {
              moduleLogger.debug(`Skipping entry point: ${args.path}`)
              return undefined
            }

            // Skip built-in Node.js modules
            if (args.path.startsWith("node:")) {
              moduleLogger.debug(`Skipping Node.js built-in: ${args.path}`)
              return undefined
            }

            // Check if this matches any of our TypeScript path aliases
            // Be more specific about alias matching to avoid false positives with scoped packages
            if (esbuildAlias) {
              for (const alias of Object.keys(esbuildAlias)) {
                // For aliases like "@", only match if it's followed by a slash (e.g., "@/models")
                // This prevents matching scoped packages like "@crossplane-js/sdk"
                if (alias === "@" && args.path.startsWith("@/")) {
                  moduleLogger.debug(`Allowing alias resolution: ${args.path}`)
                  return undefined // Let esbuild handle the alias resolution
                } else if (alias !== "@" && args.path.startsWith(alias)) {
                  moduleLogger.debug(`Allowing alias resolution: ${args.path}`)
                  return undefined // Let esbuild handle the alias resolution
                }
              }
            }

            // For all other imports (npm packages, scoped packages, etc.), mark as external
            moduleLogger.debug(`Marking as external: ${args.path}`)
            return { path: args.path, external: true }
          })
        },
      }

      defaultOptions.plugins = [externalizeNpmDepsPlugin]
      moduleLogger.debug(`Keeping all node_modules packages as external dependencies`)
    } else {
      moduleLogger.debug(`Embedding all dependencies in the bundle`)
    }

    // Merge with custom config
    const buildOptions: BuildOptions = { ...defaultOptions, ...customConfig }
    moduleLogger.debug(`esbuild options: ${JSON.stringify(buildOptions)}`)

    // Bundle with esbuild
    await build(buildOptions)

    // Read the bundled code (always single entry point now)
    const bundledCode = fs.readFileSync(outputFile, { encoding: "utf8" })
    const bundledSize = fs.statSync(outputFile).size

    // Log bundle size information
    moduleLogger.debug(`Bundling complete: ${originalSize} bytes → ${bundledSize} bytes`)

    return bundledCode
  } catch (error) {
    moduleLogger.error(`Bundling failed: ${error}`)
    throw new Error(`Failed to bundle TypeScript file ${filePath}: ${error}`)
  } finally {
    // Clean up temp directory
    if (fs.existsSync(tempDir)) {
      fs.rmSync(tempDir, { recursive: true, force: true })
      moduleLogger.debug(`Cleaned up temporary directory: ${tempDir}`)
    }
  }
}

/**
 * Main function for the gen-manifests command
 * Processes function directories and generates composition manifests
 * @param options Command options
 * @returns Promise<void>
 */
async function genManifestsAction(
  options: { bundle?: boolean; bundleConfig?: string; embedDeps?: boolean } = {}
): Promise<void> {
  // Default to bundling enabled
  const shouldBundle = options.bundle !== false
  // Default to external dependencies (not embedded)
  const shouldEmbedDeps = options.embedDeps === true

  moduleLogger.debug(`Bundle: ${shouldBundle}, Embed dependencies: ${shouldEmbedDeps}`)

  // Parse custom bundle config if provided
  let bundleConfig: Partial<BuildOptions> = {}
  if (options.bundleConfig) {
    try {
      bundleConfig = JSON.parse(options.bundleConfig) as Partial<BuildOptions>
      moduleLogger.debug(`Using custom bundle configuration: ${JSON.stringify(bundleConfig)}`)
    } catch (error) {
      moduleLogger.error(`Invalid bundle configuration JSON: ${error}`)
      process.exit(1)
    }
  }
  const cwd = () => `${process.cwd()}`
  try {
    // Find the functions directory in the current working directory
    const functionsDir = path.join(cwd(), "functions")

    // Check if the functions directory exists
    if (!fs.existsSync(functionsDir)) {
      moduleLogger.error(`Functions directory not found: ${functionsDir}`)
      process.exit(1)
    }

    // Create the manifests directory if it doesn't exist
    const manifestsDir = path.join(cwd(), "manifests")
    if (!fs.existsSync(manifestsDir)) {
      moduleLogger.info(`Creating manifests directory: ${manifestsDir}`)
      fs.mkdirSync(manifestsDir)
    }

    // Get all direct subdirectories of the functions directory
    const functionDirs = fs
      .readdirSync(functionsDir, { withFileTypes: true })
      .filter(dirent => dirent.isDirectory())
      .map(dirent => dirent.name)

    if (functionDirs.length === 0) {
      moduleLogger.warn("No function directories found")
      return
    }

    moduleLogger.info(`Found ${functionDirs.length} function directories`)

    // Process each function directory
    for (const functionName of functionDirs) {
      const functionDir = path.join(functionsDir, functionName)

      // Check for XRD file early and parse it once
      const xrdFilePath = path.join(functionDir, "xrd.yaml")

      const xrdContent: string = fs.readFileSync(xrdFilePath, { encoding: "utf8" })
      const xrdManifest = YAML.parse(xrdContent)
      moduleLogger.debug(`Loaded XRD file for ${functionName}`)

      // Check if composition.yaml exists
      let manifest: Manifest
      const yamlFilePath = path.join(functionDir, "composition.yaml")

      if (fs.existsSync(yamlFilePath)) {
        // Use the function's own composition.yaml as template
        const yamlContent = fs.readFileSync(yamlFilePath, { encoding: "utf8" })
        manifest = YAML.parse(yamlContent)
      } else {
        // Use the generic template from templates/composition.default.yaml
        // Get the directory name from the import.meta.url
        const __filename = fileURLToPath(import.meta.url)
        const __dirname = path.dirname(__filename)
        const genericTemplatePath = path.join(__dirname, "templates/composition.default.yaml")

        if (!fs.existsSync(genericTemplatePath)) {
          moduleLogger.error(`Generic template not found: ${genericTemplatePath}`)
          continue
        }

        const templateContent = fs.readFileSync(genericTemplatePath, {
          encoding: "utf8",
        })
        manifest = YAML.parse(templateContent)

        // Update the name in the manifest to match the function name
        if (manifest.metadata && manifest.metadata.name) {
          manifest.metadata.name = manifest.metadata.name.replace("__FUNCTION_NAME__", functionName)
        }

        // Update the apiVersion in the compositeTypeRef using the already loaded XRD
        if (manifest.spec && manifest.spec.compositeTypeRef) {
          const group = xrdManifest.spec.group
          const version = getLatestKubernetesVersion(xrdManifest.spec.versions)
          const apiVersion = `${group}/${version}`

          manifest.spec.compositeTypeRef.apiVersion = apiVersion
          manifest.spec.compositeTypeRef.kind = xrdManifest.spec.names.kind
          moduleLogger.debug(
            `Set compositeTypeRef.apiVersion to ${apiVersion} (latest version) from XRD for ${functionName}`
          )
        }

        // Update the step name
        if (manifest.spec && manifest.spec.pipeline && Array.isArray(manifest.spec.pipeline)) {
          for (let i = 0; i < manifest.spec.pipeline.length; i++) {
            const step = manifest.spec.pipeline[i].step
            if (step) {
              manifest.spec.pipeline[i].step = step.replace("__FUNCTION_NAME__", functionName)
            }
          }
        }
      }

      // Ensure the pipeline exists
      if (
        !manifest.spec ||
        !manifest.spec.pipeline ||
        !Array.isArray(manifest.spec.pipeline) ||
        manifest.spec.pipeline.length === 0
      ) {
        moduleLogger.error(
          `Invalid manifest structure for ${functionName}: missing or empty pipeline`
        )
        continue
      }

      // Find the first step with a functionRef to function-xfuncjs
      const xfuncjsStep = manifest.spec.pipeline.find(
        (step: PipelineStep) => step.functionRef && step.functionRef.name === "function-xfuncjs"
      )

      if (!xfuncjsStep) {
        moduleLogger.error(`No xfuncjs function step found in manifest for ${functionName}`)
        continue
      }

      // Ensure the input structure exists
      if (!xfuncjsStep.input || !xfuncjsStep.input.spec || !xfuncjsStep.input.spec.source) {
        moduleLogger.error(`Invalid input structure in xfuncjs step for ${functionName}`)
        continue
      }

      if (xfuncjsStep.input.spec.source.inline === "__FUNCTION_CODE__") {
        // Check if composition.fn.ts exists
        const fnFilePath = path.join(functionDir, "composition.fn.ts")
        if (!fs.existsSync(fnFilePath)) {
          moduleLogger.warn(`Skipping ${functionName}: composition.fn.ts not found`)
          continue
        }

        if (shouldBundle) {
          moduleLogger.info(`Bundling TypeScript for ${functionName}`)
          try {
            const bundledCode = await bundleTypeScript(fnFilePath, shouldEmbedDeps, bundleConfig)
            xfuncjsStep.input.spec.source.inline = bundledCode
            moduleLogger.info(`Successfully bundled TypeScript for ${functionName}`)
          } catch (error) {
            moduleLogger.error(`Error bundling TypeScript for ${functionName}`)
            throw error // Propagate the error up
          }
        } else {
          // Original behavior when bundling is disabled
          moduleLogger.info(`Bundling disabled, using raw TypeScript for ${functionName}`)
          const fnCode = fs.readFileSync(fnFilePath, { encoding: "utf8" })
          xfuncjsStep.input.spec.source.inline = fnCode
        }
      }

      if (xfuncjsStep.input.spec.source.dependencies === ("__DEPENDENCIES__" as any)) {
        // Check for package.json in the function directory
        let dependencies: Record<string, string> = {}
        const packageJsonPath = path.join(functionDir, "package.json")
        const rootPackageJsonPath = path.join(cwd(), "package.json")

        if (fs.existsSync(packageJsonPath)) {
          // Use package.json from the function directory
          try {
            const packageJson = JSON.parse(fs.readFileSync(packageJsonPath, { encoding: "utf8" }))
            if (packageJson.dependencies) {
              dependencies = packageJson.dependencies
              moduleLogger.debug(`Using dependencies from function directory: ${functionName}`)
            }
          } catch (error) {
            moduleLogger.error(`Error parsing package.json in function directory: ${error}`)
          }
        } else if (fs.existsSync(rootPackageJsonPath)) {
          try {
            const packageJson = JSON.parse(
              fs.readFileSync(rootPackageJsonPath, { encoding: "utf8" })
            )
            if (packageJson.dependencies) {
              dependencies = packageJson.dependencies
              moduleLogger.debug(`Using dependencies from current working directory`)
            }
          } catch (error) {
            moduleLogger.error(`Error parsing package.json in current working directory: ${error}`)
          }
        }

        // Add dependencies to the manifest if any were found
        if (Object.keys(dependencies).length > 0) {
          dependencies = Object.keys(dependencies).reduce<Record<string, string>>((obj, key) => {
            if (key === "xfuncjs-server") {
              return obj
            }
            obj[key] = dependencies[key]
            return obj
          }, {})
          xfuncjsStep.input.spec.source.dependencies = dependencies
        }
      }

      if (xfuncjsStep.input.spec.source.yarnLock === "__YARN_LOCK__") {
        // Check for yarn.lock in the function directory
        const functionYarnLockPath = path.join(functionDir, "yarn.lock")
        const rootYarnLockPath = path.join(cwd(), "yarn.lock")
        let yarnLock: string | null = null

        if (fs.existsSync(functionYarnLockPath)) {
          try {
            // Use yarn.lock from the function directory
            yarnLock = fs.readFileSync(functionYarnLockPath, {
              encoding: "utf8",
            })
            moduleLogger.debug(`Using yarn.lock from function directory: ${functionName}`)
          } catch (error) {
            moduleLogger.error(`Error reading yarn.lock in function directory: ${error}`)
          }
        } else if (fs.existsSync(rootYarnLockPath)) {
          try {
            // Use yarn.lock from the current working directory
            yarnLock = fs.readFileSync(rootYarnLockPath, { encoding: "utf8" })
            moduleLogger.debug(`Using yarn.lock from current working directory`)
          } catch (error) {
            moduleLogger.error(`Error reading yarn.lock in current working directory: ${error}`)
          }
        }

        // Add yarn.lock to the manifest if found
        if (yarnLock) {
          xfuncjsStep.input.spec.source.yarnLock = yarnLock
        }
      }

      if (xfuncjsStep.input.spec.source.tsConfig === "__TSCONFIG__") {
        // Check for tsconfig.json in the function directory
        const functionTsConfigPath = path.join(functionDir, "tsconfig.json")
        const rootTsConfigPath = path.join(cwd(), "tsconfig.json")
        let tsConfig: string | null = null

        if (fs.existsSync(functionTsConfigPath)) {
          try {
            // Use tsconfig.json from the function directory
            tsConfig = fs.readFileSync(functionTsConfigPath, {
              encoding: "utf8",
            })
            moduleLogger.debug(`Using tsconfig.json from function directory: ${functionName}`)
          } catch (error) {
            moduleLogger.error(`Error reading tsconfig.json in function directory: ${error}`)
          }
        } else if (fs.existsSync(rootTsConfigPath)) {
          try {
            // Use tsconfig.json from the current working directory
            tsConfig = fs.readFileSync(rootTsConfigPath, { encoding: "utf8" })
            moduleLogger.debug(`Using tsconfig.json from current working directory`)
          } catch (error) {
            moduleLogger.error(`Error reading tsconfig.json in current working directory: ${error}`)
          }
        }

        // Add tsconfig.json to the manifest if found
        if (tsConfig) {
          xfuncjsStep.input.spec.source.tsConfig = tsConfig
        }
      }

      // Generate final output using the already loaded XRD data
      let finalOutput: string

      if (xrdManifest && xrdContent) {
        // Generate multi-document YAML with XRD first, then composition
        const xrdYaml = YAML.stringify(xrdManifest)
        const compositionYaml = YAML.stringify(manifest)

        // Combine with document separator
        finalOutput = `${xrdYaml}---\n${compositionYaml}`

        moduleLogger.info(`Including XRD from ${xrdFilePath} for ${functionName}`)
      } else {
        // No XRD file, use composition only (existing behavior)
        finalOutput = YAML.stringify(manifest)
      }

      // Generate the output file
      const outputPath = path.join(manifestsDir, `${functionName}.compo.yaml`)

      // Write the output file
      fs.writeFileSync(outputPath, finalOutput)

      moduleLogger.info(`Generated manifest for ${functionName}: ${outputPath}`)
    }

    moduleLogger.info("Composition manifest generation completed")
  } catch (error) {
    moduleLogger.error(`Error generating composition manifests: ${error}`)
    process.exit(1)
  }
}

/**
 * Register the gen-manifests command with the CLI
 * @param program The Commander program instance
 */
export default function (program: Command): void {
  program
    .command("gen-manifests")
    .description("Generate composition manifests from function directories")
    .option("--no-bundle", "Disable TypeScript bundling")
    .option("--bundle-config <json>", "Custom esbuild configuration (JSON string)")
    .option("--embed-deps", "Embed dependencies in the bundle (default: false)")
    .action(async options => {
      try {
        await genManifestsAction(options)
      } catch (err) {
        moduleLogger.error(`Error running gen-manifests command: ${err}`)
        process.exit(1)
      }
    })
}
