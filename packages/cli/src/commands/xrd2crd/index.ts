import { createLogger } from "@crossplane-js/libs"
import { Command } from "commander"
import fs from "fs-extra"
import YAML from "yaml"

import { convertXRDtoCRD, parseAndValidateXRD } from "./converter.js"

// Create a logger for this module
const moduleLogger = createLogger("xrd2crd")

/**
 * Main function for the xrd2crd command
 * Converts an XRD file to CRD format and outputs to stdout
 * @param xrdPath Path to the XRD file
 * @returns Promise<void>
 */
async function xrd2crdAction(xrdPath: string): Promise<void> {
  try {
    // Read and parse the XRD file
    const xrdContent = await fs.readFile(xrdPath, { encoding: "utf8" })
    const xrd = parseAndValidateXRD(xrdContent)

    // Convert XRD to CRD
    const crd = convertXRDtoCRD(xrd)

    // Output the CRD to stdout
    process.stdout.write(YAML.stringify(crd))
  } catch (error) {
    moduleLogger.error(`Error converting XRD to CRD: ${error}`)
    process.exit(1)
  }
}

/**
 * Register the xrd2crd command with the CLI
 * @param program The Commander program instance
 */
export default function (program: Command): void {
  program
    .command("xrd2crd")
    .description("Convert a Crossplane XRD to a Kubernetes CRD")
    .argument("<xrdPath>", "Path to the XRD file")
    .action(async xrdPath => {
      try {
        await xrd2crdAction(xrdPath)
      } catch (err) {
        moduleLogger.error(`Error running xrd2crd command: ${err}`)
        process.exit(1)
      }
    })
}
