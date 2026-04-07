package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/NortonBen/ai-memory-go/engine"
	"github.com/NortonBen/ai-memory-go/internal/sessionid"
	"github.com/NortonBen/ai-memory-go/schema"
	"github.com/fatih/color"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/spf13/cobra"
)

var (
	mcpTransport string
	mcpListen    string
	mcpPath      string
)

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Chạy MCP server (Model Context Protocol) cho AI Memory",
	Long: `Expose bộ nhớ qua MCP để client (Cursor, Claude Desktop, v.v.) gọi công cụ.

Transport:
  stdio — mặc định; phù hợp subprocess trong IDE.
  http  — Streamable HTTP (mcp-go), mặc định :8080 và đường dẫn /mcp.

Cùng --config và --session như các lệnh khác.`,
	Run: runMCPServer,
}

func init() {
	rootCmd.AddCommand(mcpCmd)
	mcpCmd.Flags().StringVar(&mcpTransport, "transport", "stdio", `transport: "stdio" hoặc "http"`)
	mcpCmd.Flags().StringVar(&mcpListen, "listen", ":8080", `địa chỉ lắng nghe khi --transport=http (vd. ":8080")`)
	mcpCmd.Flags().StringVar(&mcpPath, "path", "/mcp", `đường dẫn endpoint MCP khi --transport=http`)
}

func runMCPServer(cmd *cobra.Command, args []string) {
	ctx := context.Background()
	eng, err := InitEngine(ctx)
	if err != nil {
		color.Red("Không khởi tạo engine: %v", err)
		os.Exit(1)
	}
	defer eng.Close()

	mcpSrv := newMemoryMCPServer(eng)

	switch strings.ToLower(strings.TrimSpace(mcpTransport)) {
	case "stdio", "":
		stdioSrv := server.NewStdioServer(mcpSrv)
		if err := stdioSrv.Listen(ctx, os.Stdin, os.Stdout); err != nil {
			color.Red("stdio MCP: %v", err)
			os.Exit(1)
		}
	case "http":
		httpSrv := server.NewStreamableHTTPServer(mcpSrv, server.WithEndpointPath(mcpPath))
		sigCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		go func() {
			<-sigCtx.Done()
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			if err := httpSrv.Shutdown(shutdownCtx); err != nil {
				log.Printf("mcp http shutdown: %v", err)
			}
		}()
		log.Printf("MCP HTTP: http://%s%s (streamable)", trimListenForLog(mcpListen), mcpPath)
		if err := httpSrv.Start(mcpListen); err != nil {
			color.Red("HTTP MCP: %v", err)
			os.Exit(1)
		}
	default:
		color.Red(`transport không hợp lệ %q (dùng "stdio" hoặc "http")`, mcpTransport)
		os.Exit(1)
	}
}

func trimListenForLog(addr string) string {
	if strings.HasPrefix(addr, ":") {
		return "localhost" + addr
	}
	return addr
}

