package memutils

import (
	"fmt"
	"io"
	"time"
)

func Log(w io.Writer, msg string) {
	fmt.Fprintf(w, "%s | %s | %s", "INFO", time.Now().Format(time.RFC3339), msg)
}
