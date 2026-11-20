const pulumi = require("@pulumi/pulumi");

const sameInputs = async (_id, olds, news) => ({
  changes: JSON.stringify(olds) !== JSON.stringify(news),
});

class Box extends pulumi.dynamic.Resource {
  constructor(name, args, opts) {
    const provider = {
      create: async () => ({ id: name, outs: {} }),
      diff: sameInputs,
      update: async () => ({ outs: {} }),
    };
    super(provider, name, args, opts);
  }
}

module.exports = { Box };
