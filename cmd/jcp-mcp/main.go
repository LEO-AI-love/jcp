package main

import (
	"context"
	"fmt"
	"log"
	"regexp"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/run-bigpig/jcp/internal/models"
	"github.com/run-bigpig/jcp/internal/pkg/paths"
	"github.com/run-bigpig/jcp/internal/pkg/proxy"
	"github.com/run-bigpig/jcp/internal/services"
	"github.com/run-bigpig/jcp/internal/services/hottrend"
)

const serverVersion = "0.1.0"

var (
	stockCodePattern  = regexp.MustCompile(`(?i)\b(?:sh|sz|bj)\d{6}\b`)
	sixDigitCodeRegex = regexp.MustCompile(`\b\d{6}\b`)
)

type serverDeps struct {
	config     *services.ConfigService
	market     *services.MarketService
	news       *services.NewsService
	f10        *services.F10Service
	reports    *services.ResearchReportService
	hotTrend   *hottrend.HotTrendService
	longHuBang *services.LongHuBangService
}

type searchStocksArgs struct {
	Keyword string `json:"keyword" jsonschema:"股票代码、名称或拼音关键词，例如 贵州茅台、600519、maotai"`
	Limit   int    `json:"limit,omitempty" jsonschema:"返回数量，默认10，最多50"`
}

type realtimeArgs struct {
	Code  string   `json:"code,omitempty" jsonschema:"单只股票代码，例如 sh600519；也支持 600519"`
	Codes []string `json:"codes,omitempty" jsonschema:"多只股票代码列表，例如 [\"sh600519\",\"sz000001\"]"`
}

type klineArgs struct {
	Code   string `json:"code" jsonschema:"股票代码，例如 sh600519；也支持 600519"`
	Period string `json:"period,omitempty" jsonschema:"K线周期：1m、1d、1w、1mo；默认1d"`
	Days   int    `json:"days,omitempty" jsonschema:"获取天数，默认30，最多500"`
}

type orderBookArgs struct {
	Code string `json:"code" jsonschema:"股票代码，例如 sh600519；也支持 600519"`
}

type emptyArgs struct{}

type newsArgs struct {
	Limit int `json:"limit,omitempty" jsonschema:"返回快讯数量，默认20，最多100"`
}

type f10Args struct {
	Code    string `json:"code" jsonschema:"股票代码，例如 sh600519；也支持 600519"`
	Section string `json:"section,omitempty" jsonschema:"F10栏目：overview、valuation、operations、core_themes、main_indicators、management、capital_operation、equity_structure、related_stocks、valuation_trend、company_survey、financial_statements、performance_events、fund_flow、institutional_holdings；默认overview"`
	Range   string `json:"range,omitempty" jsonschema:"估值趋势区间，仅 section=valuation_trend 时使用，例如 1y、3y、5y、all"`
}

type researchReportsArgs struct {
	Code     string `json:"code" jsonschema:"股票代码，例如 sh600519；也支持 600519"`
	PageSize int    `json:"pageSize,omitempty" jsonschema:"每页数量，默认10，最多50"`
	PageNo   int    `json:"pageNo,omitempty" jsonschema:"页码，默认1"`
}

type reportContentArgs struct {
	InfoCode string `json:"infoCode" jsonschema:"研报唯一标识码，可从 jcp_get_research_reports 的 infoCode 字段获得"`
}

type hotTrendArgs struct {
	Platform  string   `json:"platform,omitempty" jsonschema:"单个平台：weibo、zhihu、bilibili、baidu、douyin、toutiao；为空则返回所有平台"`
	Platforms []string `json:"platforms,omitempty" jsonschema:"多个平台列表；为空且 platform 为空则返回所有平台"`
}

type longHuBangArgs struct {
	PageSize   int    `json:"pageSize,omitempty" jsonschema:"每页数量，默认50，最多200"`
	PageNumber int    `json:"pageNumber,omitempty" jsonschema:"页码，默认1"`
	TradeDate  string `json:"tradeDate,omitempty" jsonschema:"交易日期，格式 YYYY-MM-DD；为空则取最新列表"`
	Code       string `json:"code,omitempty" jsonschema:"可选，股票代码；填写后返回该股票在指定 tradeDate 的营业部明细"`
}

