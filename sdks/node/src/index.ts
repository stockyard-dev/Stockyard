import OpenAI from "openai";

const DEFAULT_BASE = process.env.STOCKYARD_URL
  ? `${process.env.STOCKYARD_URL}/v1`
  : "http://localhost:8080/v1";

export class StockyardOpenAI extends OpenAI {
  constructor(opts: ConstructorParameters<typeof OpenAI>[0] = {} as any) {
    super({
      ...opts,
      baseURL: opts.baseURL || DEFAULT_BASE,
      defaultHeaders: {
        ...opts.defaultHeaders,
        "X-Stockyard-Source": "node-sdk",
      },
    });
  }
}

export default StockyardOpenAI;