func newMemoryMCPServer(eng engine.MemoryEngine) *server.MCPServer {
	srv := server.NewMCPServer(
		"github.com/NortonBen/ai-memory-go/mcp",
		"0.1.0",
		server.WithToolCapabilities(true),
	)

	searchTool := mcp.NewTool("memory_search",
		mcp.WithDescription("Tìm kiếm ngữ nghĩa trong kho memory (theo session CLI hoặc session_id tùy chọn)."),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithString("query", mcp.Description("Câu hỏi hoặc từ khóa tìm kiếm"), mcp.Required()),
		mcp.WithNumber("limit", mcp.Description("Số kết quả tối đa"), mcp.DefaultNumber(5)),
		mcp.WithString("mode", mcp.Description(`Chế độ: "semantic", "graph", hoặc "hybrid"`)),
		mcp.WithString("session_id", mcp.Description("Ghi đè session tìm kiếm; để trống dùng --session của CLI")),
	)
	srv.AddTool(searchTool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := req.GetArguments()
		q := argString(args, "query")
		if q == "" {
			return mcp.NewToolResultError("thiếu query"), nil
		}
		limit := argInt(args, "limit", 5)
		if limit <= 0 {
			limit = 5
		}
		mode := parseRetrievalMode(argString(args, "mode"))
		sid := argString(args, "session_id")
		if sid == "" {
			sid = GetSessionID()
		}
		sq := &schema.SearchQuery{
			Text:      q,
			SessionID: sid,
			Mode:      mode,
			Limit:     limit,
		}
		res, err := eng.Search(ctx, sq)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return mcp.NewToolResultJSON(res)
	})

	addTool := mcp.NewTool("memory_add",
		mcp.WithDescription("Thêm nội dung text vào memory; sau đó tự Cognify đồng bộ (embedding + trích graph) như khi pipeline hoàn tất."),
		mcp.WithString("content", mcp.Description("Nội dung cần lưu"), mcp.Required()),
		mcp.WithBoolean("global", mcp.Description("true = bản ghi shared (session_id rỗng), giống session global trên CLI")),
		mcp.WithString("session_id", mcp.Description("Ghi đè session lưu trữ khi global=false")),
		mcp.WithString("tier", mcp.Description("Phân vùng: core, general, data, storage")),
		mcp.WithString("labels", mcp.Description("Nhãn phân cách bằng dấu phẩy")),
	)
	srv.AddTool(addTool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := req.GetArguments()
		content := argString(args, "content")
		if content == "" {
			return mcp.NewToolResultError("thiếu content"), nil
		}
		addOpts, err := addOptionsFromMCPArgs(args)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		dp, err := eng.Add(ctx, content, addOpts...)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		_, cognifyErr := eng.Cognify(ctx, dp, engine.WithWaitCognify(true))
		out := map[string]any{
			"id":                 dp.ID,
			"session_id":         dp.SessionID,
			"created_at":         dp.CreatedAt,
			"processing_status":  dp.ProcessingStatus,
			"cognify_completed":  cognifyErr == nil,
			"cognify_error":      errStr(cognifyErr),
		}
		return mcp.NewToolResultJSON(out)
	})

	thinkTool := mcp.NewTool("memory_think",
		mcp.WithDescription("Think: suy luận đa bước trên graph/context (tương đương lệnh think)."),
		mcp.WithString("query", mcp.Description("Câu hỏi / yêu cầu"), mcp.Required()),
		mcp.WithNumber("limit", mcp.Description("Số kết quả ngữ cảnh ban đầu"), mcp.DefaultNumber(5)),
		mcp.WithNumber("steps", mcp.Description("Số bước thinking tối đa trong graph"), mcp.DefaultNumber(3)),
		mcp.WithBoolean("reasoning", mcp.Description("Trả về trace suy luận (mặc định true)")),
		mcp.WithBoolean("learn", mcp.Description("Tự học quan hệ nối (mặc định true)")),
		mcp.WithNumber("max_context_chars", mcp.Description("Giới hạn ký tự mỗi segment ngữ cảnh"), mcp.DefaultNumber(4000)),
		mcp.WithBoolean("segment", mcp.Description("Xử lý ngữ cảnh lớn theo segment (mặc định true)")),
		mcp.WithBoolean("analyze", mcp.Description("Phân tích query bằng LLM trước khi retrieve (mặc định false)")),
		mcp.WithBoolean("four_tier", mcp.Description("Bật tìm kiếm bốn tầng trong bước retrieve")),
		mcp.WithString("session_id", mcp.Description("Ghi đè session; để trống dùng --session CLI")),
	)
	srv.AddTool(thinkTool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := req.GetArguments()
		q := argString(args, "query")
		if q == "" {
			return mcp.NewToolResultError("thiếu query"), nil
		}
		sid := argString(args, "session_id")
		if sid == "" {
			sid = GetSessionID()
		}
		tq := &schema.ThinkQuery{
			Text:               q,
			SessionID:          sid,
			Limit:              argInt(args, "limit", 5),
			EnableThinking:     true,
			MaxThinkingSteps:   argInt(args, "steps", 3),
			IncludeReasoning:   argBoolDefault(args, "reasoning", true),
			LearnRelationships: argBoolDefault(args, "learn", true),
			MaxContextLength:   argInt(args, "max_context_chars", 4000),
			SegmentContext:     argBoolDefault(args, "segment", true),
			AnalyzeQuery:       argBoolDefault(args, "analyze", false),
		}
		if argBool(args, "four_tier") {
			en := true
			tq.FourTier = &schema.FourTierSearchOptions{Enabled: &en}
		}
		resp, err := eng.Think(ctx, tq)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return mcp.NewToolResultJSON(resp)
	})

	requestTool := mcp.NewTool("memory_request",
		mcp.WithDescription("Request: tin nhắn hội thoại — intent, cập nhật memory, trả lời (tương đương lệnh request)."),
		mcp.WithString("message", mcp.Description("Nội dung tin nhắn người dùng"), mcp.Required()),
		mcp.WithNumber("hop_depth", mcp.Description("Số hop graph khi retrieve"), mcp.DefaultNumber(2)),
		mcp.WithBoolean("reasoning", mcp.Description("Gồm bước suy luận retrieve (mặc định true)")),
		mcp.WithBoolean("learn", mcp.Description("Học quan hệ nối tự động (mặc định true)")),
		mcp.WithBoolean("four_tier", mcp.Description("Bật four-tier cho intent query (ghi đè mặc định engine)")),
		mcp.WithString("session_id", mcp.Description("Ghi đè session; để trống dùng --session CLI")),
	)
	srv.AddTool(requestTool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := req.GetArguments()
		msg := argString(args, "message")
		if msg == "" {
			return mcp.NewToolResultError("thiếu message"), nil
		}
		sid := argString(args, "session_id")
		if sid == "" {
			sid = GetSessionID()
		}
		opts := []engine.RequestOption{
			engine.WithHopDepth(argInt(args, "hop_depth", 2)),
			engine.WithEnableThinking(true),
			engine.WithIncludeReasoning(argBoolDefault(args, "reasoning", true)),
			engine.WithLearnRelationships(argBoolDefault(args, "learn", true)),
		}
		if argBool(args, "four_tier") {
			en := true
			opts = append(opts, engine.WithRequestFourTier(&schema.FourTierSearchOptions{Enabled: &en}))
		}
		resp, err := eng.Request(ctx, sid, msg, opts...)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return mcp.NewToolResultJSON(resp)
	})

	deleteTool := mcp.NewTool("memory_delete",
		mcp.WithDescription("Xóa memory theo id hoặc xóa toàn bộ session (giống delete). Bắt buộc confirm=true."),
		mcp.WithDestructiveHintAnnotation(true),
		mcp.WithString("id", mcp.Description("ID datapoint cần xóa; để trống nếu xóa theo session")),
		mcp.WithString("session", mcp.Description(`Session cần xóa sạch (vd. tên workspace, hoặc "global"/"shared"/"_" cho pool chung)`)),
		mcp.WithBoolean("current_session", mcp.Description("true = dùng session đang hoạt động (-s / ~/.ai-memory/session.txt), giống --current")),
		mcp.WithBoolean("confirm", mcp.Description("Phải true để thực hiện xóa (an toàn cho MCP)"), mcp.Required()),
	)
	srv.AddTool(deleteTool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := req.GetArguments()
		if !argBool(args, "confirm") {
			return mcp.NewToolResultError("thiết lập confirm=true để xác nhận xóa"), nil
		}
		sessionArg := strings.TrimSpace(argString(args, "session"))
		if argBool(args, "current_session") && sessionArg == "" {
			sessionArg = GetSessionRaw()
		}
		id := strings.TrimSpace(argString(args, "id"))
		if id == "" && sessionArg == "" {
			return mcp.NewToolResultError("cần id, hoặc session, hoặc current_session=true"), nil
		}
		if err := eng.DeleteMemory(ctx, id, sessionArg); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		out := map[string]any{
			"deleted":       true,
			"id":            id,
			"session_scope": sessionArg,
		}
		return mcp.NewToolResultJSON(out)
	})

	return srv
}

