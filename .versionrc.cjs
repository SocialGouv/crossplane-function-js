const fs = require("fs")

const getTreeDirSync = (source) =>
  fs
    .readdirSync(source, { withFileTypes: true })
    .filter((dirent) => dirent.isDirectory() || dirent.isSymbolicLink())
    .map((dirent) => dirent.name)

const workspaces = ["packages"]

const bumpFiles = [
  { filename: "package.json", type: "json" },
  ...workspaces.reduce((acc, dir) => {
    acc.push(
      ...getTreeDirSync(dir).map((subdir) => ({
        filename: `${dir}/${subdir}/package.json`,
        type: "json",
      }))
    )
    return acc
  }, []),
]

const chartsUpdater = "./.dev/version-updater-chart.cjs"
bumpFiles.push({
  filename: `charts/crossplane-function-js/Chart.yaml`,
  updater: chartsUpdater,
})


module.exports = {
  bumpFiles,
}
