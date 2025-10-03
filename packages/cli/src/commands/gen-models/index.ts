import { spawn } from "child_process"
import os from "os"
import path from "path"
import { fileURLToPath } from "url"

import { createLogger } from "@crossplane-js/libs"
import { readInput } from "@kubernetes-models/read-input"
import { type Command } from "commander"
import fs from "fs-extra"
import YAML from "yaml"

import { convertXRDtoCRD, parseAndValidateXRD } from "../xrd2crd/converter.ts"

// Create a logger for this module
const moduleLogger = createLogger("gen-models")

// Resolve CLI package root to run yarn in the right workspace
const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)
// packages/cli/src/commands/gen-models -> packages/cli
const cliRoot = path.resolve(__dirname, "../../..")

/**
 * Fetch extra CRDs
 * @param urls Array of CRD urls
 * @returns Promise<string[]> Array of CRD paths
 */
async function fetchExtraCRDs(paths: string[]): Promise<string[]> {
  const extraDocuments: string[] = []
  for (const path of paths) {
    moduleLogger.info("Reading extra CRD: %s", path)
    extraDocuments.push(await readInput(path))
  }
  return extraDocuments
}

/**
 * Find all XRD files in the functions directory
 * @returns Promise<string[]> Array of XRD file paths
 */
async function findXRDFiles(): Promise<string[]> {
  const functionsDir = "functions"

  // Check if functions directory exists
  if (!(await fs.pathExists(functionsDir))) {
    throw new Error(`Functions directory '${functionsDir}' does not exist`)
  }

  const dirs = await fs.readdir(functionsDir, { withFileTypes: true })
  const xrdFiles: string[] = []

  for (const dir of dirs) {
    if (dir.isDirectory()) {
      const xrdPath = path.join(functionsDir, dir.name, "xrd.yaml")
      if (await fs.pathExists(xrdPath)) {
        xrdFiles.push(xrdPath)
      }
    }
  }

  if (xrdFiles.length === 0) {
    throw new Error("No XRD files found in functions/*/xrd.yaml")
  }

  return xrdFiles
}

async function runCrdGenerate(crdYaml: string, outputPath: string): Promise<void> {
  const tmpBase = path.join(os.tmpdir(), "xfuncjs-")
  const tmpDir = await fs.mkdtemp(tmpBase)
  const inputFile = path.join(tmpDir, "crd.yaml")
  try {
    await fs.writeFile(inputFile, crdYaml, { encoding: "utf8" })
    const outputAbs = path.resolve(process.cwd(), outputPath)
    await new Promise<void>((resolve, reject) => {
      // we use spawning instead of importing lib, because of this https://github.com/tommy351/kubernetes-models-ts/issues/241
      const args = ["crd-generate", "--input", inputFile, "--output", outputAbs]
      const child = spawn("yarn", args, {
        cwd: cliRoot,
        stdio: ["ignore", "inherit", "inherit"],
      })
      child.on("error", err => reject(err))
      child.on("exit", code => {
        if (code === 0) resolve()
        else reject(new Error(`crd-generate exited with code ${code}`))
      })
    })
  } finally {
    // Cleanup temp files
    await fs.remove(tmpDir)
  }
}

/**
 * Main function for the gen-models command
 * Generates TypeScript models from all XRD files in functions/
 * @returns Promise<void>
 */
async function genModelsAction(): Promise<void> {
  try {
    moduleLogger.info("Starting model generation...")

    const documents: string[] = []

    // Find all XRD files
    const xrdFiles = await findXRDFiles()
    moduleLogger.info(`Found ${xrdFiles.length} XRD file(s): ${xrdFiles.join(", ")}`)

    // Clean up and recreate the models directory
    const modelsDir = "models"
    if (await fs.pathExists(modelsDir)) {
      moduleLogger.info(`Removing existing models directory: ${modelsDir}`)
      await fs.remove(modelsDir)
    }
    moduleLogger.info(`Creating fresh models directory: ${modelsDir}`)
    await fs.ensureDir(modelsDir)

    // Process each XRD file
    for (const xrdPath of xrdFiles) {
      moduleLogger.info(`Processing ${xrdPath}...`)

      try {
        // Read and parse the XRD file
        const xrdContent = await fs.readFile(xrdPath, { encoding: "utf8" })
        const xrd = parseAndValidateXRD(xrdContent)

        // Convert XRD to CRD
        const crd = convertXRDtoCRD(xrd)
        const crdYaml = YAML.stringify(crd)
        documents.push(crdYaml)

        moduleLogger.info(`✓ Generated models for ${xrdPath}`)
      } catch (error) {
        moduleLogger.error(`✗ Failed to process ${xrdPath}: ${error}`)
        throw error
      }
    }

    // Get extra CRD urls from config file
    if (await fs.exists("config.yaml")) {
      moduleLogger.info("Config file exists, searching extra CRDs...")
      const configFile = await fs.readFile("config.yaml", "utf8")
      const config = YAML.parse(configFile)
      // FIXME handle config.extraCrds not existing
      const crds = await fetchExtraCRDs(config.extraCrds)
      for (const crd of crds) {
        documents.push(crd)
      }
    }

    // Generate models using crd-generate (via child process)
    const allCrds = documents.join("\n---\n")
    await runCrdGenerate(allCrds, modelsDir)

    moduleLogger.info(`✓ Model generation completed. Models saved to '${modelsDir}/' directory.`)
  } catch (error) {
    moduleLogger.error(`Error generating models: ${error}`)
    process.exit(1)
  }
}

/**
 * Register the gen-models command with the CLI
 * @param program The Commander program instance
 */
export default function (program: Command): void {
  program
    .command("gen-models")
    .description("Generate TypeScript models from all XRD files in functions/")
    .action(async () => {
      try {
        await genModelsAction()
      } catch (err) {
        moduleLogger.error(`Error running gen-models command: ${err}`)
        process.exit(1)
      }
    })
}
