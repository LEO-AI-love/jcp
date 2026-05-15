# JCP MCP Server

JCP MCP Server 是一个本地 `stdio` MCP 服务，用来把韭菜盘已有的数据能力直接暴露给 Codex。它的目标不是再启动一个 AI 分析后端，而是把行情、K线、盘口、F10、研报、热点和龙虎榜交给 Codex，由 Codex 自己完成分析，因此可以替代 OpenClaw 的 `/analyze` 位置。

## 适用场景

- 不想安装或运行 OpenClaw，但希望 Codex 能分析 A 股。
- 希望 Codex 直接调用 JCP 的本地工具，而不是通过 HTTP 分析接口。
- 希望把 JCP 当成本地股票数据工具箱，供多个 MCP 客户端使用。

## 构建

```bash
go build -o ./bin/jcp-mcp ./cmd/jcp-mcp
```

开发调试时也可以直接运行：

```bash
go run ./cmd/jcp-mcp
```

这个命令会在标准输入/输出上运行 MCP 协议，不会打开桌面窗口。

## Codex 配置示例

推荐先构建二进制，然后在 Codex 的 MCP 配置里指向它：

```toml
[mcp_servers.jcp]
command = "/绝对路径/jcp/bin/jcp-mcp"
```

如果只是临时开发，也可以让 Codex 直接用 `go run`：

```toml
[mcp_servers.jcp]
command = "go"
args = ["run", "/绝对路径/jcp/cmd/jcp-mcp"]
```

配置完成后重启 Codex，应该能看到一组 `jcp_` 前缀的工具。

## 工具列表

| 工具 | 作用 |
|------|------|
| `jcp_search_stocks` | 搜索股票，支持代码、名称和拼音 |
| `jcp_get_realtime_quotes` | 获取一只或多只股票实时行情 |
| `jcp_get_kline` | 获取分时、日线、周线、月线 K 线 |
| `jcp_get_orderbook` | 获取五档盘口 |
| `jcp_get_market_overview` | 获取交易状态和主要指数 |
| `jcp_get_news` | 获取财联社快讯 |
| `jcp_get_f10` | 获取 F10 栏目数据 |
| `jcp_get_research_reports` | 获取个股研报列表 |
| `jcp_get_report_content` | 获取研报正文和 PDF 链接 |
| `jcp_get_hot_trends` | 获取全网热点趋势 |
| `jcp_get_longhubang` | 获取龙虎榜列表或股票营业部明细 |
| `jcp_get_analysis_context` | 一次性获取股票分析上下文，用于替代 OpenClaw `/analyze` |
| `jcp_get_watchlist` | 读取本机 JCP 自选股 |

## 推荐分析流程

用户说“分析一下贵州茅台”时：

1. 先用 `jcp_search_stocks` 搜索股票代码。
2. 再用 `jcp_get_analysis_context` 拉取完整上下文。
3. Codex 根据返回的 `realtime`、`marketStatus`、`indices`、`kline`、`orderBook`、`valuation`、`f10`、`reports` 生成结论。
4. 如需更细，继续调用 `jcp_get_f10`、`jcp_get_longhubang`、`jcp_get_hot_trends` 做补充验证。

## 和 OpenClaw 的关系

OpenClaw 的原思路是：JCP 提供 `/analyze`，后端直接产出分析结论。

MCP Server 的新思路是：JCP 只提供可信数据工具，Codex 自己组织推理和输出。这样少了一层本地 AI 服务依赖，也更适合 Codex 的工具调用工作流。

## 注意事项

- 数据来自 JCP 现有服务，网络源包括通达信、新浪、东方财富、财联社和各热点平台。
- 所有工具都是只读工具，不会修改股票数据。
- `jcp_get_analysis_context` 会尽量返回部分数据；如果某个数据源失败，失败项会出现在 `errors` 字段里。
- `jcp_get_watchlist` 会读取本机 JCP 配置目录下的自选股。
- 如果 JCP 设置过代理，MCP Server 会尽量复用 JCP 配置里的代理设置。
