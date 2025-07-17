package bootstrap

import (
    "time"
)

func parseDurationOrDefault(s string) time.Duration {
    d, err := time.ParseDuration(s)
    if err != nil {
        return 5 * time.Minute
    }
    return d
}
