import request from '@/api/request'

export interface MCPToolParam {
  name: string
  type: string
  description: string
  required?: boolean
}

export interface MCPTool {
  name: string
  description: string
  parameters: MCPToolParam[]
}

export interface ToolCallResult {
  result: string
}

export async function listTools(): Promise<MCPTool[]> {
  const res: any = await request.get('/ai/tools')
  return res.data
}

export async function callTool(workspaceId: string, tool: string, input: Record<string, unknown>): Promise<ToolCallResult> {
  const res: any = await request.post('/ai/tools/call', { workspaceId, tool, input })
  return res.data
}