type analysisContextArgs struct {
	Code             string `json:"code" jsonschema:"股票代码，例如 sh600519；也支持 600519"`
	Query            string `json:"query,omitempty" jsonschema:"用户的分析问题，原样返回给 Codex 作为上下文"`
	KLinePeriod      string `json:"klinePeriod,omitempty" jsonschema:"K线周期，默认1d"`
	KLineDays        int    `json:"klineDays,omitempty" jsonschema:"K线天数，默认60，最多500"`
	ReportLimit      int    `json:"reportLimit,omitempty" jsonschema:"研报数量，默认5，最多20"`
	IncludeHotTrends bool   `json:"includeHotTrends,omitempty" jsonschema:"是否附带全网热点，默认false"`
}

func main() {
	deps := newServerDeps()
	server := mcp.NewServer(&mcp.Implementation{
		Name:    "jcp-mcp",
		Version: serverVersion,
	}, nil)

	registerTools(server, deps)

	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Printf("jcp-mcp server failed: %v", err)
	}
}

func newServerDeps() *serverDeps {
	if config, err := services.NewConfigService(paths.GetDataDir()); err == nil {
		if appConfig := config.GetConfig(); appConfig != nil {
			proxy.GetManager().SetConfig(&appConfig.Proxy)
		}
		return buildDeps(config)
	} else {
		log.Printf("jcp-mcp: load JCP config failed, using default network settings: %v", err)
		return buildDeps(nil)
	}
}

func buildDeps(config *services.ConfigService) *serverDeps {
	hotTrendService, err := hottrend.NewHotTrendService()
	if err != nil {
		log.Printf("jcp-mcp: hot trend cache initialization failed: %v", err)
	}

	return &serverDeps{
		config:     config,
		market:     services.NewMarketService(),
		news:       services.NewNewsService(),
		f10:        services.NewF10Service(),
		reports:    services.NewResearchReportService(),
		hotTrend:   hotTrendService,
		longHuBang: services.NewLongHuBangService(),
	}
}

