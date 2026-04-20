package config

const (
	DefaultMaxAPIRequests  uint64 = 1000
	DefaultMaxStorageBytes uint64 = 1 << 30
)

type Config struct {
	MaxAPIRequests  uint64
	MaxStorageBytes uint64
}

func Default() Config {
	return Config{
		MaxAPIRequests:  DefaultMaxAPIRequests,
		MaxStorageBytes: DefaultMaxStorageBytes,
	}
}
