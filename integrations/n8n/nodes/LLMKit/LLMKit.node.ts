import {
  IExecuteFunctions,
  INodeExecutionData,
  INodeType,
  INodeTypeDescription,
  NodeOperationError,
} from 'n8n-workflow';

export class LLMKit implements INodeType {
  description: INodeTypeDescription = {
    displayName: 'LLMKit',
    name: 'llmKit',
    icon: 'file:llmkit.svg',
    group: ['transform'],
    version: 1,
    subtitle: '={{$parameter["operation"]}}',
    description: 'LLM proxy with cost caps, caching, rate limiting, and analytics',
    defaults: {
      name: 'LLMKit',
    },
    inputs: ['main'],
    outputs: ['main'],
    credentials: [
      {
        name: 'llmKitApi',
        required: true,
      },
    ],
    properties: [
      {
        displayName: 'Operation',
        name: 'operation',
        type: 'options',
        noDataExpression: true,
        options: [
          { name: 'Chat Completion', value: 'chatCompletion', description: 'Send a chat completion request through LLMKit proxy' },
          { name: 'Get Spend', value: 'getSpend', description: 'Get current spending for a project' },
          { name: 'Cache Stats', value: 'cacheStats', description: 'Get cache hit/miss statistics' },
          { name: 'Flush Cache', value: 'flushCache', description: 'Clear the response cache' },
          { name: 'Provider Status', value: 'providerStatus', description: 'Check health of all LLM providers' },
          { name: 'Analytics', value: 'analytics', description: 'Get usage analytics overview' },
          { name: 'Health Check', value: 'healthCheck', description: 'Check if LLMKit proxy is running' },
        ],
        default: 'chatCompletion',
      },
      // Chat Completion fields
      {
        displayName: 'Model',
        name: 'model',
        type: 'string',
        default: 'gpt-4o-mini',
        displayOptions: { show: { operation: ['chatCompletion'] } },
        description: 'Model to use (LLMKit may reroute based on ModelSwitch rules)',
      },
      {
        displayName: 'Messages',
        name: 'messages',
        type: 'json',
        default: '[{"role": "user", "content": "Hello!"}]',
        displayOptions: { show: { operation: ['chatCompletion'] } },
        description: 'Chat messages array (OpenAI format)',
      },
      {
        displayName: 'Simple Prompt',
        name: 'simplePrompt',
        type: 'string',
        default: '',
        displayOptions: { show: { operation: ['chatCompletion'] } },
        description: 'Simple text prompt (used if Messages is empty). Overrides Messages.',
      },
      {
        displayName: 'Max Tokens',
        name: 'maxTokens',
        type: 'number',
        default: 1000,
        displayOptions: { show: { operation: ['chatCompletion'] } },
      },
      {
        displayName: 'Temperature',
        name: 'temperature',
        type: 'number',
        default: 0.7,
        typeOptions: { minValue: 0, maxValue: 2, numberPrecision: 1 },
        displayOptions: { show: { operation: ['chatCompletion'] } },
      },
      // Spend fields
      {
        displayName: 'Project',
        name: 'project',
        type: 'string',
        default: 'default',
        displayOptions: { show: { operation: ['getSpend'] } },
      },
      // Analytics fields
      {
        displayName: 'Period',
        name: 'period',
        type: 'options',
        options: [
          { name: '1 Hour', value: '1h' },
          { name: '24 Hours', value: '24h' },
          { name: '7 Days', value: '7d' },
          { name: '30 Days', value: '30d' },
        ],
        default: '24h',
        displayOptions: { show: { operation: ['analytics'] } },
      },
      // User/feature headers
      {
        displayName: 'User ID',
        name: 'userId',
        type: 'string',
        default: '',
        displayOptions: { show: { operation: ['chatCompletion'] } },
        description: 'Optional user ID for UsagePulse metering',
      },
      {
        displayName: 'Feature',
        name: 'feature',
        type: 'string',
        default: 'n8n-workflow',
        displayOptions: { show: { operation: ['chatCompletion'] } },
        description: 'Feature tag for usage tracking',
      },
    ],
  };

  async execute(this: IExecuteFunctions): Promise<INodeExecutionData[][]> {
    const items = this.getInputData();
    const returnData: INodeExecutionData[] = [];
    const credentials = await this.getCredentials('llmKitApi');
    const baseUrl = (credentials.baseUrl as string).replace(/\/$/, '');
    const apiKey = credentials.apiKey as string;

    const headers: Record<string, string> = {
      'Content-Type': 'application/json',
      Authorization: `Bearer ${apiKey}`,
    };

    for (let i = 0; i < items.length; i++) {
      try {
        const operation = this.getNodeParameter('operation', i) as string;
        let responseData: any;

        switch (operation) {
          case 'chatCompletion': {
            const model = this.getNodeParameter('model', i) as string;
            const simplePrompt = this.getNodeParameter('simplePrompt', i, '') as string;
            const maxTokens = this.getNodeParameter('maxTokens', i) as number;
            const temperature = this.getNodeParameter('temperature', i) as number;
            const userId = this.getNodeParameter('userId', i, '') as string;
            const feature = this.getNodeParameter('feature', i, 'n8n-workflow') as string;

            let messages: any[];
            if (simplePrompt) {
              messages = [{ role: 'user', content: simplePrompt }];
            } else {
              const messagesJson = this.getNodeParameter('messages', i) as string;
              messages = typeof messagesJson === 'string' ? JSON.parse(messagesJson) : messagesJson;
            }

            const reqHeaders = { ...headers };
            if (userId) reqHeaders['X-LLMKit-User'] = userId;
            if (feature) reqHeaders['X-LLMKit-Feature'] = feature;

            const response = await this.helpers.httpRequest({
              method: 'POST',
              url: `${baseUrl}/v1/chat/completions`,
              headers: reqHeaders,
              body: { model, messages, max_tokens: maxTokens, temperature, stream: false },
            });

            responseData = {
              content: response.choices?.[0]?.message?.content || '',
              model: response.model,
              usage: response.usage,
              id: response.id,
              full_response: response,
            };
            break;
          }

          case 'getSpend': {
            const project = this.getNodeParameter('project', i) as string;
            responseData = await this.helpers.httpRequest({
              method: 'GET',
              url: `${baseUrl}/api/spend?project=${project}`,
              headers,
            });
            break;
          }

          case 'cacheStats': {
            responseData = await this.helpers.httpRequest({
              method: 'GET', url: `${baseUrl}/api/cache/stats`, headers,
            });
            break;
          }

          case 'flushCache': {
            responseData = await this.helpers.httpRequest({
              method: 'POST', url: `${baseUrl}/api/cache/flush`, headers, body: {},
            });
            break;
          }

          case 'providerStatus': {
            responseData = await this.helpers.httpRequest({
              method: 'GET', url: `${baseUrl}/api/providers/status`, headers,
            });
            break;
          }

          case 'analytics': {
            const period = this.getNodeParameter('period', i) as string;
            responseData = await this.helpers.httpRequest({
              method: 'GET', url: `${baseUrl}/api/analytics/overview?period=${period}`, headers,
            });
            break;
          }

          case 'healthCheck': {
            responseData = await this.helpers.httpRequest({
              method: 'GET', url: `${baseUrl}/health`, headers,
            });
            break;
          }
        }

        returnData.push({ json: responseData });
      } catch (error) {
        if (this.continueOnFail()) {
          returnData.push({ json: { error: (error as Error).message } });
          continue;
        }
        throw new NodeOperationError(this.getNode(), error as Error, { itemIndex: i });
      }
    }

    return [returnData];
  }
}
