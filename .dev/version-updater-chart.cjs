const yaml = require("js-yaml")

module.exports = {
  readVersion: (contents) => {
    let chart
    try {
      chart = yaml.load(contents)
    } catch (e) {
      console.error(e)
      throw e
    }
    return chart.version
  },
  writeVersion: (contents, version) => {
    const chart = yaml.load(contents)
    chart.version = version
    chart.appVersion = version
    return yaml.dump(chart, { indent: 2 })
  },
}
