# 和风天气 go mcp

配置
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

# 功能
只做了一个，懒得做了，主要为了验证
- 实时天气