func registerTools(server *mcp.Server, deps *serverDeps) {
	mcp.AddTool(server, tool("jcp_search_stocks", "搜索A股股票，支持代码、名称和拼音关键词。", "搜索股票"), func(ctx context.Context, req *mcp.CallToolRequest, args searchStocksArgs) (*mcp.CallToolResult, map[string]any, error) {
		keyword := strings.TrimSpace(args.Keyword)
		if keyword == "" {
			return nil, nil, fmt.Errorf("请提供搜索关键词，例如 股票名称、代码或拼音")
		}
		limit := clamp(args.Limit, 10, 1, 50)
		return nil, map[string]any{
			"keyword": keyword,
			"limit":   limit,
			"stocks":  deps.market.SearchStocks(keyword, limit),
		}, nil
	})

	mcp.AddTool(server, tool("jcp_get_realtime_quotes", "获取一只或多只A股的实时行情，包括价格、涨跌幅、成交量、成交额、最高最低等。", "实时行情"), func(ctx context.Context, req *mcp.CallToolRequest, args realtimeArgs) (*mcp.CallToolResult, map[string]any, error) {
		codes := normalizeStockCodes(args.Code, args.Codes)
		if len(codes) == 0 {
			return nil, nil, fmt.Errorf("请提供股票代码，例如 sh600519 或 600519")
		}
		stocks, err := deps.market.GetStockRealTimeData(codes...)
		if err != nil {
			return nil, nil, fmt.Errorf("获取实时行情失败: %w", err)
		}
		return nil, map[string]any{
			"codes":  codes,
			"stocks": stocks,
		}, nil
	})

	mcp.AddTool(server, tool("jcp_get_kline", "获取股票K线数据，支持分时、日线、周线、月线。适合技术面分析。", "K线数据"), func(ctx context.Context, req *mcp.CallToolRequest, args klineArgs) (*mcp.CallToolResult, map[string]any, error) {
		code := normalizeStockCode(args.Code)
		if code == "" {
			return nil, nil, fmt.Errorf("请提供股票代码，例如 sh600519 或 600519")
		}
		period := strings.TrimSpace(args.Period)
		if period == "" {
			period = "1d"
		}
		days := clamp(args.Days, 30, 1, 500)
		klines, err := deps.market.GetKLineData(code, period, days)
		if err != nil {
			return nil, nil, fmt.Errorf("获取K线数据失败: %w", err)
		}
		return nil, map[string]any{
			"code":   code,
			"period": period,
			"days":   days,
			"klines": klines,
		}, nil
	})

	mcp.AddTool(server, tool("jcp_get_orderbook", "获取股票五档盘口数据，包括买卖盘价格、挂单量和占比。", "盘口数据"), func(ctx context.Context, req *mcp.CallToolRequest, args orderBookArgs) (*mcp.CallToolResult, map[string]any, error) {
		code := normalizeStockCode(args.Code)
		if code == "" {
			return nil, nil, fmt.Errorf("请提供股票代码，例如 sh600519 或 600519")
		}
		orderBook, err := deps.market.GetRealOrderBook(code)
		if err != nil {
			return nil, nil, fmt.Errorf("获取盘口数据失败: %w", err)
		}
		return nil, map[string]any{
			"code":      code,
			"orderBook": orderBook,
		}, nil
	})

	mcp.AddTool(server, tool("jcp_get_market_overview", "获取A股市场交易状态和主要指数行情。适合判断大盘环境。", "市场概览"), func(ctx context.Context, req *mcp.CallToolRequest, args emptyArgs) (*mcp.CallToolResult, map[string]any, error) {
		indices, err := deps.market.GetMarketIndices()
		if err != nil {
			return nil, nil, fmt.Errorf("获取大盘指数失败: %w", err)
		}
		return nil, map[string]any{
			"status":  deps.market.GetMarketStatus(),
			"indices": indices,
		}, nil
	})

	mcp.AddTool(server, tool("jcp_get_news", "获取财联社快讯，用于政策面、消息面和突发事件分析。", "市场快讯"), func(ctx context.Context, req *mcp.CallToolRequest, args newsArgs) (*mcp.CallToolResult, map[string]any, error) {
		news, err := deps.news.GetTelegraphList()
		if err != nil {
			return nil, nil, fmt.Errorf("获取快讯失败: %w", err)
		}
		limit := clamp(args.Limit, 20, 1, 100)
		if len(news) > limit {
			news = news[:limit]
		}
		return nil, map[string]any{
			"limit": limit,
			"news":  news,
		}, nil
	})

	mcp.AddTool(server, tool("jcp_get_f10", "获取东方财富F10资料，可按栏目读取公司概况、估值、财务、题材、股东、资金流等。", "F10资料"), func(ctx context.Context, req *mcp.CallToolRequest, args f10Args) (*mcp.CallToolResult, map[string]any, error) {
		code := normalizeStockCode(args.Code)
		if code == "" {
			return nil, nil, fmt.Errorf("请提供股票代码，例如 sh600519 或 600519")
		}
		section := strings.TrimSpace(args.Section)
		if section == "" {
			section = "overview"
		}

		data, err := getF10Section(deps.f10, code, section, args.Range)
		if err != nil {
			return nil, nil, err
		}
		return nil, map[string]any{
			"code":    code,
			"section": section,
			"data":    data,
		}, nil
	})

	mcp.AddTool(server, tool("jcp_get_research_reports", "获取个股研报列表，包括标题、券商、评级、预测EPS/PE、发布日期和 infoCode。", "研报列表"), func(ctx context.Context, req *mcp.CallToolRequest, args researchReportsArgs) (*mcp.CallToolResult, map[string]any, error) {
		code := normalizeStockCode(args.Code)
		if code == "" {
			return nil, nil, fmt.Errorf("请提供股票代码，例如 sh600519 或 600519")
		}
		pageSize := clamp(args.PageSize, 10, 1, 50)
		pageNo := clamp(args.PageNo, 1, 1, 1000)
		reports, err := deps.reports.GetResearchReports(rawStockCode(code), pageSize, pageNo)
		if err != nil {
			return nil, nil, fmt.Errorf("获取研报列表失败: %w", err)
		}
		return nil, map[string]any{
			"code":     code,
			"pageSize": pageSize,
			"pageNo":   pageNo,
			"reports":  reports,
		}, nil
	})

	mcp.AddTool(server, tool("jcp_get_report_content", "根据研报 infoCode 获取研报正文摘要和PDF链接。", "研报正文"), func(ctx context.Context, req *mcp.CallToolRequest, args reportContentArgs) (*mcp.CallToolResult, map[string]any, error) {
		infoCode := strings.TrimSpace(args.InfoCode)
		if infoCode == "" {
			return nil, nil, fmt.Errorf("请提供 infoCode，可先调用 jcp_get_research_reports 获取")
		}
		content, err := deps.reports.GetReportContent(infoCode)
		if err != nil {
			return nil, nil, fmt.Errorf("获取研报正文失败: %w", err)
		}
		return nil, map[string]any{
			"infoCode": infoCode,
			"report":   content,
		}, nil
	})

	mcp.AddTool(server, tool("jcp_get_hot_trends", "获取微博、知乎、B站、百度、抖音、头条等平台热榜。适合判断全网情绪和题材热度。", "全网热点"), func(ctx context.Context, req *mcp.CallToolRequest, args hotTrendArgs) (*mcp.CallToolResult, map[string]any, error) {
		if deps.hotTrend == nil {
			return nil, nil, fmt.Errorf("热点服务未初始化，请检查本地缓存目录权限")
		}
		platforms := normalizePlatforms(args.Platform, args.Platforms)
		if len(platforms) == 0 {
			return nil, map[string]any{
				"platforms": deps.hotTrend.GetPlatforms(),
				"trends":    deps.hotTrend.GetAllHotTrends(),
			}, nil
		}
		return nil, map[string]any{
			"platforms": platforms,
			"trends":    deps.hotTrend.GetHotTrends(platforms),
		}, nil
	})

	mcp.AddTool(server, tool("jcp_get_longhubang", "获取龙虎榜列表，或查询指定股票在指定交易日的营业部买卖明细。", "龙虎榜"), func(ctx context.Context, req *mcp.CallToolRequest, args longHuBangArgs) (*mcp.CallToolResult, map[string]any, error) {
		code := normalizeStockCode(args.Code)
		if code != "" {
			if strings.TrimSpace(args.TradeDate) == "" {
				return nil, nil, fmt.Errorf("查询股票龙虎榜明细时必须提供 tradeDate，格式 YYYY-MM-DD")
			}
			detail, err := deps.longHuBang.GetStockDetail(rawStockCode(code), args.TradeDate)
			if err != nil {
				return nil, nil, fmt.Errorf("获取龙虎榜明细失败: %w", err)
			}
			return nil, map[string]any{
				"code":      code,
				"tradeDate": args.TradeDate,
				"detail":    detail,
			}, nil
		}

		pageSize := clamp(args.PageSize, 50, 1, 200)
		pageNumber := clamp(args.PageNumber, 1, 1, 1000)
		list, err := deps.longHuBang.GetLongHuBangList(pageSize, pageNumber, args.TradeDate)
		if err != nil {
			return nil, nil, fmt.Errorf("获取龙虎榜列表失败: %w", err)
		}
		return nil, map[string]any{
			"pageSize":   pageSize,
			"pageNumber": pageNumber,
			"tradeDate":  strings.TrimSpace(args.TradeDate),
			"list":       list,
		}, nil
	})

	mcp.AddTool(server, tool("jcp_get_analysis_context", "一次性获取股票分析常用上下文：实时行情、市场概览、K线、盘口、估值、F10概要和近期研报。Codex 可基于这些数据自行完成分析，用来替代 OpenClaw 的 /analyze。", "分析上下文"), func(ctx context.Context, req *mcp.CallToolRequest, args analysisContextArgs) (*mcp.CallToolResult, map[string]any, error) {
		code := normalizeStockCode(args.Code)
		if code == "" {
			return nil, nil, fmt.Errorf("请提供股票代码，例如 sh600519 或 600519")
		}

		period := strings.TrimSpace(args.KLinePeriod)
		if period == "" {
			period = "1d"
		}
		klineDays := clamp(args.KLineDays, 60, 1, 500)
		reportLimit := clamp(args.ReportLimit, 5, 1, 20)

		errors := map[string]string{}
		realtime, err := deps.market.GetStockRealTimeData(code)
		if err != nil {
			errors["realtime"] = err.Error()
		}
		indices, err := deps.market.GetMarketIndices()
		if err != nil {
			errors["indices"] = err.Error()
		}
		klines, err := deps.market.GetKLineData(code, period, klineDays)
		if err != nil {
			errors["kline"] = err.Error()
		}
		orderBook, err := deps.market.GetRealOrderBook(code)
		if err != nil {
			errors["orderBook"] = err.Error()
		}
		valuation, err := deps.f10.GetValuationByCode(code)
		if err != nil {
			errors["valuation"] = err.Error()
		}
		overview, err := deps.f10.GetOverview(code)
		if err != nil {
			errors["f10"] = err.Error()
		}
		reports, err := deps.reports.GetResearchReports(rawStockCode(code), reportLimit, 1)
		if err != nil {
			errors["reports"] = err.Error()
		}

		result := map[string]any{
			"code":         code,
			"query":        strings.TrimSpace(args.Query),
			"generatedAt":  time.Now().Format(time.RFC3339),
			"summaryHint":  buildAnalysisHint(code, realtime),
			"realtime":     realtime,
			"marketStatus": deps.market.GetMarketStatus(),
			"indices":      indices,
			"kline": map[string]any{
				"period": period,
				"days":   klineDays,
				"data":   klines,
			},
			"orderBook": orderBook,
			"valuation": valuation,
			"f10":       overview,
			"reports":   reports,
		}

		if args.IncludeHotTrends && deps.hotTrend != nil {
			result["hotTrends"] = deps.hotTrend.GetAllHotTrends()
		}
		if len(errors) > 0 {
			result["errors"] = errors
		}

		return nil, result, nil
	})

	mcp.AddTool(server, tool("jcp_get_watchlist", "读取本机JCP自选股列表。", "自选股"), func(ctx context.Context, req *mcp.CallToolRequest, args emptyArgs) (*mcp.CallToolResult, map[string]any, error) {
		if deps.config == nil {
			return nil, nil, fmt.Errorf("JCP配置未加载，无法读取自选股")
		}
		return nil, map[string]any{
			"watchlist": deps.config.GetWatchlist(),
		}, nil
	})
}

