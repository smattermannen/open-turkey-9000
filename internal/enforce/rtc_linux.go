package enforce

import (
	"os"
	"strconv"
	"strings"
)

func readHardwareClock() (int64, bool) {
	data, err := os.ReadFile("/sys/class/rtc/rtc0/since_epoch")
	if err != nil {
		return 0, false
	}
	n, err := strconv.ParseInt(strings.TrimSpace(string(data)), 10, 64)
	return n, err == nil && n > 0
}
