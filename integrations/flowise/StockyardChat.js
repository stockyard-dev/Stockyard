/**
 * Stockyard Chat Node for Flowise
 * 
 * Drop-in replacement for ChatOpenAI that routes through Stockyard proxy.
 * Copy to: Flowise/packages/components/nodes/chatmodels/
 */

const { ChatOpenAI } = require("@langchain/openai");

class StockyardChat {
    constructor() {
        this.label = "Stockyard Chat";
        this.name = "stockyardChat";
        this.version = 1.0;
        this.type = "StockyardChat";
        this.icon = "stockyard.svg";
        this.category = "Chat Models";
        this.description = "Chat model via Stockyard proxy — cost caps, caching, rate limiting, and analytics";
        this.baseClasses = ["BaseChatModel", "BaseLanguageModel"];
        this.inputs = [
            {
                label: "Stockyard Proxy URL",
                name: "proxyUrl",
                type: "string",
                default: "http://localhost:4000/v1",
                description: "Base URL of your Stockyard proxy",
            },
            {
                label: "Model Name",
                name: "modelName",
                type: "string",
                default: "gpt-4o-mini",
                description: "Model name (Stockyard ModelSwitch may override)",
            },
            {
                label: "Temperature",
                name: "temperature",
                type: "number",
                default: 0.7,
                optional: true,
            },
            {
                label: "Max Tokens",
                name: "maxTokens",
                type: "number",
                default: 1000,
                optional: true,
            },
            {
                label: "User ID",
                name: "userId",
                type: "string",
                default: "flowise",
                optional: true,
                description: "User ID for UsagePulse metering",
            },
        ];
    }

    async init(nodeData) {
        const proxyUrl = nodeData.inputs?.proxyUrl || "http://localhost:4000/v1";
        const modelName = nodeData.inputs?.modelName || "gpt-4o-mini";
        const temperature = parseFloat(nodeData.inputs?.temperature) || 0.7;
        const maxTokens = parseInt(nodeData.inputs?.maxTokens) || 1000;

        const model = new ChatOpenAI({
            openAIApiKey: "stockyard",
            configuration: {
                baseURL: proxyUrl,
                defaultHeaders: {
                    "X-Stockyard-User": nodeData.inputs?.userId || "flowise",
                    "X-Stockyard-Feature": "flowise-flow",
                },
            },
            modelName,
            temperature,
            maxTokens,
            streaming: true,
        });

        return model;
    }
}

module.exports = { nodeClass: StockyardChat };
