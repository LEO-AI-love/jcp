---
name: jcp-stock-analysis
description: A股智能分析系统，使用 JCP MCP Server 为 Codex 提供行情、F10、研报和热点工具，由 Codex 生成技术面、基本面、风险分析。
metadata:
  mcp:
    preferred_server: "jcp"
---

# JCP Stock Analysis

AI驱动的A股智能分析系统，通过多专家Agent协作提供全面的投资分析。

## 前置条件

1. 构建或运行 JCP MCP Server：`go build -o ./bin/jcp-mcp ./cmd/jcp-mcp`
2. 在 Codex MCP 配置中添加 `jcp` 服务，指向 `jcp-mcp`
3. 确认 Codex 可以看到 `jcp_` 前缀工具

## 使用方式

分析股票时，优先通过 MCP 工具获取上下文：

1. 用 `jcp_search_stocks` 搜索股票代码。
2. 用 `jcp_get_analysis_context` 获取实时行情、K线、盘口、F10、估值、研报和市场概览。
3. Codex 基于返回数据自行输出技术面、基本面、资金面、消息面和风险结论。

## MCP 工具

| 工具 | 说明 |
|------|------|
| `jcp_search_stocks` | 搜索股票 |
| `jcp_get_analysis_context` | 一次性获取分析上下文，替代 OpenClaw `/analyze` |
| `jcp_get_realtime_quotes` | 获取实时行情 |
| `jcp_get_kline` | 获取K线 |
| `jcp_get_orderbook` | 获取盘口 |
| `jcp_get_market_overview` | 获取市场状态和指数 |
| `jcp_get_f10` | 获取F10数据 |
| `jcp_get_research_reports` | 获取研报列表 |
| `jcp_get_report_content` | 获取研报正文 |
| `jcp_get_hot_trends` | 获取全网热点 |
| `jcp_get_longhubang` | 获取龙虎榜 |

## 示例

用户: 帮我分析一下贵州茅台

助手:

1. 调用 `jcp_search_stocks`，确认代码为 `sh600519`。
2. 调用 `jcp_get_analysis_context`，参数为 `{"code":"sh600519","query":"分析投资价值"}`。
3. 基于返回数据生成结论，不再调用 OpenClaw。

## 输出要求

- 明确说明数据时间和交易状态。
- 分开写技术面、基本面、资金/盘口、消息/舆情、风险。
- 不把工具数据直接当作投资建议，结论要带不确定性和风险提示。
