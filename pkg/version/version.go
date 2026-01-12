package version

var (
	Version   = "v0.0.0-dev" // 语义化版本号
	GitCommit = "unknown"    // Git 哈希
	BuildTime = "unknown"    // 构建时间
	GoVersion = "unknown"    // Go 编译器版本
)

// Info 定义了版本输出的结构
type Info struct {
	Version   string `json:"version"`
	GitCommit string `json:"gitCommit"`
	BuildTime string `json:"buildTime"`
	GoVersion string `json:"goVersion"`
}

func Get() Info {
	return Info{
		Version:   Version,
		GitCommit: GitCommit,
		BuildTime: BuildTime,
		GoVersion: GoVersion,
	}
}
