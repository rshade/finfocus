const pulumi = require("@pulumi/pulumi");
const { Box } = require("./box");

class Pair extends pulumi.ComponentResource {
  constructor(name, opts) {
    super("shop:index:Pair", name, {}, opts);
    this.left = new Box(`${name}-left`, { size: "small" }, { parent: this });
    this.right = new Box(`${name}-right`, { size: "small" }, { parent: this });
  }
}

module.exports = { Pair };
