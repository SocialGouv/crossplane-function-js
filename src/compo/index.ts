import path from 'path'
import fs from 'fs-extra'
import YAML from 'yaml'
import { createLogger } from 'skyhook-libs'

// Create a logger for this module
const moduleLogger = createLogger('compo')

// Define interfaces for the manifest structure
interface ManifestMetadata {
  name: string;
}

interface CompositeTypeRef {
  apiVersion: string;
  kind: string;
}

interface FunctionRef {
  name: string;
}

interface SourceSpec {
  inline: string;
  dependencies?: Record<string, string>;
  yarnLock?: string;
}

interface InputSpec {
  spec: {
    source: SourceSpec,
    dependencies?: unknown
  };
}

interface PipelineStep {
  step?: string;
  functionRef?: FunctionRef;
  input?: InputSpec;
}

interface Manifest {
  metadata?: ManifestMetadata;
  spec?: {
    compositeTypeRef?: CompositeTypeRef,
    pipeline?: PipelineStep[]
  };
}

/**
 * Main function for the compo command
 * Processes function directories and generates composition manifests
 * @returns Promise<void>
 */
export default async function(): Promise<void> {
  const cwd = () => `${process.cwd()}`
  const skyhookRootPath =
    path.basename(__dirname) === 'build'
      ? path.join(__dirname, '..')
      : __dirname
  try {
    // Find the functions directory in the current working directory
    const functionsDir = path.join(cwd(), 'functions')

    // Check if the functions directory exists
    if (!fs.existsSync(functionsDir)) {
      moduleLogger.error(`Functions directory not found: ${functionsDir}`)
      process.exit(1)
    }

    // Create the manifests directory if it doesn't exist
    const manifestsDir = path.join(cwd(), 'manifests')
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
      moduleLogger.warn('No function directories found')
      return
    }

    moduleLogger.info(`Found ${functionDirs.length} function directories`)

    // Process each function directory
    for (const functionName of functionDirs) {
      const functionDir = path.join(functionsDir, functionName)

      // Check if composition.yaml exists
      let manifest: Manifest
      const yamlFilePath = path.join(functionDir, 'composition.yaml')

      if (fs.existsSync(yamlFilePath)) {
        // Use the function's own composition.yaml as template
        const yamlContent = fs.readFileSync(yamlFilePath, { encoding: 'utf8' })
        manifest = YAML.parse(yamlContent)
      } else {
        // Use the generic template from src/compo/template.yaml
        const genericTemplatePath = path.join(
          skyhookRootPath,
          'src/compo/composition.default.yaml'
        )

        if (!fs.existsSync(genericTemplatePath)) {
          moduleLogger.error(
            `Generic template not found: ${genericTemplatePath}`
          )
          continue
        }

        const templateContent = fs.readFileSync(genericTemplatePath, {
          encoding: 'utf8'
        })
        manifest = YAML.parse(templateContent)

        // Update the name in the manifest to match the function name
        if (manifest.metadata && manifest.metadata.name) {
          manifest.metadata.name = manifest.metadata.name.replace(
            '__FUNCTION_NAME__',
            functionName
          )
        }

        // Update the kind in the compositeTypeRef
        if (
          manifest.spec &&
          manifest.spec.compositeTypeRef &&
          manifest.spec.compositeTypeRef.kind
        ) {
          manifest.spec.compositeTypeRef.kind = manifest.spec.compositeTypeRef.kind.replace(
            '__FUNCTION_NAME__',
            functionName
          )
        }

        // Update the step name
        if (
          manifest.spec &&
          manifest.spec.pipeline &&
          Array.isArray(manifest.spec.pipeline)
        ) {
          for (let i = 0; i < manifest.spec.pipeline.length; i++) {
            const step = manifest.spec.pipeline[i].step
            if (step) {
              manifest.spec.pipeline[i].step = step.replace(
                '__FUNCTION_NAME__',
                functionName
              )
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
          `Invalid manifest structure for ${
            functionName
          }: missing or empty pipeline`
        )
        continue
      }

      // Find the first step with a functionRef to function-skyhook
      const skyhookStep = manifest.spec.pipeline.find(
        (step: PipelineStep) =>
          step.functionRef && step.functionRef.name === 'function-skyhook'
      )

      if (!skyhookStep) {
        moduleLogger.error(
          `No skyhook function step found in manifest for ${functionName}`
        )
        continue
      }

      // Ensure the input structure exists
      if (
        !skyhookStep.input ||
        !skyhookStep.input.spec ||
        !skyhookStep.input.spec.source
      ) {
        moduleLogger.error(
          `Invalid input structure in skyhook step for ${functionName}`
        )
        continue
      }

      if (skyhookStep.input.spec.source.inline === '__FUNCTION_CODE__') {
        // Check if composition.fn.ts exists
        const fnFilePath = path.join(functionDir, 'composition.fn.ts')
        if (!fs.existsSync(fnFilePath)) {
          moduleLogger.warn(
            `Skipping ${functionName}: composition.fn.ts not found`
          )
          continue
        }
        // Read the function code
        const fnCode = fs.readFileSync(fnFilePath, { encoding: 'utf8' })
        // Set the inline code
        skyhookStep.input.spec.source.inline = fnCode
      }

      if (skyhookStep.input.spec.dependencies === '__DEPENDENCIES__') {
        // Check for package.json in the function directory
        let dependencies: Record<string, string> = {}
        const packageJsonPath = path.join(functionDir, 'package.json')
        const rootPackageJsonPath = path.join(cwd(), 'package.json')

        if (fs.existsSync(packageJsonPath)) {
          // Use package.json from the function directory
          try {
            const packageJson = JSON.parse(
              fs.readFileSync(packageJsonPath, { encoding: 'utf8' })
            )
            if (packageJson.dependencies) {
              dependencies = packageJson.dependencies
              moduleLogger.debug(
                `Using dependencies from function directory: ${functionName}`
              )
            }
          } catch (error) {
            moduleLogger.error(
              `Error parsing package.json in function directory: ${error}`
            )
          }
        } else if (fs.existsSync(rootPackageJsonPath)) {
          try {
            const packageJson = JSON.parse(
              fs.readFileSync(rootPackageJsonPath, { encoding: 'utf8' })
            )
            if (packageJson.dependencies) {
              dependencies = packageJson.dependencies
              moduleLogger.debug(
                `Using dependencies from current working directory`
              )
            }
          } catch (error) {
            moduleLogger.error(
              `Error parsing package.json in current working directory: ${
                error
              }`
            )
          }
        }

        // Add dependencies to the manifest if any were found
        if (Object.keys(dependencies).length > 0) {
          dependencies = Object.keys(dependencies).reduce<Record<string, string>>(
            (obj, key) => {
                if (key === 'crossplane-skyhook') {
                  return obj
                }
                obj[key] = dependencies[key]
                return obj
              },
              {})
          skyhookStep.input.spec.source.dependencies = dependencies
        }
      }

      if (skyhookStep.input.spec.source.yarnLock === '__YARN_LOCK__') {
        // Check for yarn.lock in the function directory
        const functionYarnLockPath = path.join(functionDir, 'yarn.lock')
        const rootYarnLockPath = path.join(cwd(), 'yarn.lock')
        let yarnLock: string | null = null

        if (fs.existsSync(functionYarnLockPath)) {
          try {
            // Use yarn.lock from the function directory
            yarnLock = fs.readFileSync(functionYarnLockPath, {
              encoding: 'utf8'
            })
            moduleLogger.debug(
              `Using yarn.lock from function directory: ${functionName}`
            )
          } catch (error) {
            moduleLogger.error(
              `Error reading yarn.lock in function directory: ${error}`
            )
          }
        } else if (fs.existsSync(rootYarnLockPath)) {
          try {
            // Use yarn.lock from the current working directory
            yarnLock = fs.readFileSync(rootYarnLockPath, { encoding: 'utf8' })
            moduleLogger.debug(`Using yarn.lock from current working directory`)
          } catch (error) {
            moduleLogger.error(
              `Error reading yarn.lock in current working directory: ${error}`
            )
          }
        }

        // Add yarn.lock to the manifest if found
        if (yarnLock) {
          skyhookStep.input.spec.source.yarnLock = yarnLock
        }
      }

      // Generate the output file
      const outputPath = path.join(manifestsDir, `${functionName}.compo.yaml`)
      const yamlOutput = YAML.stringify(manifest)

      // Write the output file
      fs.writeFileSync(outputPath, yamlOutput)

      moduleLogger.info(`Generated manifest for ${functionName}: ${outputPath}`)
    }

    moduleLogger.info('Composition manifest generation completed')
  } catch (error) {
    moduleLogger.error(`Error generating composition manifests: ${error}`)
    process.exit(1)
  }
}