func tool(name, description, title string) *mcp.Tool {
	openWorld := true
	destructive := false
	return &mcp.Tool{
		Name:        name,
		Title:       title,
		Description: description,
		Annotations: &mcp.ToolAnnotations{
			Title:           title,
			ReadOnlyHint:    true,
			DestructiveHint: &destructive,
			OpenWorldHint:   &openWorld,
		},
	}
}

func getF10Section(service *services.F10Service, code string, section string, rangeKey string) (any, error) {
	switch section {
	case "overview":
		return service.GetOverview(code)
	case "valuation":
		return service.GetValuationByCode(code)
	case "operations":
		return service.GetOperationsRequired(code)
	case "core_themes":
		return service.GetCoreThemes(code)
	case "main_indicators":
		return service.GetMainIndicators(code)
	case "management":
		return service.GetManagement(code)
	case "capital_operation":
		return service.GetCapitalOperation(code)
	case "equity_structure":
		return service.GetEquityStructure(code)
	case "related_stocks":
		return service.GetRelatedStocks(code)
	case "valuation_trend":
		if strings.TrimSpace(rangeKey) == "" {
			rangeKey = "1y"
		}
		return service.GetValuationTrend(code, rangeKey)
	case "company_survey":
		return service.GetCompanySurveyByCode(code)
	case "financial_statements":
		return service.GetFinancialStatementsByCode(code)
	case "performance_events":
		return service.GetPerformanceEventsByCode(code)
	case "fund_flow":
		return service.GetFundFlowByCode(code)
	case "institutional_holdings":
		return service.GetInstitutionalHoldingsByCode(code)
	default:
		return nil, fmt.Errorf("不支持的F10栏目 %q，请使用 overview、valuation、operations、core_themes、main_indicators、management、capital_operation、equity_structure、related_stocks、valuation_trend、company_survey、financial_statements、performance_events、fund_flow、institutional_holdings", section)
	}
}

