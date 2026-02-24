import { ICredentialType, INodeProperties } from 'n8n-workflow';

export class LLMKitApi implements ICredentialType {
  name = 'llmKitApi';
  displayName = 'LLMKit API';
  documentationUrl = 'https://llmkit.dev/docs/n8n';
  properties: INodeProperties[] = [
    {
      displayName: 'Proxy URL',
      name: 'baseUrl',
      type: 'string',
      default: 'http://localhost:4000',
      placeholder: 'http://localhost:4000',
      description: 'LLMKit proxy base URL',
    },
    {
      displayName: 'API Key',
      name: 'apiKey',
      type: 'string',
      typeOptions: { password: true },
      default: 'llmkit',
      description: 'API key for LLMKit proxy (any non-empty string if auth is disabled)',
    },
  ];
}
