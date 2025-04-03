package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"gopkg.in/yaml.v3"
)

// 配置结构体
type Config struct {
	PrivateKey string `yaml:"privateKey"`
	BaseURL    string `yaml:"baseURL"`
	Sub        string `yaml:"sub"`
	Kid        string `yaml:"kid"`
}

type RootConfig struct {
	Weather Config `yaml:"WEATHER"`
}

// 全局变量
var (
	expireAt   int64
	encodedJWT string
	config     Config
)

// 城市信息结构体
type CityLocation struct {
	Name    string `json:"name"`
	ID      string `json:"id"`
	Lat     string `json:"lat"`
	Lon     string `json:"lon"`
	Adm2    string `json:"adm2"`
	Adm1    string `json:"adm1"`
	Country string `json:"country"`
}

// 城市查询响应
type CityLookupResponse struct {
	Code     string         `json:"code"`
	Location []CityLocation `json:"location"`
}

// 天气数据结构体
type WeatherNow struct {
	ObsTime   string `json:"obsTime"`
	Temp      string `json:"temp"`
	FeelsLike string `json:"feelsLike"`
	Icon      string `json:"icon"`
	Text      string `json:"text"`
	WindDir   string `json:"windDir"`
	WindScale string `json:"windScale"`
	WindSpeed string `json:"windSpeed"`
	Humidity  string `json:"humidity"`
	Precip    string `json:"precip"`
}

// 天气响应
type WeatherResponse struct {
	Code       string     `json:"code"`
	UpdateTime string     `json:"updateTime"`
	Now        WeatherNow `json:"now"`
}

// 生成JWT令牌
func getJWT() string {
	currentTime := time.Now().Unix()
	if encodedJWT != "" && currentTime < expireAt {
		return encodedJWT
	}

	expireAt = currentTime + 86400 - 100

	claims := jwt.MapClaims{
		"iat": currentTime - 30,
		"exp": expireAt,
		"sub": config.Sub,
	}

	// 创建Ed25519私钥
	key, err := jwt.ParseEdPrivateKeyFromPEM([]byte(config.PrivateKey))
	if err != nil {
		log.Fatalf("无法解析私钥: %v", err)
	}

	// 创建JWT
	token := jwt.NewWithClaims(jwt.SigningMethodEdDSA, claims)
	// 在现有header中添加kid，而不是完全覆盖
	token.Header["kid"] = config.Kid

	// 签名JWT
	encodedJWT, err = token.SignedString(key)
	if err != nil {
		log.Fatalf("无法签名token: %v", err)
	}
	//log.Printf("JWT令牌已生成:\n %s \n", encodedJWT)

	return encodedJWT
}

// 发送HTTP请求
func makeRequest(ctx context.Context, client *http.Client, url string, params map[string]string) ([]byte, error) {
	// 创建请求URL（添加查询参数）
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	// 添加查询参数
	q := req.URL.Query()
	for key, value := range params {
		q.Add(key, value)
	}
	req.URL.RawQuery = q.Encode()

	jwtToken := getJWT()
	// 设置头部
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", jwtToken))

	// 发送请求
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()

	// 检查响应状态码
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("HTTP错误，状态码: %d", resp.StatusCode)
	}

	// 解码响应内容
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	return body, nil
}

// 获取城市代码
func getCityCode(ctx context.Context, client *http.Client, city string) (string, error) {
	params := map[string]string{
		"location": city,
	}

	body, err := makeRequest(ctx, client, config.BaseURL+"/geo/v2/city/lookup", params)
	if err != nil {
		return "", err
	}

	var response CityLookupResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return "", fmt.Errorf("解析城市数据失败: %w", err)
	}

	if response.Code != "200" || len(response.Location) == 0 {
		return "", fmt.Errorf("未找到城市: %s", city)
	}

	return response.Location[0].ID, nil
}

