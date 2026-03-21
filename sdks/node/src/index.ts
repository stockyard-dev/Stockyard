/**
 * Stockyard Node.js/TypeScript SDK
 * Zero dependencies — uses native fetch.
 */

export interface StockyardConfig {
  baseUrl?: string;
  apiKey?: string;
}

export interface Trace {
  id: string;
  provider: string;
  model: string;
  status: string;
  duration_ms: number;
  cost_usd: number;
  tokens_in: number;
  tokens_out: number;
  created_at: string;
}

export interface Module {
  name: string;
  category: string;
  enabled: boolean;
  in_chain: boolean;
}

export interface ChatMessage {
  role: "system" | "user" | "assistant";
  content: string;
}

export class StockyardClient {
  private baseUrl: string;
  private apiKey: string;

  constructor(config: StockyardConfig = {}) {
    this.baseUrl = (config.baseUrl || process.env.STOCKYARD_URL || "http://localhost:4200").replace(/\/$/, "");
    this.apiKey = config.apiKey || process.env.STOCKYARD_ADMIN_KEY || "";
  }

  private async request(method: string, path: string, body?: any): Promise<any> {
    const headers: Record<string, string> = { "Content-Type": "application/json" };
    if (this.apiKey) {
      headers["Authorization"] = `Bearer ${this.apiKey}`;
      headers["X-Admin-Key"] = this.apiKey;
    }
    const resp = await fetch(`${this.baseUrl}${path}`, {
      method,
      headers,
      body: body ? JSON.stringify(body) : undefined,
    });
    return resp.json();
  }

  // --- Status ---
  async status(): Promise<any> { return this.request("GET", "/api/status"); }
  async apps(): Promise<any[]> { return (await this.request("GET", "/api/apps")).apps || []; }

  // --- Proxy ---
  async listModules(): Promise<Module[]> { return (await this.request("GET", "/api/proxy/modules")).modules || []; }
  async toggleModule(name: string, enabled: boolean): Promise<any> { return this.request("PUT", `/api/proxy/modules/${name}`, { enabled }); }
  async listProviders(): Promise<any[]> { return (await this.request("GET", "/api/proxy/providers")).providers || []; }
  async providerHealth(): Promise<any[]> { return (await this.request("GET", "/api/proxy/providers/health")).providers || []; }

  // --- Observe ---
  async listTraces(limit = 20, model?: string): Promise<Trace[]> {
    let path = `/api/observe/traces?limit=${limit}`;
    if (model) path += `&model=${model}`;
    return (await this.request("GET", path)).traces || [];
  }
  async getTrace(id: string): Promise<any> { return this.request("GET", `/api/observe/traces/${id}`); }
  async getCosts(period = "today"): Promise<any> { return this.request("GET", `/api/observe/costs?period=${period}`); }
  async getDrift(): Promise<any> { return this.request("GET", "/api/observe/drift"); }

  // --- Billing ---
  async listCustomers(): Promise<any[]> { return this.request("GET", "/api/billing/customers"); }
  async createCustomer(name: string, email: string): Promise<any> {
    return this.request("POST", "/api/billing/customers", { name, email, account_id: "default" });
  }
  async queryUsage(): Promise<any> { return this.request("GET", "/api/billing/usage/summary"); }
  async creditBalance(customerId: string): Promise<any> { return this.request("GET", `/api/billing/credits/balance?customer_id=${customerId}`); }

  // --- Team ---
  async listTeamMembers(): Promise<any[]> { return this.request("GET", "/api/team/members"); }
  async inviteMember(email: string, name: string, role = "viewer"): Promise<any> {
    return this.request("POST", "/api/team/members", { email, name, role });
  }

  // --- Config ---
  async exportConfig(): Promise<any> { return this.request("GET", "/api/config/export"); }
  async importConfig(config: any): Promise<any> { return this.request("POST", "/api/config/import", config); }

  // --- Routing ---
  async listRoutingRules(): Promise<any[]> { return (await this.request("GET", "/api/proxy/routing/rules")).rules || []; }
  async createRoutingRule(name: string, condition: any, action: any, priority = 0): Promise<any> {
    return this.request("POST", "/api/proxy/routing/rules", { name, condition, action, priority });
  }

  // --- Firewall ---
  async scanText(text: string): Promise<any> { return this.request("POST", "/api/firewall/scan", { text }); }
  async firewallStats(): Promise<any> { return this.request("GET", "/api/firewall/stats"); }

  // --- Chat ---
  async chat(model: string, messages: ChatMessage[], opts?: any): Promise<any> {
    return this.request("POST", "/v1/chat/completions", { model, messages, ...opts });
  }
}

export default StockyardClient;
