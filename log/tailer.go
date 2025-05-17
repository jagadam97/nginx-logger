package log

import (
	"fmt"

	"github.com/hpcloud/tail"
)

func TailLogFile(filePath string) (*tail.Tail, error) {
	t, err := tail.TailFile(filePath, tail.Config{
		Follow: true,
		ReOpen: true,
		Poll:   true,
		Location: &tail.SeekInfo{Offset: 0, Whence: 2},
	})
	if err != nil {
		return nil, fmt.Errorf("error tailing file: %w", err)
	}
	return t, nil
}