// 创建MCP服务器
func NewWeatherServer() *server.MCPServer {
	mcpServer := server.NewMCPServer(
		"weather",
		"1.0.0",
		server.WithLogging(),
	)

	// 添加天气查询工具
	mcpServer.AddTool(mcp.NewTool("getWeatherNow",
		mcp.WithDescription("获取指定地区实时天气"),
		mcp.WithString("location",
			mcp.Description("查询的地区，格式为\"城市 区域\"，如\"广州 天河\"或仅\"广州\""),
			mcp.Required(),
		),
	), handleGetWeatherNow)

	return mcpServer
}

// 处理获取天气的请求
func handleGetWeatherNow(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	arguments := request.Params.Arguments
	location, ok := arguments["location"].(string)
	if !ok {
		return nil, fmt.Errorf("缺少location参数")
	}

	client := http.DefaultClient

	// 获取城市代码
	cityCode, err := getCityCode(ctx, client, location)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: fmt.Sprintf("输入的地区不存在，无法提供天气预报: %v", err),
				},
			},
		}, nil
	}

	// 获取天气信息
	params := map[string]string{
		"location": cityCode,
	}

	body, err := makeRequest(ctx, client, config.BaseURL+"/v7/weather/now", params)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: fmt.Sprintf("获取天气信息失败: %v", err),
				},
			},
		}, nil
	}

	var response WeatherResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: fmt.Sprintf("解析天气数据失败: %v", err),
				},
			},
		}, nil
	}

	if response.Code != "200" {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: "输入的地区不存在，无法提供天气预报",
				},
			},
		}, nil
	}

	result := fmt.Sprintf("当前%s的天气状况为：\n", location)
	result += fmt.Sprintf("温度：%s℃\n", response.Now.Temp)
	result += fmt.Sprintf("体感温度：%s℃\n", response.Now.FeelsLike)
	result += fmt.Sprintf("天气状况：%s\n", response.Now.Text)
	result += fmt.Sprintf("风向：%s\n", response.Now.WindDir)
	result += fmt.Sprintf("风力等级：%s\n", response.Now.WindScale)
	result += fmt.Sprintf("风速：%s公里/小时\n", response.Now.WindSpeed)
	result += fmt.Sprintf("相对湿度：%s%%\n", response.Now.Humidity)
	result += fmt.Sprintf("过去1小时降水量：%s毫米\n", response.Now.Precip)
	result += fmt.Sprintf("更新时间：%s\n", response.UpdateTime)

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{
				Type: "text",
				Text: result,
			},
		},
	}, nil
}

// 加载配置文件
func loadConfig(configPath string) error {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("读取配置文件失败: %w", err)
	}

	var rootConfig RootConfig
	err = yaml.Unmarshal(data, &rootConfig)
	if err != nil {
		return fmt.Errorf("解析配置文件失败: %w", err)
	}

	config = rootConfig.Weather
	return nil
}

func startWeatherService() {
	var transport string
	var configPath string
	flag.StringVar(&transport, "t", "stdio", "传输类型 (stdio 或 sse)")
	flag.StringVar(&transport, "transport", "stdio", "传输类型 (stdio 或 sse)")
	flag.StringVar(&configPath, "c", "conf.yaml", "配置文件路径")
	flag.StringVar(&configPath, "config", "conf.yaml", "配置文件路径")
	flag.Parse()

	// 加载配置文件
	if err := loadConfig(configPath); err != nil {
		log.Fatalf("加载配置文件失败: %v", err)
	}

	mcpServer := NewWeatherServer()

	// 根据传输类型启动服务器
	if transport == "sse" {
		sseServer := server.NewSSEServer(mcpServer)
		log.Printf("SSE 服务器监听于 :8013")
		if err := sseServer.Start(":8013"); err != nil {
			log.Fatalf("服务器错误: %v", err)
		}
	} else {
		if err := server.ServeStdio(mcpServer); err != nil {
			log.Fatalf("服务器错误: %v", err)
		}
	}
}

// 如果该文件被直接运行，则启动服务
func main() {
	startWeatherService()
}
