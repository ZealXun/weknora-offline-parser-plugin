module github.com/ZealXun/weknora-offline-parser-plugin

go 1.26.0

require (
	github.com/Tencent/WeKnora v0.0.0
	golang.org/x/net v0.56.0
	google.golang.org/grpc v1.81.0
)

require (
	github.com/blang/semver/v4 v4.0.0 // indirect
	golang.org/x/sys v0.46.0 // indirect
	golang.org/x/text v0.38.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260427160629-7cedc36a6bc4 // indirect
	google.golang.org/protobuf v1.36.11 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

// 开发期使用同级主仓中的尚未发布 SDK；SDK 发布后改为正式版本并移除此行。
replace github.com/Tencent/WeKnora => ../WeKnora
