# 和风天气 go mcp
需要注册和风天气，新建一个项目，新建一个api，使用jwt格式。配置如下
```yaml
# 天气服务配置
WEATHER:
  # 参考 https://dev.qweather.com/docs/configuration/authentication/
  privateKey: |
    -----BEGIN PRIVATE KEY-----
    -----END PRIVATE KEY-----
  baseURL: "" # 参考 https://dev.qweather.com/docs/configuration/api-config/#api-host
  sub: "" # 项目ID在控制台-项目管理中查看
  kid: "" # 凭据ID，在控制台-项目管理中查看
```


# 运行

- sse `go run weather.go -t sse -p 端口` 默认 8013
- stdio `go run weather.go`
  

# 功能
- 实时天气

其它接口懒得做了，主要为了验证
