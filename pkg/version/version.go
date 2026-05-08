package version

var (
	Version   = "v0.0.0-dev" // Semantic version
	GitCommit = "unknown"    // Git commit hash
	BuildTime = "unknown"    // Build time
	GoVersion = "unknown"    // Go compiler version
)

// Info defines the version output structure
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