func normalizeStockCodes(primary string, values []string) []string {
	rawValues := make([]string, 0, 1+len(values))
	if strings.TrimSpace(primary) != "" {
		rawValues = append(rawValues, primary)
	}
	rawValues = append(rawValues, values...)

	seen := make(map[string]struct{}, len(rawValues))
	codes := make([]string, 0, len(rawValues))
	for _, value := range rawValues {
		for _, segment := range splitCandidates(value) {
			code := normalizeStockCode(segment)
			if code == "" {
				continue
			}
			if _, ok := seen[code]; ok {
				continue
			}
			seen[code] = struct{}{}
			codes = append(codes, code)
		}
	}
	return codes
}

func normalizeStockCode(raw string) string {
	candidate := strings.ToLower(strings.TrimSpace(raw))
	if candidate == "" {
		return ""
	}
	candidate = strings.TrimPrefix(candidate, "s_")
	if match := stockCodePattern.FindString(candidate); match != "" {
		return strings.ToLower(match)
	}
	if digits := sixDigitCodeRegex.FindString(candidate); digits != "" {
		return inferStockCodePrefix(digits)
	}
	return ""
}

func inferStockCodePrefix(code string) string {
	if len(code) != 6 {
		return ""
	}
	switch code[0] {
	case '6':
		return "sh" + code
	case '0', '3':
		return "sz" + code
	case '4', '8':
		return "bj" + code
	default:
		return ""
	}
}

