import * as docker from "@pulumi/docker";

const stockyard = new docker.Container("stockyard", {
  image: "stockyard/stockyard:latest",
  ports: [{ internal: 4000, external: 4000 }],
  envs: [`OPENAI_API_KEY=${process.env.OPENAI_API_KEY}`],
});

export const url = stockyard.ports.apply(p => `http://localhost:${p![0].external}`);
