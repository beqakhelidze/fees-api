package fees

import "encore.dev/config"

type Config struct {
	TemporalServer string
}

var cfg = config.Load[*Config]()