func addOptionsFromMCPArgs(args map[string]any) ([]engine.AddOption, error) {
	var opts []engine.AddOption
	global := argBool(args, "global")
	explicitSID := argString(args, "session_id")

	switch {
	case global:
		opts = append(opts, engine.WithGlobalSession())
	case explicitSID != "":
		opts = append(opts, engine.WithSessionID(explicitSID))
	default:
		sid, g := sessionid.ForDataPointAdd(GetSessionRaw())
		if g {
			opts = append(opts, engine.WithGlobalSession())
		} else {
			opts = append(opts, engine.WithSessionID(sid))
		}
	}

	if t := argString(args, "tier"); t != "" {
		opts = append(opts, engine.WithMemoryTier(t))
	}
	if lab := argString(args, "labels"); lab != "" {
		var labs []string
		for _, p := range strings.Split(lab, ",") {
			if x := strings.TrimSpace(p); x != "" {
				labs = append(labs, x)
			}
		}
		if len(labs) > 0 {
			opts = append(opts, engine.WithLabels(labs...))
		}
	}
	return opts, nil
}

func parseRetrievalMode(s string) schema.RetrievalMode {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "graph":
		return schema.ModeGraphTraversal
	case "hybrid":
		return schema.ModeHybridSearch
	case "semantic", "":
		fallthrough
	default:
		return schema.ModeSemanticSearch
	}
}

func argString(args map[string]any, key string) string {
	if args == nil {
		return ""
	}
	v, ok := args[key]
	if !ok || v == nil {
		return ""
	}
	switch x := v.(type) {
	case string:
		return strings.TrimSpace(x)
	default:
		return strings.TrimSpace(fmt.Sprint(x))
	}
}

func argBool(args map[string]any, key string) bool {
	if args == nil {
		return false
	}
	v, ok := args[key]
	if !ok || v == nil {
		return false
	}
	switch x := v.(type) {
	case bool:
		return x
	case string:
		s := strings.TrimSpace(strings.ToLower(x))
		return s == "true" || s == "1" || s == "yes"
	default:
		return false
	}
}

// argBoolDefault giống argBool nhưng khi key không có hoặc null thì trả def (dùng cho cờ mặc định giống CLI).
func argBoolDefault(args map[string]any, key string, def bool) bool {
	if args == nil {
		return def
	}
	v, ok := args[key]
	if !ok || v == nil {
		return def
	}
	switch x := v.(type) {
	case bool:
		return x
	case string:
		s := strings.TrimSpace(strings.ToLower(x))
		if s == "true" || s == "1" || s == "yes" {
			return true
		}
		if s == "false" || s == "0" || s == "no" {
			return false
		}
		return def
	default:
		return def
	}
}

func errStr(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func argInt(args map[string]any, key string, def int) int {
	if args == nil {
		return def
	}
	v, ok := args[key]
	if !ok || v == nil {
		return def
	}
	switch x := v.(type) {
	case float64:
		return int(x)
	case int:
		return x
	case int64:
		return int(x)
	case json.Number:
		i, err := x.Int64()
		if err != nil {
			return def
		}
		return int(i)
	case string:
		n, err := strconv.Atoi(strings.TrimSpace(x))
		if err != nil {
			return def
		}
		return n
	default:
		return def
	}
}