func rawStockCode(normalized string) string {
	if len(normalized) != 8 {
		return normalized
	}
	if strings.HasPrefix(normalized, "sh") || strings.HasPrefix(normalized, "sz") || strings.HasPrefix(normalized, "bj") {
		return normalized[2:]
	}
	return normalized
}

func splitCandidates(raw string) []string {
	return strings.FieldsFunc(raw, func(r rune) bool {
		switch r {
		case ',', '，', ';', '；', '|', '/', '\\', ' ', '\n', '\t':
			return true
		default:
			return false
		}
	})
}

func normalizePlatforms(primary string, values []string) []string {
	rawValues := make([]string, 0, 1+len(values))
	if strings.TrimSpace(primary) != "" {
		rawValues = append(rawValues, primary)
	}
	rawValues = append(rawValues, values...)

	seen := make(map[string]struct{}, len(rawValues))
	platforms := make([]string, 0, len(rawValues))
	for _, value := range rawValues {
		for _, segment := range splitCandidates(value) {
			platform := strings.ToLower(strings.TrimSpace(segment))
			if platform == "" {
				continue
			}
			if _, ok := seen[platform]; ok {
				continue
			}
			seen[platform] = struct{}{}
			platforms = append(platforms, platform)
		}
	}
	return platforms
}

func clamp(value, defaultValue, minValue, maxValue int) int {
	if value == 0 {
		return defaultValue
	}
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}

func buildAnalysisHint(code string, stocks []models.Stock) string {
	if len(stocks) == 0 {
		return fmt.Sprintf("已为 %s 拉取 JCP 分析上下文。请由 Codex 基于 realtime、marketStatus、indices、kline、orderBook、valuation、f10、reports 等字段自行生成分析结论；这些数据用于替代 OpenClaw /analyze 的输入层。", code)
	}
	stock := stocks[0]
	return fmt.Sprintf("已为 %s（%s）拉取 JCP 分析上下文。当前价 %.2f，涨跌幅 %.2f%%。请由 Codex 基于 realtime、marketStatus、indices、kline、orderBook、valuation、f10、reports 等字段自行生成分析结论；这些数据用于替代 OpenClaw /analyze 的输入层。", stock.Name, stock.Symbol, stock.Price, stock.ChangePercent)
}
