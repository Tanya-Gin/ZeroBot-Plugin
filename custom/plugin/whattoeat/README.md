# 今天吃什么（ZeroBot）

群内发送 `今天吃什么`、`吃什么`、`午饭吃什么`、`中午吃什么`、`晚饭吃什么` 或 `晚上吃什么`，从 TheMealDB 的 Chinese 菜系结果中随机挑一道菜，并发送匹配的菜品图片。

## 数据来源

- 菜名与图片：TheMealDB `filter.php?a=Chinese` API。
- API 文档与测试 key 说明：https://www.themealdb.com/api.php
- 当前实现不需要额外密钥；HTTP 请求有超时和响应大小限制，并验证图片 URL 的 HTTPS、域名及路径。
- 官方将 API 描述为测试和个人项目可免费使用；测试 key `1` 用于开发/学习。公开发布应用应按官方说明获取生产/支持者 key，并核对最新使用条款。

## 实现与故障行为

- 命中 API 返回的 27 道中式菜谱列表后随机选择一项；已知菜名有中文显示映射，API 新增条目保留原始名称。
- ZeroBot 通过 OneBot 图片消息段发送远端图片，不会在服务器长期缓存图片。
- API 不可用、返回数据无效或图片地址不安全时，回复短错误提示，不会拼接或发送未经校验的 URL。
- 使用现有 ZeroBot 插件限频，避免单用户短时间重复请求。

## 本地验证

```bash
go test ./custom/plugin/whattoeat -count=1
go test -race ./custom/plugin/whattoeat -count=1
go vet ./custom/plugin/whattoeat/...
go build -ldflags='-s -w' -o /tmp/zerobot-build-check-food main.go
```
